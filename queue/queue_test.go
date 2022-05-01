package queue

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

func TestQueue(t *testing.T) {
	testCases := []struct {
		desc       string
		req        *pb.QueueRequest
		enqueueErr error
		want       *pb.QueueResponse
		wantErr    string
	}{
		{
			desc: "successful queue",
			req:  &pb.QueueRequest{},
			want: &pb.QueueResponse{},
		},
		{
			desc:       "propagates enqueue failure",
			req:        &pb.QueueRequest{},
			enqueueErr: errors.New("some enqueue error"),
			wantErr:    "some enqueue error",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := context.Background()
			d := &DatabaseQueue{
				DB: &db.Fake{
					EnqueueJobErr: tc.enqueueErr,
				},
			}

			got, gotErr := d.Queue(ctx, tc.req)
			if diff := testutil.ErrSubstring(gotErr, tc.wantErr); diff != "" {
				t.Error(diff)
			}
			if gotErr != nil {
				return
			}
			testutil.AssertProtoEqual(t, got, tc.want)
		})
	}
}

func TestPoll(t *testing.T) {
	testCases := []struct {
		desc    string
		req     *pb.PollRequest
		want    *pb.PollResponse
		wantErr string
	}{
		{
			desc: "pending job",
			req: &pb.PollRequest{
				Id: "1",
			},
			want: &pb.PollResponse{
				Id: "1",
				Status: &pb.PollResponse_InProgress{
					InProgress: &pb.PollResponse_QueryInProgress{
						NextPollTime: timestamppb.New(testutil.StaticTimeRFC3339("2022-05-01T12:20:05-08:00")),
					},
				},
			},
		},
		{
			desc: "running job",
			req: &pb.PollRequest{
				Id: "2",
			},
			want: &pb.PollResponse{
				Id: "2",
				Status: &pb.PollResponse_InProgress{
					InProgress: &pb.PollResponse_QueryInProgress{
						NextPollTime: timestamppb.New(testutil.StaticTimeRFC3339("2022-05-01T12:20:05-08:00")),
					},
				},
			},
		},
		{
			desc: "failed job",
			req: &pb.PollRequest{
				Id: "3",
			},
			want: &pb.PollResponse{
				Id: "3",
				Status: &pb.PollResponse_Failure{
					Failure: &pb.PollResponse_QueryFailure{
						FailureMessage: "some query failure",
					},
				},
			},
		},
		{
			desc: "successful job",
			req: &pb.PollRequest{
				Id: "4",
			},
			want: &pb.PollResponse{
				Id: "4",
				Status: &pb.PollResponse_Success{
					Success: &pb.PollResponse_QuerySuccess{
						ResultsGcsUrl: "gs://bucket/result.pb",
					},
				},
			},
		},
		{
			desc: "nonexistent job",
			req: &pb.PollRequest{
				Id: "5",
			},
			wantErr: "job not found",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			stubs := gostub.Stub(&timeNow, func() time.Time {
				return testutil.StaticTimeRFC3339("2022-05-01T12:20:00-08:00")
			})

			defer stubs.Reset()
			ctx := context.Background()
			var (
				queryFailure = "some query failure"
				resultURL    = "gs://bucket/result.pb"
			)
			d := &DatabaseQueue{
				DB: &db.Fake{
					Queue: []db.FakeQueueEntry{
						{
							Job: &db.QueryJob{
								ID:     "1",
								Status: db.StatusPending,
							},
						},
						{
							Job: &db.QueryJob{
								ID:     "2",
								Status: db.StatusRunning,
							},
						},
						{
							Job: &db.QueryJob{
								ID:          "3",
								Status:      db.StatusFailed,
								ResultError: &queryFailure,
							},
						},
						{
							Job: &db.QueryJob{
								ID:        "4",
								Status:    db.StatusSucceeded,
								ResultURL: &resultURL,
							},
						},
					},
				},
			}

			got, gotErr := d.Poll(ctx, tc.req)
			if diff := testutil.ErrSubstring(gotErr, tc.wantErr); diff != "" {
				t.Error(diff)
			}
			if gotErr != nil {
				return
			}
			testutil.AssertProtoEqual(t, got, tc.want)
		})
	}
}
