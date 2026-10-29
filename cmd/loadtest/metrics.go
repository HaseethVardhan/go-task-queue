package main

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// ExperimentMetrics holds all raw and derived data collected during a load test experiment.
type ExperimentMetrics struct {
	// Submission phase counters.
	SubmissionAttempts  int
	AcceptedSubmissions int
	SubmissionErrors    int
	SubmissionDuration  time.Duration

	// Task terminal state counters.
	CompletedTasks int
	FailedTasks    int
	TimedOutTasks  int

	// Tasks still outstanding at deadline.
	PendingTasks    int
	ProcessingTasks int

	TotalElapsed time.Duration

	// Latencies records the end-to-end duration for every terminal task
	// (completed or failed). Must be sorted before calling percentile methods.
	Latencies []time.Duration

	// PollingErrors is incremented atomically during concurrent polling.
	PollingErrors int64

	// HTTPErrors counts non-retryable HTTP-level errors observed across all phases.
	HTTPErrors int

	// Phase timestamps.
	SubmissionStartTime time.Time
	SubmissionEndTime   time.Time
	PollingStartTime    time.Time
	PollingEndTime      time.Time
}

// AcceptedSubmissionRate returns the sustained submission throughput in tasks/second
// over the submission phase.
func (m *ExperimentMetrics) AcceptedSubmissionRate() float64 {
	dur := m.SubmissionEndTime.Sub(m.SubmissionStartTime).Seconds()
	if dur <= 0 {
		return 0
	}
	return float64(m.AcceptedSubmissions) / dur
}

// CompletedThroughput returns the end-to-end throughput of completed tasks in tasks/second,
// measured from the first submission to the last poll.
func (m *ExperimentMetrics) CompletedThroughput() float64 {
	elapsed := m.PollingEndTime.Sub(m.SubmissionStartTime).Seconds()
	if elapsed <= 0 {
		return 0
	}
	return float64(m.CompletedTasks) / elapsed
}

// AvgLatency returns the arithmetic mean of all recorded latencies.
func (m *ExperimentMetrics) AvgLatency() time.Duration {
	if len(m.Latencies) == 0 {
		return 0
	}
	var total time.Duration
	for _, l := range m.Latencies {
		total += l
	}
	return total / time.Duration(len(m.Latencies))
}

// P50Latency returns the median latency. Latencies must be sorted first.
func (m *ExperimentMetrics) P50Latency() time.Duration {
	return calcPercentile(m.Latencies, 0.50)
}

// P95Latency returns the 95th-percentile latency. Latencies must be sorted first.
func (m *ExperimentMetrics) P95Latency() time.Duration {
	return calcPercentile(m.Latencies, 0.95)
}

// P99Latency returns the 99th-percentile latency. Latencies must be sorted first.
func (m *ExperimentMetrics) P99Latency() time.Duration {
	return calcPercentile(m.Latencies, 0.99)
}

// OutstandingTasks returns the number of tasks that were still in a non-terminal
// state when the polling deadline was reached.
func (m *ExperimentMetrics) OutstandingTasks() int {
	return m.PendingTasks + m.ProcessingTasks
}

// TerminalTasks returns the number of tasks that reached a terminal state
// (completed or failed) during the experiment.
func (m *ExperimentMetrics) TerminalTasks() int {
	return m.CompletedTasks + m.FailedTasks
}

// AccountingValid checks the fundamental accounting invariant:
//
//	AcceptedSubmissions == TerminalTasks + OutstandingTasks + Unknown
//
// TimedOutTasks represents tasks whose final status was not reached before the deadline,
// partitioned into PendingTasks, ProcessingTasks, and Unknown.
func (m *ExperimentMetrics) AccountingValid() bool {
	acct := m.Accounting()
	return acct.Accepted == acct.Completed+acct.Failed+acct.Pending+acct.Processing+acct.Unknown
}

// SortLatencies sorts the Latencies slice in ascending order. This must be
// called before any percentile methods.
func (m *ExperimentMetrics) SortLatencies() {
	sort.Slice(m.Latencies, func(i, j int) bool {
		return m.Latencies[i] < m.Latencies[j]
	})
}

// calcPercentile returns the p-th percentile value from a pre-sorted slice of
// durations using the ceiling-rank method.
//
// rank = ceil(p * N), index = rank - 1, clamped to [0, N-1].
//
// Returns 0 for an empty slice.
func calcPercentile(sorted []time.Duration, p float64) time.Duration {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	rank := int(math.Ceil(p * float64(n)))
	idx := rank - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	return sorted[idx]
}

// TaskAccounting provides a self-consistent breakdown of every submitted task
// into mutually exclusive outcome buckets:
// Accepted tasks are partitioned into:
// - Terminal tasks (Completed, Failed)
// - Non-terminal tasks at deadline (Pending, Processing)
// - Tasks with unknown status at deadline (Unknown)
// Note: TimedOut represents the subset of accepted tasks that did not reach a terminal state
// before the deadline (TimedOut = Pending + Processing + Unknown).
type TaskAccounting struct {
	Submitted  int
	Accepted   int
	Rejected   int
	Completed  int
	Failed     int
	TimedOut   int
	Pending    int
	Processing int
	Unknown    int
}

// Invariant returns a human-readable accounting statement showing whether the
// fundamental identity holds:
//
//	Accepted = Completed + Failed + Pending + Processing + Unknown
func (a TaskAccounting) Invariant() string {
	lhs := a.Accepted
	rhs := a.Completed + a.Failed + a.Pending + a.Processing + a.Unknown
	balanced := "BALANCED"
	if lhs != rhs {
		balanced = fmt.Sprintf("UNBALANCED (diff=%d)", lhs-rhs)
	}
	return fmt.Sprintf(
		"Accepted(%d) = Completed(%d) + Failed(%d) + Pending(%d) + Processing(%d) + Unknown(%d) [TimedOut=%d]  =>  %d = %d  [%s]",
		a.Accepted, a.Completed, a.Failed, a.Pending, a.Processing, a.Unknown, a.TimedOut,
		lhs, rhs, balanced,
	)
}

// Accounting derives a TaskAccounting snapshot from the current metrics.
func (m *ExperimentMetrics) Accounting() TaskAccounting {
	rejected := m.SubmissionAttempts - m.AcceptedSubmissions

	// Unknown = timed-out tasks whose status was neither pending nor processing
	unknown := m.TimedOutTasks - (m.PendingTasks + m.ProcessingTasks)
	if unknown < 0 {
		unknown = 0
	}

	return TaskAccounting{
		Submitted:  m.SubmissionAttempts,
		Accepted:   m.AcceptedSubmissions,
		Rejected:   rejected,
		Completed:  m.CompletedTasks,
		Failed:     m.FailedTasks,
		TimedOut:   m.TimedOutTasks,
		Pending:    m.PendingTasks,
		Processing: m.ProcessingTasks,
		Unknown:    unknown,
	}
}
