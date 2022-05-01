package datastore

import (
	"context"
	"fmt"

	"github.com/minorhacks/bazel_remote_query/db"
)

type DB struct{}

func New(ctx context.Context) (*DB, error) {
	return nil, fmt.Errorf("not yet implemented")
}

func (d *DB) EnqueueJob(ctx context.Context, job *db.QueryJob) error {
	return fmt.Errorf("not yet implemented")
}

func (d *DB) DequeueJob(ctx context.Context, workerName string) (*db.QueryJob, error) {
	return nil, fmt.Errorf("not yet implemented")
}

func (d *DB) GetJob(ctx context.Context, id string) (*db.QueryJob, error) {
	return nil, fmt.Errorf("not yet implemented")
}

func (d *DB) FinishJob(ctx context.Context, id string, status string, result string) error {
	return fmt.Errorf("not yet implemented")
}
