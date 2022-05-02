package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/minorhacks/bazel_remote_query/db"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

type Sqlite struct {
	db *sql.DB
}

func New(ctx context.Context, dbPath string) (*Sqlite, error) {
	sqlDB, err := sql.Open("sqlite3", fmt.Sprintf("%s?cache=shared", dbPath))
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database %q: %w", dbPath, err)
	}
	sqlDB.SetMaxOpenConns(1)
	createTableStmt := `
	CREATE TABLE IF NOT EXISTS "bazel_query_jobs" (
		id TEXT NOT NULL,
		repository TEXT NOT NULL,
		commit_hash TEXT NOT NULL,
		query_string TEXT NOT NULL,
		status TEXT NOT NULL,
		worker TEXT,
		queue_time TEXT NOT NULL,
		start_time TEXT,
		finish_time TEXT,
		query_result_url TEXT,
		query_error TEXT,
		PRIMARY KEY(id)
	);
	`
	_, err = sqlDB.Exec(createTableStmt)
	if err != nil {
		return nil, fmt.Errorf("failed to create 'bazel_query_jobs' table: %w", err)
	}

	return &Sqlite{db: sqlDB}, nil
}

func (s *Sqlite) Close() error {
	return s.db.Close()
}

func (s *Sqlite) EnqueueJob(ctx context.Context, job *db.QueryJob) (retErr error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("failed to start enqueue transaction: %w", err)
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `
	SELECT
		repository,
		commit_hash,
		query_string,
		id,
		status,
		worker,
		queue_time,
		start_time,
		finish_time,
		query_result_url,
		query_error
	FROM "bazel_query_jobs"
	WHERE
		repository = $1 AND
		commit_hash = $2 AND
		query_string = $3 AND
		status != $4;
	`, job.Repository, job.CommitHash, job.Query, db.StatusFailed)
	r, err := jobFromRow(row)
	if err != nil && !errors.Is(err, db.ErrNoOutstandingJobs) {
		return fmt.Errorf("failed to query for existing jobs: %w", err)
	} else if err == nil {
		// Job has already been executed; return the cached result
		*job = *r
		return nil
	}
	insertStmt := `
	INSERT INTO "bazel_query_jobs" (
		repository,
		commit_hash,
		query_string,
		id,
		status,
		queue_time
	)
	VALUES ($1, $2, $3, $4, $5, $6);
	`
	id, err := uuid.NewRandom()
	if err != nil {
		return fmt.Errorf("failed to create UUID for query: %w", err)
	}
	_, err = tx.ExecContext(
		ctx,
		insertStmt,
		job.Repository,
		job.CommitHash,
		job.Query,
		id,
		db.StatusPending,
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("failed to queue query: %w", err)
	}
	job.ID = id.String()

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit queued query: %w", err)
	}
	return nil
}

func (s *Sqlite) DequeueJob(ctx context.Context, workerName string) (*db.QueryJob, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, fmt.Errorf("failed to start dequeue transaction: %w", err)
	}
	defer tx.Rollback()

	// Get the first job in PENDING state
	row := tx.QueryRowContext(ctx, `
	SELECT
		repository,
		commit_hash,
		query_string,
		id,
		status,
		worker,
		queue_time,
		start_time,
		finish_time,
		query_result_url,
		query_error
	FROM "bazel_query_jobs"
	WHERE status = $1
	ORDER BY queue_time ASC;
	`, db.StatusPending)
	job, err := jobFromRow(row)
	if err != nil {
		return nil, err
	}

	job.Status = db.StatusRunning
	job.Worker = &workerName
	now := time.Now().UTC()
	job.StartTime = &now

	result, err := tx.ExecContext(ctx, `
	UPDATE "bazel_query_jobs"
	SET
		status = $1,
		worker = $2,
		start_time = $3
	WHERE
		id = $4;
	`, job.Status, job.Worker, job.StartTime.UTC().Format(time.RFC3339), job.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to mark job %s as running: %w", job.ID, err)
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return nil, fmt.Errorf("want 1 row affected, got %d rows affected with error: %w", n, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit job assignment for %s: %w", job.ID, err)
	}
	return job, nil
}

func (s *Sqlite) GetJob(ctx context.Context, id string) (*db.QueryJob, error) {
	row := s.db.QueryRowContext(ctx, `
	SELECT
		repository,
		commit_hash,
		query_string,
		id,
		status,
		worker,
		queue_time,
		start_time,
		finish_time,
		query_result_url,
		query_error
	FROM "bazel_query_jobs"
	WHERE id = $1;
	`, id)
	if err := row.Err(); errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("job %s: %w", id, db.ErrJobNotFound)
	}
	return jobFromRow(row)
}

func (s *Sqlite) FinishJob(ctx context.Context, id string, status string, result string) error {
	var (
		sqlRes sql.Result
		err    error
	)
	switch status {
	case db.StatusSucceeded:
		sqlRes, err = s.db.ExecContext(ctx, `
		UPDATE "bazel_query_jobs"
		SET
			status = $1,
			finish_time = $2,
			query_result_url = $3
		WHERE
			id = $4;
		`, status, time.Now().UTC().Format(time.RFC3339), result, id)
	case db.StatusFailed:
		sqlRes, err = s.db.ExecContext(ctx, `
		UPDATE "bazel_query_jobs"
		SET
			status = $1,
			finish_time = $2,
			query_error = $3
		WHERE
			id = $4;
		`, status, time.Now().UTC().Format(time.RFC3339), result, id)
	default:
		return fmt.Errorf("can't finish job using status %q", status)
	}
	if err != nil {
		return fmt.Errorf("failed to mark job %s as done: %w", id, err)
	}
	if n, err := sqlRes.RowsAffected(); err != nil || n != 1 {
		return fmt.Errorf("want 1 row affected; got %d rows affected with error: %w", n, err)
	}
	return nil
}

func jobFromRow(r *sql.Row) (*db.QueryJob, error) {
	var (
		j          db.QueryJob
		queryTime  string
		startTime  *string
		finishTime *string
	)
	err := r.Scan(
		&j.Repository,
		&j.CommitHash,
		&j.Query,
		&j.ID,
		&j.Status,
		&j.Worker,
		&queryTime,
		&startTime,
		&finishTime,
		&j.ResultURL,
		&j.ResultError,
	)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return nil, db.ErrNoOutstandingJobs
	} else if err != nil {
		return nil, fmt.Errorf("while translating sqlite row to QueryJob: %w", err)
	}
	j.QueueTime, err = time.Parse(time.RFC3339, queryTime)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query_time for job %s: %w", j.ID, err)
	}
	if startTime != nil {
		t, err := time.Parse(time.RFC3339, *startTime)
		if err != nil {
			return nil, fmt.Errorf("failed to parse start_time for job %s: %w", j.ID, err)
		}
		j.StartTime = &t
	}
	if finishTime != nil {
		t, err := time.Parse(time.RFC3339, *finishTime)
		if err != nil {
			return nil, fmt.Errorf("failed to parse finish_time for job %s: %w", j.ID, err)
		}
		j.FinishTime = &t
	}
	return &j, nil
}
