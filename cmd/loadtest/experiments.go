package main

import (
	"context"
	"log"
	"sort"
	"time"
)

// runExperiment is the core runner that all experiments use. It submits tasks,
// polls for results, and builds an ExperimentMetrics from the outcome.
func runExperiment(ctx context.Context, client *BenchmarkClient, tasks []TaskSpec, cfg ExperimentConfig) ExperimentMetrics {
	var m ExperimentMetrics

	// --- Submission phase ---
	m.SubmissionStartTime = time.Now()
	accepted, errorCount := client.SubmitBatch(ctx, tasks)
	m.SubmissionEndTime = time.Now()
	m.SubmissionDuration = m.SubmissionEndTime.Sub(m.SubmissionStartTime)

	m.SubmissionAttempts = len(tasks)
	m.AcceptedSubmissions = len(accepted)
	m.SubmissionErrors = errorCount

	// --- Polling phase ---
	m.PollingStartTime = time.Now()
	results := client.PollBatch(ctx, accepted, cfg.PollInterval)
	m.PollingEndTime = time.Now()

	// --- Categorise results ---
	for _, r := range results {
		if r.TimedOut {
			m.TimedOutTasks++
			switch r.Status {
			case "pending":
				m.PendingTasks++
			case "processing":
				m.ProcessingTasks++
			}
			continue
		}

		switch r.Status {
		case "completed":
			m.CompletedTasks++
			m.Latencies = append(m.Latencies, r.Latency)
		case "failed":
			m.FailedTasks++
			m.Latencies = append(m.Latencies, r.Latency)
		case "pending":
			m.PendingTasks++
		case "processing":
			m.ProcessingTasks++
		default:
			// Treat any unknown non-terminal status as failed
			m.FailedTasks++
			m.Latencies = append(m.Latencies, r.Latency)
		}
	}

	// --- Aggregate ---
	m.PollingErrors = client.PollingErrors()
	m.TotalElapsed = m.PollingEndTime.Sub(m.SubmissionStartTime)

	sort.Slice(m.Latencies, func(i, j int) bool {
		return m.Latencies[i] < m.Latencies[j]
	})

	client.ResetPollingErrors()

	return m
}

// RunBaseline runs a simple baseline experiment with a mixed workload,
// repeating cfg.Repetitions times.
func RunBaseline(ctx context.Context, client *BenchmarkClient, cfg BaselineConfig) ([]ExperimentMetrics, error) {
	metrics := make([]ExperimentMetrics, 0, cfg.Repetitions)

	for rep := 1; rep <= cfg.Repetitions; rep++ {
		log.Printf("Running baseline rep %d/%d...", rep, cfg.Repetitions)

		repCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
		tasks := GenerateMixedWorkload(cfg.TaskCount, cfg.FailureProbability, cfg.MinDurationMs, cfg.MaxDurationMs)
		m := runExperiment(repCtx, client, tasks, cfg.ExperimentConfig)
		cancel()

		metrics = append(metrics, m)
	}

	return metrics, nil
}

// RunBurst runs burst experiments for each batch size in cfg.BatchSizes,
// repeating cfg.Repetitions times per size.
func RunBurst(ctx context.Context, client *BenchmarkClient, cfg BurstConfig) (map[int][]ExperimentMetrics, error) {
	results := make(map[int][]ExperimentMetrics, len(cfg.BatchSizes))

	for _, batchSize := range cfg.BatchSizes {
		repMetrics := make([]ExperimentMetrics, 0, cfg.Repetitions)

		for rep := 1; rep <= cfg.Repetitions; rep++ {
			log.Printf("Running burst (size=%d) rep %d/%d...", batchSize, rep, cfg.Repetitions)

			repCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			tasks := GenerateBurstWorkload(batchSize, cfg.FailureProbability, cfg.MinDurationMs, cfg.MaxDurationMs)
			m := runExperiment(repCtx, client, tasks, cfg.ExperimentConfig)
			cancel()

			repMetrics = append(repMetrics, m)
		}

		results[batchSize] = repMetrics
	}

	return results, nil
}

// RunWorkerScaling runs repetitions with a mixed workload, intended for
// comparison across different server-side worker counts.
func RunWorkerScaling(ctx context.Context, client *BenchmarkClient, cfg WorkerScalingConfig) ([]ExperimentMetrics, error) {
	metrics := make([]ExperimentMetrics, 0, cfg.Repetitions)

	for rep := 1; rep <= cfg.Repetitions; rep++ {
		log.Printf("Running worker-scaling (workers=%d) rep %d/%d...", cfg.ReportedWorkerCount, rep, cfg.Repetitions)

		repCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
		tasks := GenerateMixedWorkload(cfg.TaskCount, cfg.FailureProbability, cfg.MinDurationMs, cfg.MaxDurationMs)
		m := runExperiment(repCtx, client, tasks, cfg.ExperimentConfig)
		cancel()

		metrics = append(metrics, m)
	}

	return metrics, nil
}

