package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/minorhacks/bazel_remote_query/db"
	"github.com/minorhacks/bazel_remote_query/db/sqlite"
	pb "github.com/minorhacks/bazel_remote_query/proto"

	"github.com/golang/glog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	configPath = flag.String("config", "", "Path to textproto DispatcherConfig")
)

type DatabaseDispatch struct {
	db db.DB
}

func (d *DatabaseDispatch) GetQueryJob(ctx context.Context, req *pb.GetQueryJobRequest) (*pb.GetQueryJobResponse, error) {
	res := &pb.GetQueryJobResponse{
		NextPollTime: timestamppb.New(time.Now().Add(10 * time.Second)), // TODO: parameterize
	}

	job, err := d.db.DequeueJob(ctx, req.GetWorkerName())
	if err != nil && errors.Is(err, db.ErrNoOutstandingJobs) {
		return res, nil
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to dequeue next job: %v", err)
	}
	res.Job = &pb.QueryJob{
		Id:    job.ID,
		Query: job.Query,
		Source: &pb.GitCommit{
			Repo:       job.Repository,
			Committish: job.CommitHash,
		},
	}
	return res, nil
}

func (d *DatabaseDispatch) FinishQueryJob(ctx context.Context, req *pb.FinishQueryJobRequest) (*pb.FinishQueryJobResponse, error) {
	var err error
	switch r := req.Result.(type) {
	case *pb.FinishQueryJobRequest_QueryResultGcsLocation:
		err = d.db.FinishJob(ctx, req.GetQueryJobId(), db.StatusSucceeded, r.QueryResultGcsLocation)
	case *pb.FinishQueryJobRequest_FailureMessage:
		err = d.db.FinishJob(ctx, req.GetQueryJobId(), db.StatusFailed, r.FailureMessage)
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to mark job %s as finished: %v", req.GetQueryJobId(), err)
	}
	return &pb.FinishQueryJobResponse{}, nil
}

type DatabaseQueue struct {
	db db.DB
}

func (q *DatabaseQueue) Queue(ctx context.Context, req *pb.QueueRequest) (*pb.QueueResponse, error) {
	job := &db.QueryJob{
		Repository: req.GetRepository(),
		CommitHash: req.GetCommitHash(),
		Query:      req.GetQueryString(),
	}
	if err := q.db.EnqueueJob(ctx, job); err != nil {
		return nil, status.Errorf(codes.Internal, "db.EnqueueJob() failed: %v", err)
	}
	return &pb.QueueResponse{Id: job.ID}, nil
}

func (q *DatabaseQueue) Poll(ctx context.Context, req *pb.PollRequest) (*pb.PollResponse, error) {
	job, err := q.db.GetJob(ctx, req.GetId())
	if err != nil {
		c := codes.Internal
		if errors.Is(err, db.ErrJobNotFound) {
			c = codes.NotFound
		}
		return nil, status.Errorf(c, err.Error())
	}

	res := &pb.PollResponse{
		Id: req.GetId(),
	}
	switch job.Status {
	case db.StatusPending:
		fallthrough
	case db.StatusRunning:
		res.Status = &pb.PollResponse_InProgress{
			InProgress: &pb.PollResponse_QueryInProgress{
				NextPollTime: timestamppb.New(time.Now().Add(5 * time.Second)), // TODO: parameterize
			},
		}
	case db.StatusSucceeded:
		if job.ResultURL == nil {
			return nil, status.Error(codes.FailedPrecondition, "query succeeded but ResultURL is not set")
		}
		res.Status = &pb.PollResponse_Success{
			Success: &pb.PollResponse_QuerySuccess{
				ResultsGcsUrl: *job.ResultURL,
			},
		}
	case db.StatusFailed:
		if job.ResultError == nil {
			return nil, status.Error(codes.FailedPrecondition, "query failed but ResultError is not set")
		}
		res.Status = &pb.PollResponse_Failure{
			Failure: &pb.PollResponse_QueryFailure{
				FailureMessage: *job.ResultError,
			},
		}
	default:
		return nil, status.Errorf(codes.FailedPrecondition, "can't poll job with status: %q", job.Status)
	}
	return res, nil
}

func main() {
	flag.Parse()

	config, err := loadConfig(*configPath)
	exitIf(err)

	conn, err := net.Listen("tcp", net.JoinHostPort("", config.GetGrpcPort()))
	exitIf(err)

	ctx := context.Background()

	db, err := sqlite.New(ctx, config.GetSqlite().GetDbPath())
	exitIf(err)

	dispatchService := &DatabaseDispatch{
		db: db,
	}

	queueService := &DatabaseQueue{
		db: db,
	}

	srv := grpc.NewServer()
	pb.RegisterQueryDispatchServer(srv, dispatchService)
	pb.RegisterQueryQueueServer(srv, queueService)
	reflection.Register(srv)

	glog.Infof("Listening on port %s", config.GetGrpcPort())
	srv.Serve(conn)
}

func loadConfig(path string) (*pb.DispatcherConfig, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config %q: %w", path, err)
	}
	var config pb.DispatcherConfig
	if err := prototext.Unmarshal(contents, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config %q: %w", path, err)
	}
	return &config, nil
}

func exitIf(err error) {
	if err != nil {
		glog.Exit(err)
	}
}
