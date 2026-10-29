package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// RunMetadata captures high-level information about a benchmark run for
// historical tracking and cross-run comparison.
type RunMetadata struct {
	RunID       string              `json:"run_id"`
	StartTime   time.Time           `json:"start_time"`
	EndTime     time.Time           `json:"end_time"`
	Suite       string              `json:"suite"`
	WorkerCount int                 `json:"worker_count"`
	GitRevision string              `json:"git_revision"`
	APIURL      string              `json:"api_url"`
	Experiments []ExperimentSummary `json:"experiments"`
}

// ExperimentSummary holds the aggregated results of a single experiment across
// all of its repetitions.
type ExperimentSummary struct {
	Name             string  `json:"name"`
	Status           string  `json:"status"` // "completed", "failed", "skipped"
	TaskCount        int     `json:"task_count"`
	CompletedTasks   int     `json:"completed_tasks"`
	FailedTasks      int     `json:"failed_tasks"`
	TimedOutTasks    int     `json:"timed_out_tasks"`
	MedianThroughput float64 `json:"median_throughput"`
	P50Latency       float64 `json:"p50_latency_ms"`
	P95Latency       float64 `json:"p95_latency_ms"`
	P99Latency       float64 `json:"p99_latency_ms"`
	WorkerCount      int     `json:"worker_count"`
	Repetitions      int     `json:"repetitions"`
}

// Comparison holds the result of comparing a single experiment between two
// benchmark runs.
type Comparison struct {
	ExperimentName      string
	CurrentThroughput   float64
	PreviousThroughput  float64
	ThroughputDelta     float64
	ThroughputChangePct float64
	CurrentP50          float64
	PreviousP50         float64
	P50Delta            float64
	CurrentP95          float64
	PreviousP95         float64
	P95Delta            float64
	CurrentCompleted    int
	PreviousCompleted   int
	CurrentTimedOut     int
	PreviousTimedOut    int
	Comparable          bool
	IncompatibleReason  string
	CurrentWorkerCount  int
	PreviousWorkerCount int
	CurrentRepetitions  int
	PreviousRepetitions int
}

// SaveRunMetadata writes a RunMetadata to a metadata.json file inside runDir.
func SaveRunMetadata(runDir string, meta RunMetadata) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal run metadata: %w", err)
	}
	path := filepath.Join(runDir, "metadata.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// LoadRunMetadata reads and parses a metadata.json file from runDir.
func LoadRunMetadata(runDir string) (RunMetadata, error) {
	path := filepath.Join(runDir, "metadata.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return RunMetadata{}, fmt.Errorf("read %s: %w", path, err)
	}
	var meta RunMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return RunMetadata{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return meta, nil
}

// BuildRunMetadata constructs a RunMetadata from the benchmark configuration
// and the collected experiment reports. Metrics are aggregated across all
// repetitions of each experiment.
func BuildRunMetadata(cfg *BenchmarkConfig, reports []ExperimentReport, runID string, startTime, endTime time.Time) RunMetadata {
	meta := RunMetadata{
		RunID:       runID,
		StartTime:   startTime,
		EndTime:     endTime,
		Suite:       cfg.Suite,
		WorkerCount: cfg.WorkerCount,
		GitRevision: cfg.GitRevision,
		APIURL:      cfg.APIURL,
	}

	for _, report := range reports {
		summary := buildExperimentSummary(report, cfg.WorkerCount)
		meta.Experiments = append(meta.Experiments, summary)
	}

	return meta
}

// buildExperimentSummary creates an ExperimentSummary from a single
// ExperimentReport, aggregating across all repetitions.
func buildExperimentSummary(report ExperimentReport, workerCount int) ExperimentSummary {
	summary := ExperimentSummary{
		Name:        report.Name,
		WorkerCount: workerCount,
		Repetitions: len(report.Metrics),
	}

	if report.Error != nil {
		summary.Status = "failed"
		return summary
	}

	if len(report.Metrics) == 0 {
		summary.Status = "skipped"
		return summary
	}

	summary.Status = "completed"

	// Aggregate task counts across repetitions.
	var throughputs []float64
	var p50s, p95s, p99s []float64

	for _, m := range report.Metrics {
		summary.TaskCount += m.AcceptedSubmissions
		summary.CompletedTasks += m.CompletedTasks
		summary.FailedTasks += m.FailedTasks
		summary.TimedOutTasks += m.TimedOutTasks
		throughputs = append(throughputs, m.CompletedThroughput())
		p50s = append(p50s, float64(m.P50Latency())/float64(time.Millisecond))
		p95s = append(p95s, float64(m.P95Latency())/float64(time.Millisecond))
		p99s = append(p99s, float64(m.P99Latency())/float64(time.Millisecond))
	}

	summary.MedianThroughput = medianFloat64(throughputs)
	summary.P50Latency = medianFloat64(p50s)
	summary.P95Latency = medianFloat64(p95s)
	summary.P99Latency = medianFloat64(p99s)

	return summary
}

// ListHistoricalRuns scans resultsDir for run-* directories, loads their
// metadata, and returns them sorted by start time (most recent first).
// Directories without valid metadata are silently skipped.
func ListHistoricalRuns(resultsDir string) ([]RunMetadata, error) {
	entries, err := os.ReadDir(resultsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read results directory: %w", err)
	}

	var runs []RunMetadata
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "run-") {
			continue
		}
		meta, err := LoadRunMetadata(filepath.Join(resultsDir, entry.Name()))
		if err != nil {
			continue // silently skip
		}
		runs = append(runs, meta)
	}

	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartTime.After(runs[j].StartTime)
	})

	return runs, nil
}

