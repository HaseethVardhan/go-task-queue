package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// TaskPayload describes the work a task should perform, including optional
// simulation knobs used by the benchmark harness.
type TaskPayload struct {
	Message            string  `json:"message,omitempty"`
	ReportType         string  `json:"reportType,omitempty"`
	ImageName          string  `json:"imageName,omitempty"`
	MinDurationMs      int     `json:"minDurationMs"`
	MaxDurationMs      int     `json:"maxDurationMs"`
	FailureProbability float64 `json:"failureProbability"`
}

// TaskSpec pairs a task type with its payload for batch submission.
type TaskSpec struct {
	Type    string
	Payload TaskPayload
}

// SubmittedTask records a successfully accepted task and the instant it was
// submitted so latency can be measured later.
type SubmittedTask struct {
	ID          string
	Type        string
	SubmittedAt time.Time
}

// TaskResult captures the final observed state of a task after polling.
type TaskResult struct {
	Task       SubmittedTask
	Status     string
	Latency    time.Duration
	TimedOut   bool
	PollErrors int
}


type BenchmarkClient struct {
	httpClient    *http.Client
	apiURL        string
	submissionSem chan struct{} // bounded submission concurrency
	pollingSem    chan struct{} // bounded polling concurrency
	pollingErrors int64         // atomic counter
	verbose       bool
}

// NewBenchmarkClientWithHTTP creates a client with a custom http.Client (e.g. for testing with in-memory RoundTripper).
func NewBenchmarkClientWithHTTP(httpClient *http.Client, apiURL string, submitConcurrency, pollConcurrency int, verbose bool) *BenchmarkClient {
	if httpClient == nil {
		httpClient = &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     90 * time.Second,
			},
			Timeout: 10 * time.Second,
		}
	}
	return &BenchmarkClient{
		httpClient:    httpClient,
		apiURL:        apiURL,
		submissionSem: make(chan struct{}, submitConcurrency),
		pollingSem:    make(chan struct{}, pollConcurrency),
		verbose:       verbose,
	}
}

// NewBenchmarkClient creates a client configured for sustained load testing.
func NewBenchmarkClient(apiURL string, submitConcurrency, pollConcurrency int, verbose bool) *BenchmarkClient {
	return NewBenchmarkClientWithHTTP(nil, apiURL, submitConcurrency, pollConcurrency, verbose)
}

