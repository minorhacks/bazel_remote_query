package queue

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

type DatabaseQueue struct {
	DB db.DB
}

func (q *DatabaseQueue) Queue(ctx context.Context, req *pb.QueueRequest) (*pb.QueueResponse, error) {
	job := &db.QueryJob{
		Repository: req.GetRepository(),
		CommitHash: req.GetCommitHash(),
		Query:      req.GetQueryString(),
	}
	if err := q.DB.EnqueueJob(ctx, job); err != nil {
		return nil, status.Errorf(codes.Internal, "db.EnqueueJob() failed: %v", err)
	}
	return &pb.QueueResponse{Id: job.ID}, nil
}

func (q *DatabaseQueue) Poll(ctx context.Context, req *pb.PollRequest) (*pb.PollResponse, error) {
	job, err := q.DB.GetJob(ctx, req.GetId())
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
