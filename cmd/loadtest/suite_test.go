package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// 1. Metric calculations
func TestMetricCalculations(t *testing.T) {
	m := &ExperimentMetrics{
		SubmissionAttempts:  100,
		AcceptedSubmissions: 95,
		SubmissionErrors:    5,
		CompletedTasks:      90,
		FailedTasks:         3,
		TimedOutTasks:       2,
		PendingTasks:        1,
		ProcessingTasks:     1,
		SubmissionStartTime: time.Unix(100, 0),
		SubmissionEndTime:   time.Unix(105, 0), // 5s duration
		PollingStartTime:    time.Unix(105, 0),
		PollingEndTime:      time.Unix(115, 0), // total elapsed 15s
		Latencies: []time.Duration{
			100 * time.Millisecond,
			200 * time.Millisecond,
			300 * time.Millisecond,
			400 * time.Millisecond,
		},
	}

	expectedRate := 95.0 / 5.0 // 19 tasks/sec
	if m.AcceptedSubmissionRate() != expectedRate {
		t.Errorf("expected AcceptedSubmissionRate %f, got %f", expectedRate, m.AcceptedSubmissionRate())
	}

	expectedThroughput := 90.0 / 15.0 // 6 tasks/sec
	if m.CompletedThroughput() != expectedThroughput {
		t.Errorf("expected CompletedThroughput %f, got %f", expectedThroughput, m.CompletedThroughput())
	}

	avgLat := m.AvgLatency()
	if avgLat != 250*time.Millisecond {
		t.Errorf("expected AvgLatency 250ms, got %s", avgLat)
	}

	if m.TerminalTasks() != 93 {
		t.Errorf("expected TerminalTasks 93, got %d", m.TerminalTasks())
	}

	if m.OutstandingTasks() != 2 {
		t.Errorf("expected OutstandingTasks 2, got %d", m.OutstandingTasks())
	}
}

// 2. Percentile calculations
func TestPercentileCalculations(t *testing.T) {
	latencies := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
		60 * time.Millisecond,
		70 * time.Millisecond,
		80 * time.Millisecond,
		90 * time.Millisecond,
		100 * time.Millisecond,
	}

	p50 := calcPercentile(latencies, 0.50)
	if p50 != 50*time.Millisecond {
		t.Errorf("expected P50 50ms, got %v", p50)
	}

	p95 := calcPercentile(latencies, 0.95)
	if p95 != 100*time.Millisecond {
		t.Errorf("expected P95 100ms, got %v", p95)
	}

	p99 := calcPercentile(latencies, 0.99)
	if p99 != 100*time.Millisecond {
		t.Errorf("expected P99 100ms, got %v", p99)
	}

	// Empty slice test
	if calcPercentile(nil, 0.50) != 0 {
		t.Errorf("expected 0 for empty slice, got %v", calcPercentile(nil, 0.50))
	}
}

// 3. Task accounting
func TestTaskAccounting(t *testing.T) {
	m := &ExperimentMetrics{
		SubmissionAttempts:  100,
		AcceptedSubmissions: 95,
		SubmissionErrors:    5,
		CompletedTasks:      80,
		FailedTasks:         5,
		TimedOutTasks:       10,
		PendingTasks:        4,
		ProcessingTasks:     6,
	}

	acct := m.Accounting()
	if acct.Accepted != 95 {
		t.Errorf("expected Accepted 95, got %d", acct.Accepted)
	}
	if acct.Completed != 80 {
		t.Errorf("expected Completed 80, got %d", acct.Completed)
	}
	if acct.TimedOut != 10 {
		t.Errorf("expected TimedOut 10, got %d", acct.TimedOut)
	}

	inv := acct.Invariant()
	if !strings.Contains(inv, "Accepted(95)") {
		t.Errorf("expected Invariant string to contain Accepted count, got %s", inv)
	}
	if !strings.Contains(inv, "[BALANCED]") {
		t.Errorf("expected Invariant to be balanced, got %s", inv)
	}
	if !m.AccountingValid() {
		t.Errorf("expected AccountingValid() to be true")
	}
}

// 4. Workload generation
func TestWorkloadGeneration(t *testing.T) {
	mixed := GenerateMixedWorkload(30, 0.1, 100, 500)
	if len(mixed) != 30 {
		t.Fatalf("expected 30 tasks, got %d", len(mixed))
	}
	if mixed[0].Type != "email.send" || mixed[1].Type != "report.generate" || mixed[2].Type != "image.process" {
		t.Errorf("unexpected round-robin distribution: %v, %v, %v", mixed[0].Type, mixed[1].Type, mixed[2].Type)
	}

	profile := WorkloadProfile{
		Name:          "custom",
		EmailPct:      0.5,
		ReportPct:     0.3,
		ImagePct:      0.2,
		MinDurationMs: 100,
		MaxDurationMs: 200,
	}
	profileTasks := GenerateProfileWorkload(100, profile, 0.05)
	if len(profileTasks) != 100 {
		t.Fatalf("expected 100 tasks, got %d", len(profileTasks))
	}

	var emailCount, reportCount, imageCount int
	for _, task := range profileTasks {
		switch task.Type {
		case "email.send":
			emailCount++
		case "report.generate":
			reportCount++
		case "image.process":
			imageCount++
		}
	}
	if emailCount != 50 || reportCount != 30 || imageCount != 20 {
		t.Errorf("unexpected profile distribution: email=%d, report=%d, image=%d", emailCount, reportCount, imageCount)
	}
}

