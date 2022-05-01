package sqlite

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/minorhacks/bazel_remote_query/db"

	"github.com/stretchr/testify/assert"
)

func TestStressEnqueueDequeue(t *testing.T) {
	tempFile, err := os.CreateTemp(os.Getenv("TEST_TMPDIR"), "stress_enqueue_dequeue_*.sqlite")
	assert.Nil(t, err)
	assert.Nil(t, tempFile.Close())
	tempDB, err := New(context.Background(), tempFile.Name())
	assert.Nil(t, err)

	numWorkers := 10
	numJobsPerWorker := 100
	numRounds := 10

	// Create 10 workers, each enqueueing 100 builds 10 times
	var wg sync.WaitGroup
	enqueueWorker := func(t *testing.T, s *Sqlite, worker int, start int, end int) {
		t.Helper()
		defer wg.Done()
		for round := 0; round < numRounds; round++ {
			for i := start; i < end; i++ {
				err := s.EnqueueJob(context.Background(), &db.QueryJob{
					Repository: "https://github.com/grpc/grpc",
					CommitHash: fmt.Sprintf("%d", i),
					Query:      "deps(//...)",
				})
				assert.Nilf(t, err, "worker %d round %d job %d: %v", worker, round, i, err)
			}
		}
	}
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go enqueueWorker(t, tempDB, i, numJobsPerWorker*i, numJobsPerWorker*(i+1))
	}
	wg.Wait()

	// There should now be numWorkers * numJobsPerWorker builds queued; dequeue
	// them all in parallel
	wg = sync.WaitGroup{}
	dequeueWorker := func(t *testing.T, s *Sqlite, worker int, numJobs int) {
		t.Helper()
		defer wg.Done()
		for i := 0; i < numJobs; i++ {
			_, err := s.DequeueJob(context.Background(), fmt.Sprint("worker-%d", worker))
			assert.Nilf(t, err, "worker %d job %d: %v", worker, i, err)
		}
	}
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go dequeueWorker(t, tempDB, i, numJobsPerWorker)
	}
	wg.Wait()

	// All jobs should be dequeued; additional dequeues should result in no jobs
	// available
	_, err = tempDB.DequeueJob(context.Background(), "worker-0")
	assert.ErrorIs(t, err, db.ErrNoOutstandingJobs)
}
