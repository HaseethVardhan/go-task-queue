package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	cfg := ParseBenchmarkFlags()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	client := NewBenchmarkClient(
		cfg.APIURL,
		cfg.SubmissionConcurrency(),
		cfg.PollingConcurrency(),
		cfg.Verbose,
	)
	defer client.Close()

	ctx := context.Background()

	// 1. Health preflight check
	fmt.Printf("Checking GoTask API health at %s...\n", cfg.APIURL)
	preflightCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	err := client.CheckHealth(preflightCtx)
	cancel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n[FATAL] GoTask API is unreachable or unhealthy at %s: %v\n", cfg.APIURL, err)
		fmt.Fprintf(os.Stderr, "Diagnostic hints:\n")
		fmt.Fprintf(os.Stderr, "  1. Verify GoTask is running (`go run ./cmd/gotask/`).\n")
		fmt.Fprintf(os.Stderr, "  2. Verify the base URL flag `-url %s` is correct.\n", cfg.APIURL)
		fmt.Fprintf(os.Stderr, "  3. Verify PostgreSQL is up and accessible to the server.\n\n")
		os.Exit(1)
	}
	fmt.Println("API health check passed.")

	// 2. Setup run directory
	startTime := time.Now()
	runID := startTime.Format("20060102-150405")
	runDirName := fmt.Sprintf("run-%s", runID)
	runDir := filepath.Join(cfg.ResultsDir, runDirName)
	if err := os.MkdirAll(runDir, 0755); err != nil {
		log.Fatalf("Failed to create run directory %s: %v", runDir, err)
	}

	fmt.Printf("\n=======================================================\n")
	fmt.Printf(" GoTask Benchmark Suite: %s\n", strings.ToUpper(cfg.Suite))
	fmt.Printf(" Run ID:            %s\n", runID)
	fmt.Printf(" Server Workers:    %d (reported metadata)\n", cfg.WorkerCount)
	fmt.Printf(" Git Revision:      %s\n", cfg.GitRevision)
	fmt.Printf(" Results Directory: %s\n", runDir)
	fmt.Printf("=======================================================\n\n")

	// Determine list of experiments to run
	experimentsToRun := cfg.SelectedExperiments()
	fmt.Printf("Experiments scheduled (%d):\n", len(experimentsToRun))
	for i, name := range experimentsToRun {
		fmt.Printf("  %d. %s\n", i+1, name)
	}
	fmt.Println()

	var reports []ExperimentReport
	var suiteHasFailures bool

	for _, expName := range experimentsToRun {
		fmt.Printf("\n>>> Starting Experiment: %s\n", expName)
		expStart := time.Now()
		report := runSingleNamedExperiment(ctx, client, cfg, expName)
		report.EndTime = time.Now()
		if report.StartTime.IsZero() {
			report.StartTime = expStart
		}

		if report.Error != nil {
			suiteHasFailures = true
			fmt.Printf(">>> Experiment %s FAILED: %v\n", expName, report.Error)
		} else {
			fmt.Printf(">>> Experiment %s completed successfully (%s)\n", expName, formatDuration(time.Since(expStart)))
		}

		// Save per-experiment reports immediately
		expDir := filepath.Join(runDir, expName)
		if err := os.MkdirAll(expDir, 0755); err == nil {
			// Markdown
			md := GenerateMarkdownReport(report)
			_ = os.WriteFile(filepath.Join(expDir, "report.md"), []byte(md), 0644)

			// CSV
			csv := GenerateCSV(report)
			_ = os.WriteFile(filepath.Join(expDir, "metrics.csv"), []byte(csv), 0644)

			// HTML
			html := GenerateHTML(report)
			_ = os.WriteFile(filepath.Join(expDir, "report.html"), []byte(html), 0644)
		} else {
			fmt.Printf("Warning: failed to create experiment directory %s: %v\n", expDir, err)
		}

		reports = append(reports, report)
	}

	endTime := time.Now()

	// 3. Save run metadata & historical comparison
	meta := BuildRunMetadata(cfg, reports, runID, startTime, endTime)
	if err := SaveRunMetadata(runDir, meta); err != nil {
		fmt.Printf("Warning: failed to save run metadata: %v\n", err)
	}

	// 4. Generate overall run summaries
	summaryMD := GenerateSummaryMarkdown(reports, cfg, runDir, startTime, endTime)
	_ = os.WriteFile(filepath.Join(runDir, "summary.md"), []byte(summaryMD), 0644)

	summaryCSV := GenerateSummaryCSV(reports)
	_ = os.WriteFile(filepath.Join(runDir, "summary.csv"), []byte(summaryCSV), 0644)

	summaryHTML := GenerateSummaryHTML(reports, cfg, runDir, startTime, endTime)
	_ = os.WriteFile(filepath.Join(runDir, "summary.html"), []byte(summaryHTML), 0644)

	// 5. Update historical index
	allRuns, err := ListHistoricalRuns(cfg.ResultsDir)
	if err == nil {
		indexMD := GenerateIndexMarkdown(allRuns, cfg.ResultsDir)
		_ = os.WriteFile(filepath.Join(cfg.ResultsDir, "index.md"), []byte(indexMD), 0644)
	}

	// Historical comparison if a previous compatible run exists
	prevRun, _ := FindPreviousComparableRun(cfg.ResultsDir, meta)
	if prevRun != nil {
		comparisons := CompareRuns(meta, *prevRun)
		compMD := GenerateComparisonMarkdown(comparisons, meta.RunID, prevRun.RunID)
		_ = os.WriteFile(filepath.Join(runDir, "comparison.md"), []byte(compMD), 0644)
		fmt.Println("\nHistorical Comparison:")
		fmt.Printf("  Compared against previous run: %s\n", prevRun.RunID)
	}

	// 6. Print final summary
	totalDuration := endTime.Sub(startTime)
	fmt.Printf("\n=======================================================\n")
	fmt.Printf(" Benchmark Suite Completed in %s\n", formatDuration(totalDuration))
	fmt.Printf(" Results saved to:\n")
	fmt.Printf("   Directory:    %s\n", runDir)
	fmt.Printf("   Summary HTML: %s\n", filepath.Join(runDir, "summary.html"))
	fmt.Printf("   Summary MD:   %s\n", filepath.Join(runDir, "summary.md"))
	fmt.Printf("   Summary CSV:  %s\n", filepath.Join(runDir, "summary.csv"))
	fmt.Printf("   Index MD:     %s\n", filepath.Join(cfg.ResultsDir, "index.md"))
	fmt.Printf("=======================================================\n")

	if suiteHasFailures {
		fmt.Fprintf(os.Stderr, "\nBenchmark suite finished with errors.\n")
		os.Exit(1)
	}
}