// 5. Configuration validation
func TestConfigValidation(t *testing.T) {
	cfg := &BenchmarkConfig{
		APIURL:      "http://localhost:8080",
		ResultsDir:  "results",
		WorkerCount: 3,
		Suite:       "quick",
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected valid config, got error: %v", err)
	}

	badURL := &BenchmarkConfig{
		APIURL:      "",
		ResultsDir:  "results",
		WorkerCount: 3,
		Suite:       "quick",
	}
	if err := badURL.Validate(); err == nil {
		t.Errorf("expected error for empty APIURL, got nil")
	}

	badWorkers := &BenchmarkConfig{
		APIURL:      "http://localhost:8080",
		ResultsDir:  "results",
		WorkerCount: 0,
		Suite:       "quick",
	}
	if err := badWorkers.Validate(); err == nil {
		t.Errorf("expected error for non-positive WorkerCount, got nil")
	}

	badSuite := &BenchmarkConfig{
		APIURL:      "http://localhost:8080",
		ResultsDir:  "results",
		WorkerCount: 3,
		Suite:       "invalid-suite",
	}
	if err := badSuite.Validate(); err == nil {
		t.Errorf("expected error for invalid Suite, got nil")
	}
}

// 6. CSV output
func TestCSVGeneration(t *testing.T) {
	report := ExperimentReport{
		Name: "test-baseline",
		BenchConfig: &BenchmarkConfig{
			WorkerCount: 4,
		},
		Metrics: []ExperimentMetrics{
			{
				SubmissionAttempts:  50,
				AcceptedSubmissions: 50,
				CompletedTasks:      50,
				Latencies:           []time.Duration{100 * time.Millisecond},
			},
		},
	}

	csv := GenerateCSV(report)
	if !strings.Contains(csv, "test-baseline") {
		t.Errorf("CSV missing experiment name: %s", csv)
	}
	if !strings.Contains(csv, "completed_throughput") {
		t.Errorf("CSV missing header: %s", csv)
	}
}

// 7. Markdown report generation
func TestMarkdownReportGeneration(t *testing.T) {
	report := ExperimentReport{
		Name:        "test-markdown",
		Description: "A test experiment",
		BenchConfig: &BenchmarkConfig{
			APIURL:      "http://localhost:8080",
			WorkerCount: 2,
		},
		Metrics: []ExperimentMetrics{
			{
				SubmissionAttempts:  10,
				AcceptedSubmissions: 10,
				CompletedTasks:      10,
			},
		},
	}

	md := GenerateMarkdownReport(report)
	if !strings.Contains(md, "test-markdown") {
		t.Errorf("Markdown missing experiment name: %s", md)
	}
	if !strings.Contains(md, "Task Accounting") {
		t.Errorf("Markdown missing task accounting section")
	}
}

// 8. HTML report generation
func TestHTMLReportGeneration(t *testing.T) {
	report := ExperimentReport{
		Name:        "test-html",
		Description: "A test HTML experiment",
		BenchConfig: &BenchmarkConfig{
			WorkerCount: 2,
		},
		Metrics: []ExperimentMetrics{
			{
				SubmissionAttempts:  10,
				AcceptedSubmissions: 10,
				CompletedTasks:      10,
			},
		},
	}

	html := GenerateHTML(report)
	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Errorf("HTML missing doctype declaration")
	}
	if !strings.Contains(html, "test-html") {
		t.Errorf("HTML missing experiment name")
	}
}