// CheckHealth verifies the API server is reachable and healthy.
func (c *BenchmarkClient) CheckHealth(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("creating health request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}
	return nil
}

// clientSubmitRequest is the JSON body sent to POST /tasks.
type clientSubmitRequest struct {
	Type    string      `json:"type"`
	Payload TaskPayload `json:"payload"`
}

// clientSubmitResponse is the JSON body returned from POST /tasks.
type clientSubmitResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// SubmitTask sends a single task to the API, respecting the submission
// semaphore and context cancellation.
func (c *BenchmarkClient) SubmitTask(ctx context.Context, taskType string, payload TaskPayload) (SubmittedTask, error) {
	// Acquire submission semaphore with context support.
	select {
	case c.submissionSem <- struct{}{}:
		defer func() { <-c.submissionSem }()
	case <-ctx.Done():
		return SubmittedTask{}, ctx.Err()
	}

	body, err := json.Marshal(clientSubmitRequest{
		Type:    taskType,
		Payload: payload,
	})
	if err != nil {
		return SubmittedTask{}, fmt.Errorf("marshaling task body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL+"/tasks", bytes.NewReader(body))
	if err != nil {
		return SubmittedTask{}, fmt.Errorf("creating submit request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Record submittedAt BEFORE the HTTP call so latency includes network time.
	submittedAt := time.Now()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return SubmittedTask{}, fmt.Errorf("submitting task: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return SubmittedTask{}, fmt.Errorf("reading submit response: %w", err)
	}

	if resp.StatusCode != http.StatusAccepted {
		return SubmittedTask{}, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var sr clientSubmitResponse
	if err := json.Unmarshal(respBody, &sr); err != nil {
		return SubmittedTask{}, fmt.Errorf("decoding submit response: %w", err)
	}

	if sr.ID == "" {
		return SubmittedTask{}, fmt.Errorf("server returned empty task ID")
	}

	return SubmittedTask{
		ID:          sr.ID,
		Type:        taskType,
		SubmittedAt: submittedAt,
	}, nil
}

// clientPollResponse is the JSON body returned from GET /tasks/{id}.
type clientPollResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// PollTaskStatus polls a task until it reaches a terminal status or the
// context is cancelled. It respects the polling semaphore for concurrency
// control.
func (c *BenchmarkClient) PollTaskStatus(ctx context.Context, task SubmittedTask, pollInterval time.Duration) TaskResult {
	result := TaskResult{
		Task: task,
	}

	// Acquire polling semaphore with context support.
	select {
	case c.pollingSem <- struct{}{}:
		defer func() { <-c.pollingSem }()
	case <-ctx.Done():
		result.TimedOut = true
		result.Latency = time.Since(task.SubmittedAt)
		return result
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	url := c.apiURL + "/tasks/" + task.ID

	for {
		select {
		case <-ctx.Done():
			result.TimedOut = true
			result.Latency = time.Since(task.SubmittedAt)
			return result
		case <-ticker.C:
			status, err := c.pollOnce(ctx, url)
			if err != nil {
				atomic.AddInt64(&c.pollingErrors, 1)
				result.PollErrors++
				if c.verbose {
					fmt.Printf("[poll] error for task %s: %v\n", task.ID, err)
				}
				continue
			}

			result.Status = status

			if status == "completed" || status == "failed" {
				result.Latency = time.Since(task.SubmittedAt)
				return result
			}
		}
	}
}

// pollOnce performs a single GET /tasks/{id} request and returns the status.
func (c *BenchmarkClient) pollOnce(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("creating poll request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("polling task: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", fmt.Errorf("reading poll response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("poll returned status %d: %s", resp.StatusCode, string(body))
	}

	var pr clientPollResponse
	if err := json.Unmarshal(body, &pr); err != nil {
		return "", fmt.Errorf("decoding poll response: %w", err)
	}

	return pr.Status, nil
}

// SubmitBatch submits a slice of task specs concurrently using a worker pool
// bounded by the submission semaphore. It returns the successfully submitted
// tasks and the number of errors.
func (c *BenchmarkClient) SubmitBatch(ctx context.Context, tasks []TaskSpec) ([]SubmittedTask, int) {
	type indexedResult struct {
		task SubmittedTask
		err  error
	}

	results := make([]indexedResult, len(tasks))

	var wg sync.WaitGroup
	work := make(chan int, len(tasks))

	// Feed task indices into the work channel.
	for i := range tasks {
		work <- i
	}
	close(work)

	// Spin up workers equal to the submission semaphore capacity so the
	// semaphore inside SubmitTask provides the actual bound.
	workerCount := cap(c.submissionSem)
	if workerCount > len(tasks) {
		workerCount = len(tasks)
	}

	wg.Add(workerCount)
	for w := 0; w < workerCount; w++ {
		go func() {
			defer wg.Done()
			for idx := range work {
				spec := tasks[idx]
				submitted, err := c.SubmitTask(ctx, spec.Type, spec.Payload)
				results[idx] = indexedResult{task: submitted, err: err}
				if err != nil && c.verbose {
					fmt.Printf("[submit] error for task %d (%s): %v\n", idx, spec.Type, err)
				}
			}
		}()
	}

	wg.Wait()

	accepted := make([]SubmittedTask, 0, len(tasks))
	errCount := 0
	for _, r := range results {
		if r.err != nil {
			errCount++
			continue
		}
		accepted = append(accepted, r.task)
	}

	return accepted, errCount
}

// PollBatch polls all submitted tasks concurrently using a worker pool bounded
// by the polling semaphore capacity. Workers pick tasks from a channel and poll
// each until it reaches a terminal status or the context is cancelled.
func (c *BenchmarkClient) PollBatch(ctx context.Context, tasks []SubmittedTask, pollInterval time.Duration) []TaskResult {
	results := make([]TaskResult, len(tasks))

	var wg sync.WaitGroup
	work := make(chan int, len(tasks))

	for i := range tasks {
		work <- i
	}
	close(work)

	workerCount := cap(c.pollingSem)
	if workerCount > len(tasks) {
		workerCount = len(tasks)
	}

	wg.Add(workerCount)
	for w := 0; w < workerCount; w++ {
		go func() {
			defer wg.Done()
			for idx := range work {
				results[idx] = c.PollTaskStatus(ctx, tasks[idx], pollInterval)
			}
		}()
	}

	wg.Wait()
	return results
}

// PollingErrors returns the total number of transient polling errors observed
// across all poll operations.
func (c *BenchmarkClient) PollingErrors() int64 {
	return atomic.LoadInt64(&c.pollingErrors)
}

// ResetPollingErrors resets the polling error counter to zero.
func (c *BenchmarkClient) ResetPollingErrors() {
	atomic.StoreInt64(&c.pollingErrors, 0)
}

// Close shuts down idle connections. The client should not be used after Close.
func (c *BenchmarkClient) Close() {
	c.httpClient.CloseIdleConnections()
}