// CompareRuns compares each experiment in the current run against the matching
// experiment in the previous run. Experiments are matched by name.
func CompareRuns(current, previous RunMetadata) []Comparison {
	prevByName := make(map[string]ExperimentSummary, len(previous.Experiments))
	for _, exp := range previous.Experiments {
		prevByName[exp.Name] = exp
	}

	var comparisons []Comparison
	for _, cur := range current.Experiments {
		c := Comparison{
			ExperimentName:     cur.Name,
			CurrentThroughput:  cur.MedianThroughput,
			CurrentP50:         cur.P50Latency,
			CurrentP95:         cur.P95Latency,
			CurrentCompleted:   cur.CompletedTasks,
			CurrentTimedOut:    cur.TimedOutTasks,
			CurrentWorkerCount: cur.WorkerCount,
			CurrentRepetitions: cur.Repetitions,
		}

		prev, found := prevByName[cur.Name]
		if !found {
			c.Comparable = false
			c.IncompatibleReason = "experiment not found in previous run"
			comparisons = append(comparisons, c)
			continue
		}

		c.PreviousThroughput = prev.MedianThroughput
		c.PreviousP50 = prev.P50Latency
		c.PreviousP95 = prev.P95Latency
		c.PreviousCompleted = prev.CompletedTasks
		c.PreviousTimedOut = prev.TimedOutTasks
		c.PreviousWorkerCount = prev.WorkerCount
		c.PreviousRepetitions = prev.Repetitions

		// Check compatibility.
		switch {
		case cur.WorkerCount != prev.WorkerCount:
			c.Comparable = false
			c.IncompatibleReason = fmt.Sprintf("different worker counts: %d vs %d", cur.WorkerCount, prev.WorkerCount)
		case cur.Status != "completed" || prev.Status != "completed":
			c.Comparable = false
			c.IncompatibleReason = fmt.Sprintf("experiment status: current=%s, previous=%s", cur.Status, prev.Status)
		default:
			c.Comparable = true
		}

		// Calculate deltas regardless of comparability so callers can
		// inspect the raw numbers.
		c.ThroughputDelta = cur.MedianThroughput - prev.MedianThroughput
		if prev.MedianThroughput > 0 {
			c.ThroughputChangePct = (c.ThroughputDelta / prev.MedianThroughput) * 100
		}
		c.P50Delta = cur.P50Latency - prev.P50Latency
		c.P95Delta = cur.P95Latency - prev.P95Latency

		comparisons = append(comparisons, c)
	}

	return comparisons
}

// FindPreviousComparableRun locates the most recent historical run that shares
// the same suite type and worker count as the current run, excluding the
// current run itself. Returns nil if no comparable run is found.
func FindPreviousComparableRun(resultsDir string, current RunMetadata) (*RunMetadata, error) {
	runs, err := ListHistoricalRuns(resultsDir)
	if err != nil {
		return nil, err
	}

	for i := range runs {
		r := &runs[i]
		if r.RunID == current.RunID {
			continue
		}
		if r.Suite != current.Suite {
			continue
		}
		if r.WorkerCount != current.WorkerCount {
			continue
		}
		return r, nil
	}

	return nil, nil
}

