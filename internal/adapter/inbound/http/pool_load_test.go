package http_test

import (
	"bytes"
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectionPoolUnderLoad(t *testing.T) {
	app, repo, db := setupIntegrationTest(t)
	defer db.Close()

	concurrency := 100

	var wg sync.WaitGroup
	errs := make(chan error, concurrency)
	durations := make(chan time.Duration, concurrency)

	globalStart := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			reqBody := fmt.Sprintf(`{"user_id":"load-test","type":"text","body":"message %d"}`, idx)
			req := httptest.NewRequest("POST", "/api/v1/contents", bytes.NewBufferString(reqBody))
			req.Header.Set("Content-Type", "application/json")

			start := time.Now()
			resp, err := app.Test(req, -1)
			durations <- time.Since(start)

			if err != nil {
				errs <- err
				return
			}
			if resp.StatusCode != 201 {
				errs <- fmt.Errorf("expected status 201, got %d", resp.StatusCode)
				return
			}
			errs <- nil
		}(i)
	}

	wg.Wait()
	close(errs)
	close(durations)

	var errorsFound []error
	for e := range errs {
		if e != nil {
			errorsFound = append(errorsFound, e)
		}
	}
	require.Empty(t, errorsFound, "some requests failed")

	var total time.Duration
	count := 0
	for d := range durations {
		total += d
		count++
	}
	avg := total / time.Duration(count)
	t.Logf("Total time for all requests: %v", time.Since(globalStart))
	t.Logf("Average response time: %v", avg)
	assert.Less(t, avg, 100*time.Millisecond, "Average response time should be under 100ms")

	stats := repo.Stats()
	t.Logf("Connection Pool Stats -> Open: %d, Idle: %d, InUse: %d, MaxOpen: %d",
		stats.OpenConnections, stats.Idle, stats.InUse, stats.MaxOpenConnections)

	assert.LessOrEqual(t, stats.OpenConnections, 25,
		"Open connections should not exceed max open limit")
	assert.Greater(t, stats.Idle, 0, "There should be some idle connections after load")
}
