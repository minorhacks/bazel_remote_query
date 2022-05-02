package datastore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/minorhacks/bazel_remote_query/db"

	"cloud.google.com/go/datastore"
	"github.com/google/uuid"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	typeQueryJob = "QueryJob"
)

var errTooMuchContention = status.Errorf(codes.Aborted, "too much contention on these datastore entities. please try again.")

type DB struct {
	client *datastore.Client
}

func New(ctx context.Context, projectName string) (*DB, error) {
	dsClient, err := datastore.NewClient(ctx, projectName)
	if err != nil {
		return nil, fmt.Errorf("failed to create datastore client for GCP project %q: %w", projectName, err)
	}
	return &DB{
		client: dsClient,
	}, nil
}

func (d *DB) Close() error {
	return d.client.Close()
}

func (d *DB) EnqueueJob(ctx context.Context, job *db.QueryJob) error {
	_, err := d.client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		q := datastore.NewQuery(typeQueryJob)
		q = q.Filter("repository =", job.Repository)
		q = q.Filter("commit_hash =", job.CommitHash)
		q = q.Filter("query_string =", job.Query)
		iter := d.client.Run(ctx, q)

		var iterJob db.QueryJob
		var err error
		for _, err = iter.Next(&iterJob); err == nil; _, err = iter.Next(&iterJob) {
			if iterJob.Status != db.StatusFailed {
				// Job is either
				// * successful, and is cacheable
				// * in progress, and we want to dedupe this request
				*job = iterJob
				return nil
			}
		}
		// Either iteration is finished or failed
		if !errors.Is(err, iterator.Done) {
			return fmt.Errorf("failed while searching for matching existing queries: %w", err)
		}
		// Successfully scanned but found no matching cacheable jobs; add a new
		// entry
		id, err := uuid.NewRandom()
		if err != nil {
			return fmt.Errorf("failed to create UUID for query: %w", err)
		}
		job = &db.QueryJob{
			ID:         id.String(),
			Repository: job.Repository,
			CommitHash: job.CommitHash,
			Query:      job.Query,
			Status:     db.StatusPending,
			QueueTime:  time.Now().UTC(),
		}
		_, err = tx.Put(datastore.IncompleteKey(typeQueryJob, nil), job)
		if err != nil {
			return fmt.Errorf("failed to queue query: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

func (d *DB) attemptDequeueTx(ctx context.Context, workerName string) (*db.QueryJob, error) {
	var retJob db.QueryJob

	_, err := d.client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		q := datastore.NewQuery(typeQueryJob)
		q = q.Filter("status =", db.StatusPending)
		q = q.Order("queue_time") // Ascending is the default
		iter := d.client.Run(ctx, q)

		var err error
		var key *datastore.Key
		for key, err = iter.Next(nil); err == nil; key, err = iter.Next(nil) {
			queryErr := tx.Get(key, &retJob)
			if queryErr != nil {
				// Alias any errors about contention to
				// ErrConcurrentTransaction, so that upper levels can deal with
				// them equivalently. This is only really a problem in tests,
				// where a bunch of contention is created artificially to ensure
				// that when contention does occur (which is rare) we behave as
				// expected.
				if errors.Is(queryErr, errTooMuchContention) {
					return fmt.Errorf("while fetching %v: %w", key, datastore.ErrConcurrentTransaction)
				}
				return fmt.Errorf("while fetching %v: %w", key, queryErr)
			}
			// Double-check the condition, since queries happen outside the
			// transaction - it's possible that a dequeue operation has already
			// marked this as running. Performing this Get does happen within
			// the transaction, so it should be "locked" after this point.
			if retJob.Status == db.StatusPending {
				break
			}
		}
		if err != nil && !errors.Is(err, iterator.Done) {
			return fmt.Errorf("error while searching for queued query jobs: %w", err)
		} else if errors.Is(err, iterator.Done) {
			return db.ErrNoOutstandingJobs
		}
		retJob.Status = db.StatusRunning
		retJob.Worker = &workerName
		now := time.Now().UTC()
		retJob.StartTime = &now

		_, err = tx.Put(key, &retJob)
		if err != nil {
			return fmt.Errorf("failed to mark job %s as running: %w", retJob.ID, err)
		}
		return nil
	})
	if err != nil && errors.Is(err, db.ErrNoOutstandingJobs) {
		return nil, err
	} else if err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	return &retJob, nil
}

func (d *DB) DequeueJob(ctx context.Context, workerName string) (*db.QueryJob, error) {
	for {
		job, err := d.attemptDequeueTx(ctx, workerName)
		if err != nil && errors.Is(err, datastore.ErrConcurrentTransaction) {
			continue
		}
		return job, err
	}
}

func (d *DB) GetJob(ctx context.Context, id string) (*db.QueryJob, error) {
	q := datastore.NewQuery(typeQueryJob)
	q = q.Filter("id =", id)
	iter := d.client.Run(ctx, q)

	key, err := singleKeyFromIter(iter)
	var job db.QueryJob
	if err := d.client.Get(ctx, key, &job); err != nil {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query datastore by QueryJob.id: %w", err)
	}

	return &job, nil
}

func (d *DB) FinishJob(ctx context.Context, id string, status string, result string) error {
	_, err := d.client.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		q := datastore.NewQuery(typeQueryJob)
		q = q.Filter("id =", id)
		iter := d.client.Run(ctx, q)
		key, err := singleKeyFromIter(iter)
		if err != nil {
			return fmt.Errorf("failed to query datastore by QueryJob.id %q: %w", id, err)
		}

		var job db.QueryJob
		if err := tx.Get(key, &job); err != nil {
			return err
		}

		now := time.Now().UTC()
		job.FinishTime = &now
		job.Status = status
		switch status {
		case db.StatusSucceeded:
			job.ResultURL = &result
		case db.StatusFailed:
			job.ResultError = &result
		default:
			return fmt.Errorf("can't finish job using status %q", status)
		}

		_, err = tx.Put(key, &job)
		if err != nil {
			return fmt.Errorf("failed to mark job %s as done: %w", id, err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to mark job %s as done: %w", id, err)
	}
	return nil
}

func singleKeyFromIter(iter *datastore.Iterator) (*datastore.Key, error) {
	key, err := iter.Next(nil)
	if err != nil && !errors.Is(err, iterator.Done) {
		return nil, err
	} else if errors.Is(err, iterator.Done) {
		return nil, nil
	}
	if _, err := iter.Next(nil); err == nil {
		return nil, fmt.Errorf("found multiple QueryJobs; want <=1")
	}
	return key, nil
}
