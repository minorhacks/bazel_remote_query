package test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/minorhacks/bazel_remote_query/db"
	"github.com/minorhacks/bazel_remote_query/db/datastore"
	"github.com/minorhacks/bazel_remote_query/db/sqlite"
	"github.com/minorhacks/bazel_remote_query/testdatastore"

	"github.com/stretchr/testify/assert"
)

func TestStressEnqueueDequeue(t *testing.T) {
	testCases := []struct {
		desc      string
		dbFactory func(t *testing.T) (db.DB, func(), error)
	}{
		{
			desc: "sqlite",
			dbFactory: func(t *testing.T) (db.DB, func(), error) {
				tempFile, err := os.CreateTemp(os.Getenv("TEST_TMPDIR"), "stress_enqueue_dequeue_*.sqlite")
				assert.Nil(t, err)
				assert.Nil(t, tempFile.Close())
				tempDB, err := sqlite.New(context.Background(), tempFile.Name())
				assert.Nil(t, err)
				return tempDB, func() {}, err
			},
		},
		{
			desc: "datastore",
			dbFactory: func(t *testing.T) (db.DB, func(), error) {
				ctx := context.Background()
				tds, err := testdatastore.New(ctx, os.Getenv("TEST_TMPDIR"), false)
				assert.Nil(t, err)
				d, err := datastore.New(ctx, "")
				assert.Nil(t, err)
				return d, func() {
					tds.Close()
				}, err
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			tempDB, cleanup, err := tc.dbFactory(t)
			if err != nil {
				return
			}
			defer tempDB.Close()
			defer cleanup()

			numWorkers := 10
			numJobsPerWorker := 100
			numRounds := 1
			t.Logf("Using %d workers to queue %d jobs %d times...", numWorkers, numJobsPerWorker, numRounds)

			// Create a bunch workers, each enqueueing a set of queries multiple times
			var wg sync.WaitGroup
			enqueueWorker := func(t *testing.T, d db.DB, worker int, start int, end int) {
				t.Helper()
				defer t.Logf("Worker %d done enqueuing", worker)
				defer wg.Done()
				for round := 0; round < numRounds; round++ {
					for i := start; i < end; i++ {
						err := d.EnqueueJob(context.Background(), &db.QueryJob{
							Repository: "https://github.com/grpc/grpc",
							CommitHash: fmt.Sprintf("%d", i),
							Query:      "deps(//...)",
						})
						assert.Nilf(t, err, "during enqueue: worker %d round %d job %d: %v", worker, round, i, err)
					}
				}
			}
			for i := 0; i < numWorkers; i++ {
				wg.Add(1)
				go enqueueWorker(t, tempDB, i, numJobsPerWorker*i, numJobsPerWorker*(i+1))
			}
			wg.Wait()
			t.Log("Finished enqueuing")

			// There should now be numWorkers * numJobsPerWorker builds queued; dequeue
			// them all in parallel
			t.Logf("Using %d workers to dequeue %d jobs each...", numWorkers, numJobsPerWorker)
			wg = sync.WaitGroup{}
			dequeueWorker := func(t *testing.T, d db.DB, worker int, numJobs int) {
				t.Helper()
				defer t.Logf("Worker %d done dequeuing", worker)
				defer wg.Done()
				for i := 0; i < numJobs; i++ {
					_, err := d.DequeueJob(context.Background(), fmt.Sprint("worker-%d", worker))
					assert.Nilf(t, err, "during dequeue: worker %d job %d: %v", worker, i, err)
				}
			}
			for i := 0; i < numWorkers; i++ {
				wg.Add(1)
				go dequeueWorker(t, tempDB, i, numJobsPerWorker)
			}
			wg.Wait()
			t.Log("Finished dequeuing")

			// All jobs should be dequeued; additional dequeues should result in no jobs
			// available
			t.Log("Checking for extra jobs...")
			_, err = tempDB.DequeueJob(context.Background(), "worker-0")
			assert.ErrorIs(t, err, db.ErrNoOutstandingJobs)
		})
	}
}
