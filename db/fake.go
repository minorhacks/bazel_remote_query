package db

import (
	"context"
)

type FakeQueueEntry struct {
	Job *QueryJob
	Err error
}

type Fake struct {
	Queue []FakeQueueEntry

	EnqueueJobErr error
	GetJobErr     error
	FinishJobErr  error
}

func (f *Fake) EnqueueJob(ctx context.Context, job *QueryJob) error {
	if f.EnqueueJobErr != nil {
		return f.EnqueueJobErr
	}
	f.Queue = append(f.Queue, FakeQueueEntry{Job: job, Err: nil})
	return nil
}

func (f *Fake) DequeueJob(ctx context.Context, workerName string) (*QueryJob, error) {
	if len(f.Queue) == 0 {
		return nil, ErrNoOutstandingJobs
	}
	head := f.Queue[0]
	if len(f.Queue) > 1 {
		f.Queue = f.Queue[1:]
	} else {
		f.Queue = nil
	}
	return head.Job, head.Err
}

func (f *Fake) GetJob(ctx context.Context, id string) (*QueryJob, error) {
	if f.GetJobErr != nil {
		return nil, f.GetJobErr
	}
	for _, entry := range f.Queue {
		if entry.Job != nil && entry.Job.ID == id {
			return entry.Job, nil
		}
	}
	return nil, ErrJobNotFound
}

func (f *Fake) FinishJob(ctx context.Context, id string, status string, result string) error {
	return f.FinishJobErr
}