func runSingleNamedExperiment(ctx context.Context, client *BenchmarkClient, bcfg *BenchmarkConfig, name string) ExperimentReport {
	isQuick := bcfg.IsQuick()
	report := ExperimentReport{
		Name:        name,
		BenchConfig: bcfg,
		StartTime:   time.Now(),
	}

	switch name {
	case "baseline":
		report.Description = "Establish baseline performance reference under mixed workload."
		cfg := DefaultBaselineConfig(isQuick)
		cfg.ExperimentConfig = bcfg.ApplyOverrides(cfg.ExperimentConfig)
		report.Config = cfg

		metrics, err := RunBaseline(ctx, client, *cfg)
		report.Metrics = metrics
		report.Error = err

	case "worker-scaling":
		report.Description = "Measure observed performance for the current server worker count."
		cfg := DefaultWorkerScalingConfig(isQuick)
		cfg.ReportedWorkerCount = bcfg.WorkerCount
		cfg.ExperimentConfig = bcfg.ApplyOverrides(cfg.ExperimentConfig)
		report.Config = cfg

		metrics, err := RunWorkerScaling(ctx, client, *cfg)
		report.Metrics = metrics
		report.Error = err
		report.Warnings = append(report.Warnings,
			fmt.Sprintf("Worker scaling measurement assumes GoTask was started with WORKER_COUNT=%d. The runner does not modify server configuration.", bcfg.WorkerCount))

	case "burst":
		report.Description = "Measure response when batches of tasks arrive quickly."
		cfg := DefaultBurstConfig(isQuick)
		cfg.ExperimentConfig = bcfg.ApplyOverrides(cfg.ExperimentConfig)
		report.Config = cfg

		subExp, err := RunBurst(ctx, client, *cfg)
		report.Error = err
		report.SubExperiments = make(map[string][]ExperimentMetrics)
		for size, ms := range subExp {
			report.SubExperiments[fmt.Sprintf("batch-%d", size)] = ms
		}

	case "workload-profiles":
		report.Description = "Evaluate GoTask throughput across varying task mix profiles."
		cfg := DefaultWorkloadProfileConfig(isQuick)
		cfg.ExperimentConfig = bcfg.ApplyOverrides(cfg.ExperimentConfig)
		report.Config = cfg

		subExp, err := RunWorkloadProfiles(ctx, client, *cfg)
		report.Error = err
		report.SubExperiments = subExp

	case "sustained-load":
		report.Description = "Evaluate performance under controlled, continuous task arrival rates."
		cfg := DefaultSustainedRateConfig(isQuick)
		cfg.ExperimentConfig = bcfg.ApplyOverrides(cfg.ExperimentConfig)
		report.Config = cfg

		subExp, err := RunSustainedRate(ctx, client, *cfg)
		report.Error = err
		report.SubExperiments = make(map[string][]ExperimentMetrics)
		for rate, ms := range subExp {
			report.SubExperiments[fmt.Sprintf("rate-%.1f", rate)] = ms
		}

	case "large-table":
		report.Description = "Historical large-table accumulation test."
		report.Warnings = append(report.Warnings,
			"DEFERRED: Testing true large-table performance without mutating or pre-populating database rows directly is deferred. In accordance with database safety constraints, the benchmark suite will not insert arbitrary synthetic historical rows outside normal task submission.")
		// We leave Metrics empty and error nil, marking it cleanly as deferred/skipped.

	default:
		report.Description = fmt.Sprintf("Custom experiment: %s", name)
		report.Error = fmt.Errorf("unknown experiment name: %s", name)
	}

	return report
}