// roundTripFunc implements http.RoundTripper as a function.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// 9. Error handling for malformed API responses
func TestMalformedAPIResponseHandling(t *testing.T) {
	fakeClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/health" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"status":"ok"}`)),
					Header:     make(http.Header),
				}, nil
			}
			if r.URL.Path == "/tasks" {
				return &http.Response{
					StatusCode: http.StatusAccepted,
					Body:       io.NopCloser(strings.NewReader("not valid json {")),
					Header:     make(http.Header),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	client := NewBenchmarkClientWithHTTP(fakeClient, "http://mock-gotask", 5, 5, false)
	defer client.Close()

	ctx := context.Background()
	_, err := client.SubmitTask(ctx, "email.send", TaskPayload{})
	if err == nil {
		t.Errorf("expected error for malformed JSON response, got nil")
	}
}

// 10. Context cancellation and deadlines
func TestContextCancellation(t *testing.T) {
	fakeClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			select {
			case <-r.Context().Done():
				return nil, r.Context().Err()
			case <-time.After(200 * time.Millisecond):
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"status":"ok"}`)),
					Header:     make(http.Header),
				}, nil
			}
		}),
	}

	client := NewBenchmarkClientWithHTTP(fakeClient, "http://mock-gotask", 2, 2, false)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	err := client.CheckHealth(ctx)
	if err == nil {
		t.Errorf("expected timeout/cancellation error, got nil")
	}
}

// 11. Bounded submission and polling concurrency
func TestBoundedConcurrency(t *testing.T) {
	var activeSubmissions int64
	var maxObservedSubmissions int64

	fakeClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/tasks" && r.Method == http.MethodPost {
				curr := atomic.AddInt64(&activeSubmissions, 1)
				defer atomic.AddInt64(&activeSubmissions, -1)

				for {
					max := atomic.LoadInt64(&maxObservedSubmissions)
					if curr <= max || atomic.CompareAndSwapInt64(&maxObservedSubmissions, max, curr) {
						break
					}
				}

				time.Sleep(20 * time.Millisecond)
				respBody := `{"id":"task_123","status":"pending"}`
				return &http.Response{
					StatusCode: http.StatusAccepted,
					Body:       io.NopCloser(strings.NewReader(respBody)),
					Header:     make(http.Header),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"status":"ok"}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	limit := 3
	client := NewBenchmarkClientWithHTTP(fakeClient, "http://mock-gotask", limit, limit, false)
	defer client.Close()

	specs := make([]TaskSpec, 20)
	for i := range specs {
		specs[i] = TaskSpec{Type: "email.send"}
	}

	ctx := context.Background()
	accepted, errs := client.SubmitBatch(ctx, specs)
	if errs != 0 {
		t.Errorf("unexpected submission errors: %d", errs)
	}
	if len(accepted) != 20 {
		t.Errorf("expected 20 accepted tasks, got %d", len(accepted))
	}

	maxObs := atomic.LoadInt64(&maxObservedSubmissions)
	if maxObs > int64(limit) {
		t.Errorf("concurrency exceeded limit: observed %d, limit was %d", maxObs, limit)
	}
}

// 12. Historical comparison compatibility
func TestHistoricalComparisonCompatibility(t *testing.T) {
	run1 := RunMetadata{
		RunID:       "run-1",
		Suite:       "quick",
		WorkerCount: 3,
		Experiments: []ExperimentSummary{
			{Name: "baseline", Status: "completed", MedianThroughput: 10.0, P50Latency: 100},
		},
	}

	run2 := RunMetadata{
		RunID:       "run-2",
		Suite:       "quick",
		WorkerCount: 3,
		Experiments: []ExperimentSummary{
			{Name: "baseline", Status: "completed", MedianThroughput: 15.0, P50Latency: 80},
		},
	}

	comparisons := CompareRuns(run2, run1)
	if len(comparisons) != 1 {
		t.Fatalf("expected 1 comparison, got %d", len(comparisons))
	}
	c := comparisons[0]
	if !c.Comparable {
		t.Errorf("expected runs to be comparable, reason: %s", c.IncompatibleReason)
	}
	if c.ThroughputDelta != 5.0 {
		t.Errorf("expected ThroughputDelta 5.0, got %f", c.ThroughputDelta)
	}

	// Incompatible worker count
	runDiffWorkers := RunMetadata{
		RunID:       "run-3",
		Suite:       "quick",
		WorkerCount: 8,
		Experiments: []ExperimentSummary{
			{Name: "baseline", Status: "completed", MedianThroughput: 25.0, P50Latency: 50, WorkerCount: 8},
		},
	}

	compDiff := CompareRuns(runDiffWorkers, run1)
	if compDiff[0].Comparable {
		t.Errorf("expected runs with different worker counts to be marked non-comparable")
	}

	// Check FindPreviousComparableRun
	tempDir, err := os.MkdirTemp("", "bench-history-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	runDir1 := filepath.Join(tempDir, "run-20260101-120000")
	_ = os.MkdirAll(runDir1, 0755)
	run1.RunID = "20260101-120000"
	run1.StartTime = time.Unix(1000, 0)
	_ = SaveRunMetadata(runDir1, run1)

	found, err := FindPreviousComparableRun(tempDir, RunMetadata{
		RunID:       "20260102-120000",
		Suite:       "quick",
		WorkerCount: 3,
	})
	if err != nil || found == nil {
		t.Errorf("expected to find comparable run, got %v (err: %v)", found, err)
	}
}
