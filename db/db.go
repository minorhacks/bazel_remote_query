package db

import (
	"context"
	"errors"
	"time"
)

const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
)

var (
	ErrNoOutstandingJobs = errors.New("no pending jobs")
	ErrJobNotFound       = errors.New("job not found")
)

type QueryJob struct {
	Repository  string
	CommitHash  string
	Query       string
	ID          string
	Status      string
	Worker      *string
	QueueTime   time.Time
	StartTime   *time.Time
	FinishTime  *time.Time
	ResultURL   *string
	ResultError *string
}

// The invariants of the DB are:
// * There should be only one (repository, commit, query) tuple in the
//   non-failed state (either queued or running or succeeded) at any point in
//   time
// * There can be multiple (repository, commit, query) tuples in the failed
//   state
type DB interface {
	// EnqueueJob enqueues a query to be run in a specific repository at a
	// specific point in the commit history.
	//
	// Input QueryJob must have the Repository, CommitHash, Query fields
	// populated.
	//
	// On exit, QueryJob has the ID and QueueTime fields populated.
	//
	// If there is an existing non-failed job with the same Repository, CommitHash, and
	// Query, enqueue requests should deduplicate to the same request ID; failed
	// jobs are ignored for the purposes of this deduplication.
	EnqueueJob(context.Context, *QueryJob) error

	DequeueJob(ctx context.Context, workerName string) (*QueryJob, error)

	GetJob(ctx context.Context, id string) (*QueryJob, error)

	FinishJob(ctx context.Context, id string, status string, result string) error
}
