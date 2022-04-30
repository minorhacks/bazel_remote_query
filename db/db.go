package db

import (
	"context"
	"time"
)

const (
	StatusPending = "pending"
)

type QueryJob struct {
	Repository  string
	CommitHash  string
	Query       string
	ID          string
	Status      string
	Worker      string
	StartTime   time.Time
	FinishTime  time.Time
	ResultURL   string
	ResultError string
}

type DB interface {
	Get(context.Context, string /* id */) (string, error)
	Insert(context.Context, string /* repo */, string /* committish */, string /* query */) (string, error)
	Update(context.Context, string /* id */, string /* state */, string /* result */) error
}