// GenerateIndexMarkdown produces a top-level index.md summarising all
// historical benchmark runs in a Markdown table.
func GenerateIndexMarkdown(runs []RunMetadata, resultsDir string) string {
	var sb strings.Builder

	sb.WriteString("# GoTask Benchmark Results\n\n")

	if len(runs) == 0 {
		sb.WriteString("No benchmark runs recorded yet.\n")
		return sb.String()
	}

	sb.WriteString("| Date | Suite | Workers | Git Rev | Experiments | Status |\n")
	sb.WriteString("|------|-------|---------|---------|-------------|--------|\n")

	for _, run := range runs {
		date := run.StartTime.Format("2006-01-02 15:04")
		gitRev := run.GitRevision
		if len(gitRev) > 8 {
			gitRev = gitRev[:8]
		}
		if gitRev == "" {
			gitRev = "unknown"
		}

		expCount := len(run.Experiments)
		completed := 0
		failed := 0
		for _, exp := range run.Experiments {
			switch exp.Status {
			case "completed":
				completed++
			case "failed":
				failed++
			}
		}

		var status string
		switch {
		case failed > 0:
			status = fmt.Sprintf("⚠ %d/%d failed", failed, expCount)
		case completed == expCount:
			status = "✓ all passed"
		default:
			status = fmt.Sprintf("%d/%d completed", completed, expCount)
		}

		// Build a relative link from the index to the run's summary.
		runDir := fmt.Sprintf("run-%s", run.RunID)
		link := fmt.Sprintf("[%s](%s/summary.md)", date, runDir)

		sb.WriteString(fmt.Sprintf("| %s | %s | %d | `%s` | %d | %s |\n",
			link, run.Suite, run.WorkerCount, gitRev, expCount, status))
	}

	return sb.String()
}

// GenerateComparisonMarkdown produces a Markdown section comparing two
// benchmark runs, highlighting improvements and regressions.
func GenerateComparisonMarkdown(comparisons []Comparison, currentRunID, previousRunID string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Comparison: `%s` vs `%s`\n\n", currentRunID, previousRunID))

	if len(comparisons) == 0 {
		sb.WriteString("No experiments to compare.\n")
		return sb.String()
	}

	// Main throughput/latency table.
	sb.WriteString("| Experiment | Throughput (cur) | Throughput (prev) | Delta | Change | P50 (cur) | P50 (prev) | P95 (cur) | P95 (prev) |\n")
	sb.WriteString("|------------|-----------------|-------------------|-------|--------|-----------|------------|-----------|------------|\n")

	for _, c := range comparisons {
		if !c.Comparable {
			sb.WriteString(fmt.Sprintf("| %s | %.1f | %.1f | — | — | — | — | — | — |\n",
				c.ExperimentName, c.CurrentThroughput, c.PreviousThroughput))
			continue
		}

		arrow := directionArrow(c.ThroughputChangePct, true)
		sb.WriteString(fmt.Sprintf("| %s | %.1f | %.1f | %+.1f | %s %.1f%% | %.1f ms | %.1f ms | %.1f ms | %.1f ms |\n",
			c.ExperimentName,
			c.CurrentThroughput, c.PreviousThroughput,
			c.ThroughputDelta, arrow, math.Abs(c.ThroughputChangePct),
			c.CurrentP50, c.PreviousP50,
			c.CurrentP95, c.PreviousP95,
		))
	}

	// Non-comparable notes.
	var notes []string
	for _, c := range comparisons {
		if !c.Comparable {
			notes = append(notes, fmt.Sprintf("- **%s**: %s", c.ExperimentName, c.IncompatibleReason))
		}
	}
	if len(notes) > 0 {
		sb.WriteString("\n### Non-comparable experiments\n\n")
		for _, n := range notes {
			sb.WriteString(n + "\n")
		}
	}

	sb.WriteString("\n> **Note:** Benchmark results can be noisy. Small percentage changes (< 5%) may not be statistically significant, especially with a single observation per experiment.\n")

	return sb.String()
}

// directionArrow returns an arrow character indicating the direction of a
// percentage change. When higherIsBetter is true, positive changes get ↑ and
// negative get ↓; the reverse applies when higherIsBetter is false.
func directionArrow(pct float64, higherIsBetter bool) string {
	if pct > 0 {
		if higherIsBetter {
			return "↑"
		}
		return "↓"
	}
	if pct < 0 {
		if higherIsBetter {
			return "↓"
		}
		return "↑"
	}
	return "→"
}

// medianFloat64 returns the median value of a float64 slice. It returns 0 for
// an empty slice. The input slice is not modified.
func medianFloat64(values []float64) float64 {
	n := len(values)
	if n == 0 {
		return 0
	}

	sorted := make([]float64, n)
	copy(sorted, values)
	sort.Float64s(sorted)

	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}
