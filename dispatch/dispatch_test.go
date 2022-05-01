package dispatch

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/minorhacks/bazel_remote_query/db"
	pb "github.com/minorhacks/bazel_remote_query/proto"
	"github.com/minorhacks/bazel_remote_query/testutil"

	"github.com/prashantv/gostub"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type queueEntry struct{}

func TestGetQueryJob(t *testing.T) {
	testCases := []struct {
		desc    string
		req     *pb.GetQueryJobRequest
		queue   []db.FakeQueueEntry
		want    *pb.GetQueryJobResponse
		wantErr string
	}{
		{
			desc: "successful response",
			req: &pb.GetQueryJobRequest{
				WorkerName: "worker-1",
			},
			queue: []db.FakeQueueEntry{
				{
					Job: &db.QueryJob{
						Repository: "https://github.com/grpc/grpc",
						CommitHash: "foobar",
						Query:      "deps(//...)",
						ID:         "abcd",
						Status:     db.StatusPending,
					},
				},
			},
			want: &pb.GetQueryJobResponse{
				Job: &pb.QueryJob{
					Id:    "abcd",
					Query: "deps(//...)",
					Source: &pb.GitCommit{
						Repo:       "https://github.com/grpc/grpc",
						Committish: "foobar",
					},
				},
				NextPollTime: timestamppb.New(testutil.StaticTimeRFC3339("2022-05-01T12:20:10-08:00")),
			},
		},
		{
			desc: "propagates DB error",
			req: &pb.GetQueryJobRequest{
				WorkerName: "worker-1",
			},
			queue: []db.FakeQueueEntry{
				{
					Err: errors.New("some DB error"),
				},
			},
			wantErr: "some DB error",
		},
		{
			desc: "no error when no jobs available",
			req: &pb.GetQueryJobRequest{
				WorkerName: "worker-1",
			},
			queue: []db.FakeQueueEntry{},
			want: &pb.GetQueryJobResponse{
				NextPollTime: timestamppb.New(testutil.StaticTimeRFC3339("2022-05-01T12:20:10-08:00")),
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			stubs := gostub.Stub(&timeNow, func() time.Time {
				return testutil.StaticTimeRFC3339("2022-05-01T12:20:00-08:00")
			})
			defer stubs.Reset()

			ctx := context.Background()
			d := &DatabaseDispatch{
				DB: &db.Fake{
					Queue: tc.queue,
				},
			}
			res, gotErr := d.GetQueryJob(ctx, tc.req)
			if diff := testutil.ErrSubstring(gotErr, tc.wantErr); diff != "" {
				t.Error(diff)
			}
			if gotErr != nil {
				return
			}
			testutil.AssertProtoEqual(t, res, tc.want)
		})
	}
}
