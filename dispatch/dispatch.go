package dispatch

import (
	"context"
	"errors"
	"time"

	"github.com/minorhacks/bazel_remote_query/db"
	pb "github.com/minorhacks/bazel_remote_query/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type DatabaseDispatch struct {
	DB db.DB
}

func (d *DatabaseDispatch) GetQueryJob(ctx context.Context, req *pb.GetQueryJobRequest) (*pb.GetQueryJobResponse, error) {
	res := &pb.GetQueryJobResponse{
		NextPollTime: timestamppb.New(time.Now().Add(10 * time.Second)), // TODO: parameterize
	}

	job, err := d.DB.DequeueJob(ctx, req.GetWorkerName())
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
		err = d.DB.FinishJob(ctx, req.GetQueryJobId(), db.StatusSucceeded, r.QueryResultGcsLocation)
	case *pb.FinishQueryJobRequest_FailureMessage:
		err = d.DB.FinishJob(ctx, req.GetQueryJobId(), db.StatusFailed, r.FailureMessage)
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to mark job %s as finished: %v", req.GetQueryJobId(), err)
	}
	return &pb.FinishQueryJobResponse{}, nil
}
