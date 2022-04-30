package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/minorhacks/bazel_remote_query/db"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

type Sqlite struct {
	db *sql.DB
}

func New(ctx context.Context, dbPath string) (*Sqlite, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database %q: %w", dbPath, err)
	}
	createTableStmt := `
	CREATE TABLE IF NOT EXISTS "bazel_query_jobs" (
		repository TEXT,
		commit_hash TEXT,
		query_string TEXT,
		id TEXT,
		status TEXT,
		worker TEXT,
		start_time TEXT,
		finish_time TEXT,
		query_result_url TEXT,
		query_error TEXT,
		PRIMARY KEY(repository, commit_hash, query_string)
	);
	`
	_, err = db.Exec(createTableStmt)
	if err != nil {
		return nil, fmt.Errorf("failed to create 'bazel_query_jobs' table: %w", err)
	}

	return &Sqlite{db: db}, nil
}

func (s *Sqlite) Get(ctx context.Context, id string) (string, error) {
	return "", fmt.Errorf("Get() not yet implemented")
}

func (s *Sqlite) Insert(ctx context.Context, repo string, committish string, query string) (string, error) {
	insertStmt := `
	INSERT INTO "bazel_query_jobs" (
		repository,
		commit_hash,
		query_string,
		id,
		status
	)
	VALUES ($1, $2, $3, $4, $5);
	`
	id, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("failed to create UUID for query: %w", err)
	}
	_, err = s.db.ExecContext(ctx, insertStmt, repo, committish, query, id, db.StatusPending)
	if err != nil {
		return "", fmt.Errorf("failed to queue query: %w", err)
	}
	return id.String(), nil
}

func (s *Sqlite) Update(ctx context.Context, id string, state string, result string) error {
	return fmt.Errorf("Update() not yet implemented")
}