// RunWorkloadProfiles runs experiments for each workload profile, repeating
// cfg.Repetitions times per profile.
func RunWorkloadProfiles(ctx context.Context, client *BenchmarkClient, cfg WorkloadProfileConfig) (map[string][]ExperimentMetrics, error) {
	results := make(map[string][]ExperimentMetrics, len(cfg.Profiles))

	for _, profile := range cfg.Profiles {
		repMetrics := make([]ExperimentMetrics, 0, cfg.Repetitions)

		for rep := 1; rep <= cfg.Repetitions; rep++ {
			log.Printf("Running workload profile %q rep %d/%d...", profile.Name, rep, cfg.Repetitions)

			repCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			tasks := GenerateProfileWorkload(cfg.TaskCount, profile, cfg.FailureProbability)
			m := runExperiment(repCtx, client, tasks, cfg.ExperimentConfig)
			cancel()

			repMetrics = append(repMetrics, m)
		}

		results[profile.Name] = repMetrics
	}

	return results, nil
}

// RunSustainedRate runs rate-limited experiments for each target rate in
// cfg.Rates, repeating cfg.Repetitions times per rate.
func RunSustainedRate(ctx context.Context, client *BenchmarkClient, cfg SustainedRateConfig) (map[float64][]ExperimentMetrics, error) {
	results := make(map[float64][]ExperimentMetrics, len(cfg.Rates))

	for _, rate := range cfg.Rates {
		repMetrics := make([]ExperimentMetrics, 0, cfg.Repetitions)

		for rep := 1; rep <= cfg.Repetitions; rep++ {
			log.Printf("Running sustained rate (%.1f tasks/s) rep %d/%d...", rate, rep, cfg.Repetitions)

			repCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
			m := runSingleRateExperiment(repCtx, client, rate, cfg.Duration, cfg)
			cancel()

			repMetrics = append(repMetrics, m)
		}

		results[rate] = repMetrics
	}

	return results, nil
}

// taskTypes cycles through the three task types for rate-limited submission.
var taskTypes = []string{"email", "report", "image"}

// runSingleRateExperiment implements the rate-limited submission loop for a
// single rate value and duration, then polls all accepted tasks.
func runSingleRateExperiment(ctx context.Context, client *BenchmarkClient, rate float64, duration time.Duration, cfg SustainedRateConfig) ExperimentMetrics {
	interval := time.Duration(float64(time.Second) / rate)

	var m ExperimentMetrics
	m.SubmissionStartTime = time.Now()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	deadline := time.After(duration)

	var accepted []SubmittedTask
	var submissionAttempts int
	typeIdx := 0

submissionLoop:
	for {
		select {
		case <-ctx.Done():
			break submissionLoop
		case <-deadline:
			break submissionLoop
		case <-ticker.C:
			taskType := taskTypes[typeIdx%len(taskTypes)]
			typeIdx++

			spec := GenerateRateWorkload(taskType, cfg.FailureProbability, cfg.MinDurationMs, cfg.MaxDurationMs)
			submissionAttempts++

			submitted, err := client.SubmitTask(ctx, spec.Type, spec.Payload)
			if err != nil {
				m.SubmissionErrors++
				continue
			}
			accepted = append(accepted, submitted)
		}
	}

	m.SubmissionEndTime = time.Now()
	m.SubmissionDuration = m.SubmissionEndTime.Sub(m.SubmissionStartTime)
	m.SubmissionAttempts = submissionAttempts
	m.AcceptedSubmissions = len(accepted)

	// --- Polling phase ---
	m.PollingStartTime = time.Now()
	results := client.PollBatch(ctx, accepted, cfg.PollInterval)
	m.PollingEndTime = time.Now()

	for _, r := range results {
		if r.TimedOut {
			m.TimedOutTasks++
			switch r.Status {
			case "pending":
				m.PendingTasks++
			case "processing":
				m.ProcessingTasks++
			}
			continue
		}

		switch r.Status {
		case "completed":
			m.CompletedTasks++
			m.Latencies = append(m.Latencies, r.Latency)
		case "failed":
			m.FailedTasks++
			m.Latencies = append(m.Latencies, r.Latency)
		case "pending":
			m.PendingTasks++
		case "processing":
			m.ProcessingTasks++
		default:
			m.FailedTasks++
			m.Latencies = append(m.Latencies, r.Latency)
		}
	}

	m.PollingErrors = client.PollingErrors()
	m.TotalElapsed = m.PollingEndTime.Sub(m.SubmissionStartTime)

	sort.Slice(m.Latencies, func(i, j int) bool {
		return m.Latencies[i] < m.Latencies[j]
	})

	client.ResetPollingErrors()

	return m
}
