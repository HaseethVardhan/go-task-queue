package main

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// ExperimentReport aggregates configuration, metrics, and metadata for a single
// experiment across all its repetitions and optional sub-experiments.
type ExperimentReport struct {
	Name           string
	Description    string
	Config         interface{} // the experiment-specific config
	BenchConfig    *BenchmarkConfig
	Metrics        []ExperimentMetrics            // one per repetition
	SubExperiments map[string][]ExperimentMetrics // for burst (by batch size), profiles (by name), rates (by rate)
	StartTime      time.Time
	EndTime        time.Time
	Warnings       []string
	Error          error
}

// ---------------------------------------------------------------------------
// Helper formatters
// ---------------------------------------------------------------------------

// formatDuration renders a time.Duration as a human-readable string with
// millisecond precision.
func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%.1fms", float64(d.Nanoseconds())/1e6)
	}
	if d < time.Minute {
		return fmt.Sprintf("%.3fs", d.Seconds())
	}
	minutes := int(d.Minutes())
	secs := d.Seconds() - float64(minutes*60)
	return fmt.Sprintf("%dm%.1fs", minutes, secs)
}

// formatRate formats a throughput value as "X.XX tasks/s".
func formatRate(r float64) string {
	return fmt.Sprintf("%.2f tasks/s", r)
}

// formatLatencyMs formats a duration in milliseconds with one decimal place.
func formatLatencyMs(d time.Duration) string {
	return fmt.Sprintf("%.1f ms", float64(d.Nanoseconds())/1e6)
}

// ---------------------------------------------------------------------------
// SVG bar chart
// ---------------------------------------------------------------------------

// svgBarChart generates an inline SVG vertical bar chart. Bars are coloured
// using a blue/green palette. The chart is completely self-contained and
// requires no external resources.
func svgBarChart(title string, labels []string, values []float64, unit string, width, height int) string {
	if len(labels) == 0 || len(values) == 0 {
		return ""
	}
	n := len(labels)
	if len(values) < n {
		n = len(values)
	}

	var b strings.Builder

	// Palette: alternating blue/green shades.
	palette := []string{"#3b82f6", "#10b981", "#6366f1", "#14b8a6", "#2563eb", "#059669"}

	// Chart layout constants.
	padLeft := 70
	padRight := 20
	padTop := 40
	padBottom := 60
	chartW := width - padLeft - padRight
	chartH := height - padTop - padBottom

	// Find max value for Y-axis scaling.
	maxVal := 0.0
	for i := 0; i < n; i++ {
		if values[i] > maxVal {
			maxVal = values[i]
		}
	}
	if maxVal <= 0 {
		maxVal = 1
	}
	// Round up max to a nice number.
	niceMax := niceAxisMax(maxVal)

	barWidth := chartW / n
	barPad := barWidth / 5
	if barPad < 4 {
		barPad = 4
	}

	b.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" style="font-family:sans-serif;background:#fff;">`, width, height, width, height))
	b.WriteByte('\n')

	// Title.
	b.WriteString(fmt.Sprintf(`  <text x="%d" y="%d" text-anchor="middle" font-size="14" font-weight="bold" fill="#1e293b">%s</text>`, width/2, 22, escapeXML(title)))
	b.WriteByte('\n')

	// Y-axis gridlines and labels.
	gridLines := 5
	for i := 0; i <= gridLines; i++ {
		yFrac := float64(i) / float64(gridLines)
		yVal := niceMax * (1.0 - yFrac)
		yPos := padTop + int(yFrac*float64(chartH))
		b.WriteString(fmt.Sprintf(`  <line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#e2e8f0" stroke-width="1"/>`, padLeft, yPos, padLeft+chartW, yPos))
		b.WriteByte('\n')
		b.WriteString(fmt.Sprintf(`  <text x="%d" y="%d" text-anchor="end" font-size="10" fill="#64748b">%.1f</text>`, padLeft-6, yPos+4, yVal))
		b.WriteByte('\n')
	}

	// Bars.
	for i := 0; i < n; i++ {
		x := padLeft + i*barWidth + barPad
		bw := barWidth - 2*barPad
		if bw < 2 {
			bw = 2
		}
		barH := int(float64(chartH) * (values[i] / niceMax))
		if barH < 1 && values[i] > 0 {
			barH = 1
		}
		y := padTop + chartH - barH
		colour := palette[i%len(palette)]

		b.WriteString(fmt.Sprintf(`  <rect x="%d" y="%d" width="%d" height="%d" fill="%s" rx="3"/>`, x, y, bw, barH, colour))
		b.WriteByte('\n')

		// Value label above bar.
		b.WriteString(fmt.Sprintf(`  <text x="%d" y="%d" text-anchor="middle" font-size="10" fill="#1e293b">%.2f</text>`, x+bw/2, y-4, values[i]))
		b.WriteByte('\n')

		// X-axis label.
		labelY := padTop + chartH + 16
		label := labels[i]
		if len(label) > 12 {
			label = label[:12] + "…"
		}
		b.WriteString(fmt.Sprintf(`  <text x="%d" y="%d" text-anchor="middle" font-size="10" fill="#475569" transform="rotate(-25,%d,%d)">%s</text>`, x+bw/2, labelY, x+bw/2, labelY, escapeXML(label)))
		b.WriteByte('\n')
	}

	// X-axis line.
	b.WriteString(fmt.Sprintf(`  <line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#94a3b8" stroke-width="1"/>`, padLeft, padTop+chartH, padLeft+chartW, padTop+chartH))
	b.WriteByte('\n')
	// Y-axis line.
	b.WriteString(fmt.Sprintf(`  <line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#94a3b8" stroke-width="1"/>`, padLeft, padTop, padLeft, padTop+chartH))
	b.WriteByte('\n')

	// Unit label.
	b.WriteString(fmt.Sprintf(`  <text x="%d" y="%d" text-anchor="middle" font-size="10" fill="#64748b">%s</text>`, width/2, height-4, escapeXML(unit)))
	b.WriteByte('\n')

	b.WriteString("</svg>\n")
	return b.String()
}

// niceAxisMax rounds v up to a "nice" number for axis display.
func niceAxisMax(v float64) float64 {
	if v <= 0 {
		return 1
	}
	exp := math.Floor(math.Log10(v))
	base := math.Pow(10, exp)
	frac := v / base
	switch {
	case frac <= 1.0:
		return base
	case frac <= 2.0:
		return 2 * base
	case frac <= 5.0:
		return 5 * base
	default:
		return 10 * base
	}
}

// escapeXML escapes special characters for safe embedding in SVG/XML.
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

// ---------------------------------------------------------------------------
// Markdown Report
// ---------------------------------------------------------------------------

// GenerateMarkdownReport generates a complete Markdown report for a single
// experiment including configuration, metrics, latency percentiles, task
// accounting, throughput, warnings, and interpretation notes.
func GenerateMarkdownReport(report ExperimentReport) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("## Experiment: %s\n\n", report.Name))

	if report.Description != "" {
		b.WriteString(fmt.Sprintf("%s\n\n", report.Description))
	}

	// Date/Time.
	b.WriteString(fmt.Sprintf("**Date:** %s  \n", report.StartTime.Format("2006-01-02 15:04:05 MST")))
	b.WriteString(fmt.Sprintf("**Duration:** %s  \n\n", formatDuration(report.EndTime.Sub(report.StartTime))))

	// Environment.
	if report.BenchConfig != nil {
		b.WriteString("### Environment\n\n")
		b.WriteString(fmt.Sprintf("| Parameter | Value |\n"))
		b.WriteString(fmt.Sprintf("| --- | --- |\n"))
		b.WriteString(fmt.Sprintf("| API URL | `%s` |\n", report.BenchConfig.APIURL))
		b.WriteString(fmt.Sprintf("| Git Revision | `%s` |\n", report.BenchConfig.GitRevision))
		b.WriteString(fmt.Sprintf("| Worker Count | %d |\n", report.BenchConfig.WorkerCount))
		b.WriteString(fmt.Sprintf("| Suite | %s |\n", report.BenchConfig.Suite))
		b.WriteByte('\n')
	}

	// Test parameters.
	b.WriteString("### Test Parameters\n\n")
	writeMarkdownExperimentConfigTable(&b, report.Config)

	// Error banner.
	if report.Error != nil {
		b.WriteString(fmt.Sprintf("\n> **ERROR:** %s\n\n", report.Error.Error()))
	}

	// Main results table.
	if len(report.Metrics) > 0 {
		b.WriteString("### Results\n\n")
		writeMarkdownMetricsTable(&b, "", report.Metrics)
		b.WriteByte('\n')
	}

	// Sub-experiments.
	if len(report.SubExperiments) > 0 {
		b.WriteString("### Sub-Experiments\n\n")
		for name, metrics := range report.SubExperiments {
			b.WriteString(fmt.Sprintf("#### %s\n\n", name))
			writeMarkdownMetricsTable(&b, name, metrics)
			b.WriteByte('\n')
		}
	}

	// Latency percentiles.
	allMetrics := collectAllMetrics(report)
	if len(allMetrics) > 0 {
		b.WriteString("### Latency Percentiles\n\n")
		b.WriteString("| Run | Observations | Avg | P50 | P95 | P99 |\n")
		b.WriteString("| --- | ---: | ---: | ---: | ---: | ---: |\n")
		idx := 0
		if len(report.SubExperiments) > 0 {
			for name, metrics := range report.SubExperiments {
				for i, m := range metrics {
					label := fmt.Sprintf("%s #%d", name, i+1)
					b.WriteString(fmt.Sprintf("| %s | %d | %s | %s | %s | %s |\n",
						label, len(m.Latencies),
						formatLatencyMs(m.AvgLatency()),
						formatLatencyMs(m.P50Latency()),
						formatLatencyMs(m.P95Latency()),
						formatLatencyMs(m.P99Latency()),
					))
				}
			}
		} else {
			for _, m := range report.Metrics {
				idx++
				b.WriteString(fmt.Sprintf("| Rep %d | %d | %s | %s | %s | %s |\n",
					idx, len(m.Latencies),
					formatLatencyMs(m.AvgLatency()),
					formatLatencyMs(m.P50Latency()),
					formatLatencyMs(m.P95Latency()),
					formatLatencyMs(m.P99Latency()),
				))
			}
		}
		b.WriteByte('\n')
	}

	// Task accounting.
	if len(allMetrics) > 0 {
		b.WriteString("### Task Accounting\n\n")
		for i, m := range allMetrics {
			a := m.Accounting()
			if len(allMetrics) > 1 {
				b.WriteString(fmt.Sprintf("**Run %d:**  \n", i+1))
			}
			b.WriteString(fmt.Sprintf("```\n%s\n```\n\n", a.Invariant()))
		}
	}

	// Throughput & submission rate.
	if len(allMetrics) > 0 {
		b.WriteString("### Throughput & Submission Rate\n\n")
		b.WriteString("| Run | Submission Rate | Completed Throughput |\n")
		b.WriteString("| --- | ---: | ---: |\n")
		for i, m := range allMetrics {
			b.WriteString(fmt.Sprintf("| %d | %s | %s |\n",
				i+1,
				formatRate(m.AcceptedSubmissionRate()),
				formatRate(m.CompletedThroughput()),
			))
		}
		b.WriteByte('\n')
	}

	// Errors section.
	if len(allMetrics) > 0 {
		hasErrors := false
		for _, m := range allMetrics {
			if m.SubmissionErrors > 0 || m.PollingErrors > 0 || m.HTTPErrors > 0 {
				hasErrors = true
				break
			}
		}
		if hasErrors {
			b.WriteString("### Errors\n\n")
			b.WriteString("| Run | Submission Errors | Polling Errors | HTTP Errors |\n")
			b.WriteString("| --- | ---: | ---: | ---: |\n")
			for i, m := range allMetrics {
				b.WriteString(fmt.Sprintf("| %d | %d | %d | %d |\n",
					i+1, m.SubmissionErrors, m.PollingErrors, m.HTTPErrors,
				))
			}
			b.WriteByte('\n')
		}
	}

	// Warnings.
	if len(report.Warnings) > 0 {
		b.WriteString("### Warnings\n\n")
		for _, w := range report.Warnings {
			b.WriteString(fmt.Sprintf("- ⚠ %s\n", w))
		}
		b.WriteByte('\n')
	}

	// Interpretation notes.
	b.WriteString("### Interpretation Notes\n\n")
	b.WriteString("- Latency measurements include polling interval and HTTP overhead\n")
	b.WriteString("- Handler simulates execution time; does not represent real workload\n")
	totalTimedOut := 0
	for _, m := range allMetrics {
		totalTimedOut += m.TimedOutTasks
	}
	if totalTimedOut > 0 {
		b.WriteString(fmt.Sprintf("- %d tasks timed out — latency statistics represent only completed tasks\n", totalTimedOut))
	}
	b.WriteByte('\n')

	return b.String()
}

// writeMarkdownMetricsTable writes a Markdown table of per-repetition metrics.
func writeMarkdownMetricsTable(b *strings.Builder, label string, metrics []ExperimentMetrics) {
	b.WriteString("| Rep | Submitted | Accepted | Completed | Failed | Timed Out | Pending | Processing | Elapsed | Sub Rate | Throughput | Avg Latency | P50 | P95 | P99 |\n")
	b.WriteString("| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n")

	for i, m := range metrics {
		b.WriteString(fmt.Sprintf("| %d | %d | %d | %d | %d | %d | %d | %d | %s | %s | %s | %s | %s | %s | %s |\n",
			i+1,
			m.SubmissionAttempts,
			m.AcceptedSubmissions,
			m.CompletedTasks,
			m.FailedTasks,
			m.TimedOutTasks,
			m.PendingTasks,
			m.ProcessingTasks,
			formatDuration(m.TotalElapsed),
			formatRate(m.AcceptedSubmissionRate()),
			formatRate(m.CompletedThroughput()),
			formatLatencyMs(m.AvgLatency()),
			formatLatencyMs(m.P50Latency()),
			formatLatencyMs(m.P95Latency()),
			formatLatencyMs(m.P99Latency()),
		))
	}
}

// writeMarkdownExperimentConfigTable writes a table of experiment config parameters.
func writeMarkdownExperimentConfigTable(b *strings.Builder, cfg interface{}) {
	b.WriteString("| Parameter | Value |\n")
	b.WriteString("| --- | --- |\n")

	switch c := cfg.(type) {
	case *BaselineConfig:
		writeExperimentBaseParams(b, &c.ExperimentConfig)
		b.WriteString(fmt.Sprintf("| Min Duration (ms) | %d |\n", c.MinDurationMs))
		b.WriteString(fmt.Sprintf("| Max Duration (ms) | %d |\n", c.MaxDurationMs))
	case *BurstConfig:
		writeExperimentBaseParams(b, &c.ExperimentConfig)
		b.WriteString(fmt.Sprintf("| Batch Sizes | %v |\n", c.BatchSizes))
		b.WriteString(fmt.Sprintf("| Min Duration (ms) | %d |\n", c.MinDurationMs))
		b.WriteString(fmt.Sprintf("| Max Duration (ms) | %d |\n", c.MaxDurationMs))
	case *WorkerScalingConfig:
		writeExperimentBaseParams(b, &c.ExperimentConfig)
		b.WriteString(fmt.Sprintf("| Reported Worker Count | %d |\n", c.ReportedWorkerCount))
		b.WriteString(fmt.Sprintf("| Min Duration (ms) | %d |\n", c.MinDurationMs))
		b.WriteString(fmt.Sprintf("| Max Duration (ms) | %d |\n", c.MaxDurationMs))
	case *WorkloadProfileConfig:
		writeExperimentBaseParams(b, &c.ExperimentConfig)
		for _, p := range c.Profiles {
			b.WriteString(fmt.Sprintf("| Profile: %s | email=%.0f%% report=%.0f%% image=%.0f%% |\n",
				p.Name, p.EmailPct*100, p.ReportPct*100, p.ImagePct*100))
		}
	case *SustainedRateConfig:
		writeExperimentBaseParams(b, &c.ExperimentConfig)
		b.WriteString(fmt.Sprintf("| Rates (tasks/s) | %v |\n", c.Rates))
		b.WriteString(fmt.Sprintf("| Rate Duration | %s |\n", c.Duration))
		b.WriteString(fmt.Sprintf("| Min Duration (ms) | %d |\n", c.MinDurationMs))
		b.WriteString(fmt.Sprintf("| Max Duration (ms) | %d |\n", c.MaxDurationMs))
	case *ExperimentConfig:
		writeExperimentBaseParams(b, c)
	default:
		b.WriteString(fmt.Sprintf("| Config | %v |\n", cfg))
	}
	b.WriteByte('\n')
}

// writeExperimentBaseParams writes the common ExperimentConfig fields as Markdown table rows.
func writeExperimentBaseParams(b *strings.Builder, ec *ExperimentConfig) {
	b.WriteString(fmt.Sprintf("| Name | %s |\n", ec.Name))
	b.WriteString(fmt.Sprintf("| Task Count | %d |\n", ec.TaskCount))
	b.WriteString(fmt.Sprintf("| Submission Concurrency | %d |\n", ec.SubmissionConcurrency))
	b.WriteString(fmt.Sprintf("| Polling Concurrency | %d |\n", ec.PollingConcurrency))
	b.WriteString(fmt.Sprintf("| Poll Interval | %s |\n", ec.PollInterval))
	b.WriteString(fmt.Sprintf("| Timeout | %s |\n", ec.Timeout))
	b.WriteString(fmt.Sprintf("| Failure Probability | %.2f |\n", ec.FailureProbability))
	b.WriteString(fmt.Sprintf("| Repetitions | %d |\n", ec.Repetitions))
}

// collectAllMetrics returns a flat slice of all metrics from both the main
// repetitions and any sub-experiments.
func collectAllMetrics(report ExperimentReport) []ExperimentMetrics {
	var all []ExperimentMetrics
	all = append(all, report.Metrics...)
	for _, ms := range report.SubExperiments {
		all = append(all, ms...)
	}
	return all
}

// ---------------------------------------------------------------------------
// CSV Report
// ---------------------------------------------------------------------------

// GenerateCSV generates a CSV representation of the experiment report with one
// row per repetition (and optionally per sub-experiment).
func GenerateCSV(report ExperimentReport) string {
	var b strings.Builder

	hasSubExperiments := len(report.SubExperiments) > 0

	// Header.
	if hasSubExperiments {
		b.WriteString("experiment,sub_experiment,repetition,")
	} else {
		b.WriteString("experiment,repetition,")
	}
	b.WriteString("task_count,submission_concurrency,poll_interval_ms,timeout_s,failure_probability,worker_count,")
	b.WriteString("submission_attempts,accepted,submission_errors,completed,failed,timed_out,pending,processing,")
	b.WriteString("total_elapsed_s,submission_duration_s,accepted_rate,completed_throughput,")
	b.WriteString("avg_latency_ms,p50_latency_ms,p95_latency_ms,p99_latency_ms,polling_errors,latency_observations\n")

	ec := extractExperimentConfig(report.Config)
	workerCount := 0
	if report.BenchConfig != nil {
		workerCount = report.BenchConfig.WorkerCount
	}

	writeCSVRow := func(subName string, rep int, m *ExperimentMetrics) {
		if hasSubExperiments {
			b.WriteString(fmt.Sprintf("%s,%s,%d,", csvEscape(report.Name), csvEscape(subName), rep))
		} else {
			b.WriteString(fmt.Sprintf("%s,%d,", csvEscape(report.Name), rep))
		}

		pollIntervalMs := 0.0
		timeoutS := 0.0
		failProb := 0.0
		taskCount := 0
		subConcurrency := 0
		if ec != nil {
			pollIntervalMs = float64(ec.PollInterval.Nanoseconds()) / 1e6
			timeoutS = ec.Timeout.Seconds()
			failProb = ec.FailureProbability
			taskCount = ec.TaskCount
			subConcurrency = ec.SubmissionConcurrency
		}

		b.WriteString(fmt.Sprintf("%d,%d,%.1f,%.1f,%.2f,%d,",
			taskCount, subConcurrency, pollIntervalMs, timeoutS, failProb, workerCount))

		b.WriteString(fmt.Sprintf("%d,%d,%d,%d,%d,%d,%d,%d,",
			m.SubmissionAttempts, m.AcceptedSubmissions, m.SubmissionErrors,
			m.CompletedTasks, m.FailedTasks, m.TimedOutTasks,
			m.PendingTasks, m.ProcessingTasks))

		b.WriteString(fmt.Sprintf("%.3f,%.3f,%.2f,%.2f,",
			m.TotalElapsed.Seconds(), m.SubmissionDuration.Seconds(),
			m.AcceptedSubmissionRate(), m.CompletedThroughput()))

		b.WriteString(fmt.Sprintf("%.1f,%.1f,%.1f,%.1f,%d,%d\n",
			float64(m.AvgLatency().Nanoseconds())/1e6,
			float64(m.P50Latency().Nanoseconds())/1e6,
			float64(m.P95Latency().Nanoseconds())/1e6,
			float64(m.P99Latency().Nanoseconds())/1e6,
			m.PollingErrors,
			len(m.Latencies)))
	}

	// Main repetitions.
	if !hasSubExperiments {
		for i := range report.Metrics {
			writeCSVRow("", i+1, &report.Metrics[i])
		}
	}

	// Sub-experiments.
	for name, metrics := range report.SubExperiments {
		for i := range metrics {
			writeCSVRow(name, i+1, &metrics[i])
		}
	}

	return b.String()
}

// extractExperimentConfig returns the embedded ExperimentConfig from the
// report's Config, or nil if the type is not recognised.
func extractExperimentConfig(cfg interface{}) *ExperimentConfig {
	switch c := cfg.(type) {
	case *BaselineConfig:
		return &c.ExperimentConfig
	case *BurstConfig:
		return &c.ExperimentConfig
	case *WorkerScalingConfig:
		return &c.ExperimentConfig
	case *WorkloadProfileConfig:
		return &c.ExperimentConfig
	case *SustainedRateConfig:
		return &c.ExperimentConfig
	case *ExperimentConfig:
		return c
	default:
		return nil
	}
}

// csvEscape wraps a string in double quotes if it contains commas or quotes.
func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}

// ---------------------------------------------------------------------------
// HTML Report
// ---------------------------------------------------------------------------

// GenerateHTML generates a self-contained HTML report for a single experiment.
// The file is fully offline-capable — all CSS is inline, charts are SVG, and
// there are no external dependencies.
func GenerateHTML(report ExperimentReport) string {
	var b strings.Builder

	elapsed := report.EndTime.Sub(report.StartTime)
	allMetrics := collectAllMetrics(report)

	b.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Experiment Report: `)
	b.WriteString(escapeHTML(report.Name))
	b.WriteString(`</title>
<style>
`)
	b.WriteString(htmlCSS())
	b.WriteString(`</style>
</head>
<body>
<div class="container">
`)

	// Navigation.
	b.WriteString(`<nav class="nav">
  <a href="#summary">Summary</a>
  <a href="#results">Results</a>
  <a href="#latency">Latency</a>
  <a href="#throughput">Throughput</a>
  <a href="#accounting">Accounting</a>
`)
	if len(report.Warnings) > 0 {
		b.WriteString(`  <a href="#warnings">Warnings</a>` + "\n")
	}
	b.WriteString("</nav>\n\n")

	// Header.
	b.WriteString(fmt.Sprintf(`<header class="header" id="summary">
  <h1>%s</h1>
  <div class="meta">
`, escapeHTML(report.Name)))
	if report.Description != "" {
		b.WriteString(fmt.Sprintf("    <p class=\"desc\">%s</p>\n", escapeHTML(report.Description)))
	}
	b.WriteString(fmt.Sprintf("    <span class=\"tag\">%s</span>\n", report.StartTime.Format("2006-01-02 15:04:05")))
	b.WriteString(fmt.Sprintf("    <span class=\"tag\">Duration: %s</span>\n", formatDuration(elapsed)))
	if report.BenchConfig != nil {
		b.WriteString(fmt.Sprintf("    <span class=\"tag\">API: %s</span>\n", escapeHTML(report.BenchConfig.APIURL)))
		b.WriteString(fmt.Sprintf("    <span class=\"tag\">Rev: %s</span>\n", escapeHTML(report.BenchConfig.GitRevision)))
		b.WriteString(fmt.Sprintf("    <span class=\"tag\">Workers: %d</span>\n", report.BenchConfig.WorkerCount))
	}
	b.WriteString("  </div>\n</header>\n\n")

	// Error banner.
	if report.Error != nil {
		b.WriteString(fmt.Sprintf(`<div class="error-banner">Error: %s</div>`+"\n", escapeHTML(report.Error.Error())))
	}

	// Config summary.
	b.WriteString("<section class=\"section\">\n<h2>Configuration</h2>\n")
	writeHTMLConfigTable(&b, report.Config)
	b.WriteString("</section>\n\n")

	// Results table.
	if len(allMetrics) > 0 {
		b.WriteString("<section class=\"section\" id=\"results\">\n<h2>Results</h2>\n")

		if len(report.SubExperiments) > 0 {
			for name, metrics := range report.SubExperiments {
				b.WriteString(fmt.Sprintf("<h3>%s</h3>\n", escapeHTML(name)))
				writeHTMLMetricsTable(&b, metrics)
			}
		} else {
			writeHTMLMetricsTable(&b, report.Metrics)
		}
		b.WriteString("</section>\n\n")
	}

	// Latency chart.
	if len(allMetrics) > 0 {
		b.WriteString("<section class=\"section\" id=\"latency\">\n<h2>Latency Percentiles</h2>\n")
		b.WriteString("<div class=\"chart-container\">\n")

		labels, p50vals, p95vals, p99vals := buildLatencyChartData(report)
		if len(labels) > 0 {
			// Build grouped bar chart with P50, P95, P99.
			b.WriteString(svgGroupedBarChart("Latency Comparison", labels, [][]float64{p50vals, p95vals, p99vals}, []string{"P50", "P95", "P99"}, []string{"#3b82f6", "#10b981", "#f59e0b"}, "ms", 600, 320))
		}

		b.WriteString("</div>\n")

		// Latency table.
		b.WriteString("<table>\n<thead><tr><th>Run</th><th>Observations</th><th>Avg</th><th>P50</th><th>P95</th><th>P99</th></tr></thead>\n<tbody>\n")
		if len(report.SubExperiments) > 0 {
			for name, metrics := range report.SubExperiments {
				for i, m := range metrics {
					b.WriteString(fmt.Sprintf("<tr><td>%s #%d</td><td>%d</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n",
						escapeHTML(name), i+1, len(m.Latencies),
						formatLatencyMs(m.AvgLatency()),
						formatLatencyMs(m.P50Latency()),
						formatLatencyMs(m.P95Latency()),
						formatLatencyMs(m.P99Latency()),
					))
				}
			}
		} else {
			for i, m := range report.Metrics {
				b.WriteString(fmt.Sprintf("<tr><td>Rep %d</td><td>%d</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n",
					i+1, len(m.Latencies),
					formatLatencyMs(m.AvgLatency()),
					formatLatencyMs(m.P50Latency()),
					formatLatencyMs(m.P95Latency()),
					formatLatencyMs(m.P99Latency()),
				))
			}
		}
		b.WriteString("</tbody>\n</table>\n")
		b.WriteString("</section>\n\n")
	}

	// Throughput chart.
	if len(allMetrics) > 0 {
		b.WriteString("<section class=\"section\" id=\"throughput\">\n<h2>Throughput</h2>\n")
		b.WriteString("<div class=\"chart-container\">\n")

		labels, throughputs := buildThroughputChartData(report)
		if len(labels) > 0 {
			b.WriteString(svgBarChart("Completed Throughput", labels, throughputs, "tasks/s", 600, 300))
		}

		b.WriteString("</div>\n")

		// Throughput table.
		b.WriteString("<table>\n<thead><tr><th>Run</th><th>Submission Rate</th><th>Completed Throughput</th></tr></thead>\n<tbody>\n")
		if len(report.SubExperiments) > 0 {
			for name, metrics := range report.SubExperiments {
				for i, m := range metrics {
					b.WriteString(fmt.Sprintf("<tr><td>%s #%d</td><td>%s</td><td>%s</td></tr>\n",
						escapeHTML(name), i+1,
						formatRate(m.AcceptedSubmissionRate()),
						formatRate(m.CompletedThroughput()),
					))
				}
			}
		} else {
			for i, m := range report.Metrics {
				b.WriteString(fmt.Sprintf("<tr><td>Rep %d</td><td>%s</td><td>%s</td></tr>\n",
					i+1,
					formatRate(m.AcceptedSubmissionRate()),
					formatRate(m.CompletedThroughput()),
				))
			}
		}
		b.WriteString("</tbody>\n</table>\n")
		b.WriteString("</section>\n\n")
	}

	// Task accounting.
	if len(allMetrics) > 0 {
		b.WriteString("<section class=\"section\" id=\"accounting\">\n<h2>Task Accounting</h2>\n")
		for i, m := range allMetrics {
			a := m.Accounting()
			valid := m.AccountingValid()
			statusClass := "status-ok"
			statusLabel := "BALANCED"
			if !valid {
				statusClass = "status-fail"
				statusLabel = "UNBALANCED"
			}
			if len(allMetrics) > 1 {
				b.WriteString(fmt.Sprintf("<h3>Run %d</h3>\n", i+1))
			}
			b.WriteString(fmt.Sprintf("<p class=\"%s\">%s</p>\n", statusClass, statusLabel))
			b.WriteString("<table>\n<thead><tr><th>Submitted</th><th>Accepted</th><th>Rejected</th><th>Completed</th><th>Failed</th><th>Timed Out</th><th>Pending</th><th>Processing</th><th>Unknown</th></tr></thead>\n<tbody>\n")
			b.WriteString(fmt.Sprintf("<tr><td>%d</td><td>%d</td><td>%d</td><td>%d</td><td>%d</td><td>%d</td><td>%d</td><td>%d</td><td>%d</td></tr>\n",
				a.Submitted, a.Accepted, a.Rejected, a.Completed, a.Failed, a.TimedOut, a.Pending, a.Processing, a.Unknown))
			b.WriteString("</tbody>\n</table>\n")
		}
		b.WriteString("</section>\n\n")
	}

	// Warnings.
	if len(report.Warnings) > 0 {
		b.WriteString("<section class=\"section\" id=\"warnings\">\n<h2>Warnings</h2>\n<ul class=\"warnings\">\n")
		for _, w := range report.Warnings {
			b.WriteString(fmt.Sprintf("  <li>%s</li>\n", escapeHTML(w)))
		}
		b.WriteString("</ul>\n</section>\n\n")
	}

	// Interpretation notes.
	b.WriteString("<section class=\"section\">\n<h2>Interpretation Notes</h2>\n<ul class=\"notes\">\n")
	b.WriteString("  <li>Latency measurements include polling interval and HTTP overhead</li>\n")
	b.WriteString("  <li>Handler simulates execution time; does not represent real workload</li>\n")
	totalTimedOut := 0
	for _, m := range allMetrics {
		totalTimedOut += m.TimedOutTasks
	}
	if totalTimedOut > 0 {
		b.WriteString(fmt.Sprintf("  <li>%d tasks timed out — latency statistics represent only completed tasks</li>\n", totalTimedOut))
	}
	b.WriteString("</ul>\n</section>\n\n")

	// Footer.
	b.WriteString(fmt.Sprintf("<footer>Generated %s</footer>\n", time.Now().Format("2006-01-02 15:04:05 MST")))

	b.WriteString("</div>\n</body>\n</html>\n")
	return b.String()
}

// svgGroupedBarChart generates an inline SVG grouped bar chart with multiple
// series displayed side-by-side for each label.
func svgGroupedBarChart(title string, labels []string, series [][]float64, seriesNames []string, colors []string, unit string, width, height int) string {
	if len(labels) == 0 || len(series) == 0 {
		return ""
	}

	numGroups := len(labels)
	numSeries := len(series)

	var b strings.Builder

	padLeft := 70
	padRight := 20
	padTop := 40
	padBottom := 80
	chartW := width - padLeft - padRight
	chartH := height - padTop - padBottom

	// Find global max.
	maxVal := 0.0
	for _, s := range series {
		for _, v := range s {
			if v > maxVal {
				maxVal = v
			}
		}
	}
	if maxVal <= 0 {
		maxVal = 1
	}
	niceMax := niceAxisMax(maxVal)

	groupWidth := chartW / numGroups
	barWidth := (groupWidth - 8) / numSeries
	if barWidth < 4 {
		barWidth = 4
	}

	b.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" style="font-family:sans-serif;background:#fff;">`, width, height, width, height))
	b.WriteByte('\n')

	// Title.
	b.WriteString(fmt.Sprintf(`  <text x="%d" y="%d" text-anchor="middle" font-size="14" font-weight="bold" fill="#1e293b">%s</text>`, width/2, 22, escapeXML(title)))
	b.WriteByte('\n')

	// Y-axis gridlines.
	gridLines := 5
	for i := 0; i <= gridLines; i++ {
		yFrac := float64(i) / float64(gridLines)
		yVal := niceMax * (1.0 - yFrac)
		yPos := padTop + int(yFrac*float64(chartH))
		b.WriteString(fmt.Sprintf(`  <line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#e2e8f0" stroke-width="1"/>`, padLeft, yPos, padLeft+chartW, yPos))
		b.WriteByte('\n')
		b.WriteString(fmt.Sprintf(`  <text x="%d" y="%d" text-anchor="end" font-size="10" fill="#64748b">%.1f</text>`, padLeft-6, yPos+4, yVal))
		b.WriteByte('\n')
	}

	// Bars.
	for g := 0; g < numGroups; g++ {
		groupX := padLeft + g*groupWidth + 4
		for s := 0; s < numSeries; s++ {
			val := 0.0
			if g < len(series[s]) {
				val = series[s][g]
			}
			x := groupX + s*barWidth
			barH := int(float64(chartH) * (val / niceMax))
			if barH < 1 && val > 0 {
				barH = 1
			}
			y := padTop + chartH - barH
			colour := colors[s%len(colors)]
			b.WriteString(fmt.Sprintf(`  <rect x="%d" y="%d" width="%d" height="%d" fill="%s" rx="2"/>`, x, y, barWidth-1, barH, colour))
			b.WriteByte('\n')
		}
		// Group label.
		labelX := groupX + (numSeries*barWidth)/2
		labelY := padTop + chartH + 16
		label := labels[g]
		if len(label) > 14 {
			label = label[:14] + "…"
		}
		b.WriteString(fmt.Sprintf(`  <text x="%d" y="%d" text-anchor="middle" font-size="10" fill="#475569" transform="rotate(-25,%d,%d)">%s</text>`, labelX, labelY, labelX, labelY, escapeXML(label)))
		b.WriteByte('\n')
	}

	// Axes.
	b.WriteString(fmt.Sprintf(`  <line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#94a3b8" stroke-width="1"/>`, padLeft, padTop+chartH, padLeft+chartW, padTop+chartH))
	b.WriteByte('\n')
	b.WriteString(fmt.Sprintf(`  <line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#94a3b8" stroke-width="1"/>`, padLeft, padTop, padLeft, padTop+chartH))
	b.WriteByte('\n')

	// Legend.
	legendY := height - 12
	legendX := padLeft
	for s := 0; s < numSeries; s++ {
		colour := colors[s%len(colors)]
		b.WriteString(fmt.Sprintf(`  <rect x="%d" y="%d" width="12" height="12" fill="%s" rx="2"/>`, legendX, legendY-10, colour))
		name := ""
		if s < len(seriesNames) {
			name = seriesNames[s]
		}
		b.WriteString(fmt.Sprintf(`  <text x="%d" y="%d" font-size="10" fill="#475569">%s</text>`, legendX+16, legendY, escapeXML(name)))
		legendX += 16 + len(name)*7 + 16
	}
	b.WriteByte('\n')

	// Unit label.
	b.WriteString(fmt.Sprintf(`  <text x="%d" y="%d" text-anchor="middle" font-size="10" fill="#64748b">%s</text>`, width/2, height-28, escapeXML(unit)))
	b.WriteByte('\n')

	b.WriteString("</svg>\n")
	return b.String()
}

// buildLatencyChartData prepares labels and per-percentile values for latency charts.
func buildLatencyChartData(report ExperimentReport) (labels []string, p50, p95, p99 []float64) {
	msOf := func(d time.Duration) float64 { return float64(d.Nanoseconds()) / 1e6 }

	if len(report.SubExperiments) > 0 {
		for name, metrics := range report.SubExperiments {
			for i, m := range metrics {
				label := name
				if len(metrics) > 1 {
					label = fmt.Sprintf("%s #%d", name, i+1)
				}
				labels = append(labels, label)
				p50 = append(p50, msOf(m.P50Latency()))
				p95 = append(p95, msOf(m.P95Latency()))
				p99 = append(p99, msOf(m.P99Latency()))
			}
		}
	} else {
		for i, m := range report.Metrics {
			labels = append(labels, fmt.Sprintf("Rep %d", i+1))
			p50 = append(p50, msOf(m.P50Latency()))
			p95 = append(p95, msOf(m.P95Latency()))
			p99 = append(p99, msOf(m.P99Latency()))
		}
	}
	return
}

// buildThroughputChartData prepares labels and throughput values for charts.
func buildThroughputChartData(report ExperimentReport) (labels []string, throughputs []float64) {
	if len(report.SubExperiments) > 0 {
		for name, metrics := range report.SubExperiments {
			for i, m := range metrics {
				label := name
				if len(metrics) > 1 {
					label = fmt.Sprintf("%s #%d", name, i+1)
				}
				labels = append(labels, label)
				throughputs = append(throughputs, m.CompletedThroughput())
			}
		}
	} else {
		for i, m := range report.Metrics {
			labels = append(labels, fmt.Sprintf("Rep %d", i+1))
			throughputs = append(throughputs, m.CompletedThroughput())
		}
	}
	return
}

// writeHTMLMetricsTable writes an HTML table of per-repetition metrics.
func writeHTMLMetricsTable(b *strings.Builder, metrics []ExperimentMetrics) {
	b.WriteString("<table>\n<thead><tr>")
	for _, h := range []string{"Rep", "Submitted", "Accepted", "Completed", "Failed", "Timed Out", "Pending", "Processing", "Elapsed", "Sub Rate", "Throughput"} {
		b.WriteString(fmt.Sprintf("<th>%s</th>", h))
	}
	b.WriteString("</tr></thead>\n<tbody>\n")
	for i, m := range metrics {
		b.WriteString(fmt.Sprintf("<tr><td>%d</td><td>%d</td><td>%d</td><td>%d</td><td>%d</td><td>%d</td><td>%d</td><td>%d</td><td>%s</td><td>%s</td><td>%s</td></tr>\n",
			i+1,
			m.SubmissionAttempts,
			m.AcceptedSubmissions,
			m.CompletedTasks,
			m.FailedTasks,
			m.TimedOutTasks,
			m.PendingTasks,
			m.ProcessingTasks,
			formatDuration(m.TotalElapsed),
			formatRate(m.AcceptedSubmissionRate()),
			formatRate(m.CompletedThroughput()),
		))
	}
	b.WriteString("</tbody>\n</table>\n")
}

// writeHTMLConfigTable writes an HTML table of experiment configuration parameters.
func writeHTMLConfigTable(b *strings.Builder, cfg interface{}) {
	b.WriteString("<table>\n<thead><tr><th>Parameter</th><th>Value</th></tr></thead>\n<tbody>\n")

	writeRow := func(param, value string) {
		b.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%s</td></tr>\n", escapeHTML(param), escapeHTML(value)))
	}

	writeBaseRows := func(ec *ExperimentConfig) {
		writeRow("Name", ec.Name)
		writeRow("Task Count", fmt.Sprintf("%d", ec.TaskCount))
		writeRow("Submission Concurrency", fmt.Sprintf("%d", ec.SubmissionConcurrency))
		writeRow("Polling Concurrency", fmt.Sprintf("%d", ec.PollingConcurrency))
		writeRow("Poll Interval", ec.PollInterval.String())
		writeRow("Timeout", ec.Timeout.String())
		writeRow("Failure Probability", fmt.Sprintf("%.2f", ec.FailureProbability))
		writeRow("Repetitions", fmt.Sprintf("%d", ec.Repetitions))
	}

	switch c := cfg.(type) {
	case *BaselineConfig:
		writeBaseRows(&c.ExperimentConfig)
		writeRow("Min Duration (ms)", fmt.Sprintf("%d", c.MinDurationMs))
		writeRow("Max Duration (ms)", fmt.Sprintf("%d", c.MaxDurationMs))
	case *BurstConfig:
		writeBaseRows(&c.ExperimentConfig)
		writeRow("Batch Sizes", fmt.Sprintf("%v", c.BatchSizes))
		writeRow("Min Duration (ms)", fmt.Sprintf("%d", c.MinDurationMs))
		writeRow("Max Duration (ms)", fmt.Sprintf("%d", c.MaxDurationMs))
	case *WorkerScalingConfig:
		writeBaseRows(&c.ExperimentConfig)
		writeRow("Reported Worker Count", fmt.Sprintf("%d", c.ReportedWorkerCount))
		writeRow("Min Duration (ms)", fmt.Sprintf("%d", c.MinDurationMs))
		writeRow("Max Duration (ms)", fmt.Sprintf("%d", c.MaxDurationMs))
	case *WorkloadProfileConfig:
		writeBaseRows(&c.ExperimentConfig)
		for _, p := range c.Profiles {
			writeRow("Profile: "+p.Name, fmt.Sprintf("email=%.0f%% report=%.0f%% image=%.0f%%", p.EmailPct*100, p.ReportPct*100, p.ImagePct*100))
		}
	case *SustainedRateConfig:
		writeBaseRows(&c.ExperimentConfig)
		writeRow("Rates (tasks/s)", fmt.Sprintf("%v", c.Rates))
		writeRow("Rate Duration", c.Duration.String())
		writeRow("Min Duration (ms)", fmt.Sprintf("%d", c.MinDurationMs))
		writeRow("Max Duration (ms)", fmt.Sprintf("%d", c.MaxDurationMs))
	case *ExperimentConfig:
		writeBaseRows(c)
	default:
		writeRow("Config", fmt.Sprintf("%v", cfg))
	}

	b.WriteString("</tbody>\n</table>\n")
}

// escapeHTML escapes special characters for safe HTML embedding.
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// htmlCSS returns the inline CSS for the HTML report.
func htmlCSS() string {
	return `* { margin: 0; padding: 0; box-sizing: border-box; }
body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
  line-height: 1.6;
  color: #1e293b;
  background: #f8fafc;
}
.container { max-width: 1100px; margin: 0 auto; padding: 24px; }
.nav {
  display: flex; gap: 16px; padding: 12px 0; margin-bottom: 24px;
  border-bottom: 2px solid #e2e8f0; flex-wrap: wrap;
}
.nav a {
  color: #3b82f6; text-decoration: none; font-weight: 500; font-size: 14px;
  padding: 4px 8px; border-radius: 4px;
}
.nav a:hover { background: #eff6ff; }
.header {
  background: linear-gradient(135deg, #1e3a5f 0%, #0f766e 100%);
  color: #fff; padding: 32px; border-radius: 12px; margin-bottom: 24px;
}
.header h1 { font-size: 28px; margin-bottom: 12px; }
.meta { display: flex; gap: 12px; flex-wrap: wrap; align-items: center; }
.tag {
  background: rgba(255,255,255,0.15); padding: 4px 12px; border-radius: 20px;
  font-size: 13px;
}
.desc { width: 100%; margin-bottom: 8px; opacity: 0.9; }
.section {
  background: #fff; border: 1px solid #e2e8f0; border-radius: 8px;
  padding: 24px; margin-bottom: 20px;
}
.section h2 {
  font-size: 20px; color: #0f766e; margin-bottom: 16px;
  padding-bottom: 8px; border-bottom: 2px solid #e2e8f0;
}
.section h3 { font-size: 16px; color: #1e3a5f; margin: 16px 0 8px 0; }
table {
  width: 100%; border-collapse: collapse; margin-bottom: 16px;
  font-size: 13px;
}
thead th {
  background: #f1f5f9; color: #475569; padding: 10px 12px;
  text-align: left; font-weight: 600; border-bottom: 2px solid #e2e8f0;
  white-space: nowrap;
}
tbody td {
  padding: 8px 12px; border-bottom: 1px solid #f1f5f9;
}
tbody tr:hover { background: #f8fafc; }
.chart-container { overflow-x: auto; margin: 16px 0; text-align: center; }
.error-banner {
  background: #fef2f2; border: 1px solid #fca5a5; color: #991b1b;
  padding: 12px 16px; border-radius: 8px; margin-bottom: 20px;
  font-weight: 500;
}
.status-ok {
  display: inline-block; background: #ecfdf5; color: #065f46;
  padding: 4px 12px; border-radius: 4px; font-weight: 600; font-size: 13px;
}
.status-fail {
  display: inline-block; background: #fef2f2; color: #991b1b;
  padding: 4px 12px; border-radius: 4px; font-weight: 600; font-size: 13px;
}
.warnings { list-style: none; }
.warnings li {
  padding: 8px 12px; background: #fffbeb; border-left: 3px solid #f59e0b;
  margin-bottom: 6px; border-radius: 0 4px 4px 0; font-size: 13px;
}
.notes { list-style: none; }
.notes li {
  padding: 6px 12px; color: #64748b; font-size: 13px;
  border-left: 3px solid #94a3b8; margin-bottom: 4px;
}
footer {
  text-align: center; padding: 24px 0; color: #94a3b8; font-size: 12px;
}
.cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 16px; margin-bottom: 20px; }
.card {
  background: #fff; border: 1px solid #e2e8f0; border-radius: 8px;
  padding: 20px; text-align: center;
}
.card .value { font-size: 28px; font-weight: 700; color: #0f766e; }
.card .label { font-size: 12px; color: #64748b; margin-top: 4px; text-transform: uppercase; letter-spacing: 0.5px; }
.status-pass { color: #065f46; }
.status-warn { color: #92400e; }
.status-error { color: #991b1b; }
@media (max-width: 768px) {
  .container { padding: 12px; }
  .header { padding: 20px; }
  .header h1 { font-size: 22px; }
  table { font-size: 11px; }
  thead th, tbody td { padding: 6px 8px; }
}
`
}

// ---------------------------------------------------------------------------
// Summary reports (multi-experiment)
// ---------------------------------------------------------------------------

// GenerateSummaryMarkdown generates an overall Markdown summary of all
// experiments in a benchmark run.
func GenerateSummaryMarkdown(reports []ExperimentReport, cfg *BenchmarkConfig, runDir string, startTime, endTime time.Time) string {
	var b strings.Builder

	b.WriteString("# Benchmark Run Summary\n\n")

	// Run metadata.
	b.WriteString("## Run Metadata\n\n")
	b.WriteString(fmt.Sprintf("| Field | Value |\n"))
	b.WriteString(fmt.Sprintf("| --- | --- |\n"))
	b.WriteString(fmt.Sprintf("| Date | %s |\n", startTime.Format("2006-01-02 15:04:05 MST")))
	b.WriteString(fmt.Sprintf("| Total Elapsed | %s |\n", formatDuration(endTime.Sub(startTime))))
	if cfg != nil {
		b.WriteString(fmt.Sprintf("| Suite | %s |\n", cfg.Suite))
		b.WriteString(fmt.Sprintf("| Git Revision | `%s` |\n", cfg.GitRevision))
		b.WriteString(fmt.Sprintf("| Worker Count | %d |\n", cfg.WorkerCount))
		b.WriteString(fmt.Sprintf("| API URL | `%s` |\n", cfg.APIURL))
	}
	b.WriteString(fmt.Sprintf("| Results Directory | `%s` |\n", runDir))
	b.WriteByte('\n')

	// Experiments table.
	b.WriteString("## Experiments\n\n")
	b.WriteString("| # | Name | Status | Tasks | Completed | Throughput | P50 | P95 | Report |\n")
	b.WriteString("| ---: | --- | :---: | ---: | ---: | ---: | ---: | ---: | --- |\n")

	failedCount := 0
	for i, r := range reports {
		status := "✓"
		tasks := 0
		completed := 0
		throughput := 0.0
		p50 := time.Duration(0)
		p95 := time.Duration(0)

		if r.Error != nil {
			status = "✗"
			failedCount++
		} else if len(r.Warnings) > 0 {
			status = "⚠"
		}

		allM := collectAllMetrics(r)
		for _, m := range allM {
			tasks += m.SubmissionAttempts
			completed += m.CompletedTasks
		}

		// Use first metrics for representative rates.
		if len(allM) > 0 {
			throughput = allM[0].CompletedThroughput()
			p50 = allM[0].P50Latency()
			p95 = allM[0].P95Latency()
		}

		reportLink := fmt.Sprintf("[report](%s/%s.md)", runDir, r.Name)
		b.WriteString(fmt.Sprintf("| %d | %s | %s | %d | %d | %s | %s | %s | %s |\n",
			i+1, r.Name, status, tasks, completed,
			formatRate(throughput),
			formatLatencyMs(p50),
			formatLatencyMs(p95),
			reportLink,
		))
	}
	b.WriteByte('\n')

	// Highlight failures.
	if failedCount > 0 {
		b.WriteString("### Failed Experiments\n\n")
		for _, r := range reports {
			if r.Error != nil {
				b.WriteString(fmt.Sprintf("- **%s**: %s\n", r.Name, r.Error.Error()))
			}
		}
		b.WriteByte('\n')
	}

	b.WriteString(fmt.Sprintf("---\n*Total elapsed: %s*\n", formatDuration(endTime.Sub(startTime))))

	return b.String()
}

// ---------------------------------------------------------------------------
// Summary CSV
// ---------------------------------------------------------------------------

// GenerateSummaryCSV generates a CSV with one row per experiment containing
// key metrics.
func GenerateSummaryCSV(reports []ExperimentReport) string {
	var b strings.Builder

	b.WriteString("experiment,status,total_tasks,accepted,completed,failed,timed_out,throughput,submission_rate,avg_latency_ms,p50_latency_ms,p95_latency_ms,p99_latency_ms,elapsed_s,error\n")

	for _, r := range reports {
		status := "pass"
		errMsg := ""
		if r.Error != nil {
			status = "fail"
			errMsg = r.Error.Error()
		} else if len(r.Warnings) > 0 {
			status = "warn"
		}

		totalTasks := 0
		totalAccepted := 0
		totalCompleted := 0
		totalFailed := 0
		totalTimedOut := 0
		throughput := 0.0
		subRate := 0.0
		avgLat := time.Duration(0)
		p50 := time.Duration(0)
		p95 := time.Duration(0)
		p99 := time.Duration(0)

		allM := collectAllMetrics(r)
		for _, m := range allM {
			totalTasks += m.SubmissionAttempts
			totalAccepted += m.AcceptedSubmissions
			totalCompleted += m.CompletedTasks
			totalFailed += m.FailedTasks
			totalTimedOut += m.TimedOutTasks
		}
		if len(allM) > 0 {
			throughput = allM[0].CompletedThroughput()
			subRate = allM[0].AcceptedSubmissionRate()
			avgLat = allM[0].AvgLatency()
			p50 = allM[0].P50Latency()
			p95 = allM[0].P95Latency()
			p99 = allM[0].P99Latency()
		}

		elapsed := r.EndTime.Sub(r.StartTime).Seconds()

		b.WriteString(fmt.Sprintf("%s,%s,%d,%d,%d,%d,%d,%.2f,%.2f,%.1f,%.1f,%.1f,%.1f,%.3f,%s\n",
			csvEscape(r.Name),
			status,
			totalTasks,
			totalAccepted,
			totalCompleted,
			totalFailed,
			totalTimedOut,
			throughput,
			subRate,
			float64(avgLat.Nanoseconds())/1e6,
			float64(p50.Nanoseconds())/1e6,
			float64(p95.Nanoseconds())/1e6,
			float64(p99.Nanoseconds())/1e6,
			elapsed,
			csvEscape(errMsg),
		))
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// Summary HTML
// ---------------------------------------------------------------------------

// GenerateSummaryHTML generates a self-contained HTML dashboard summarising all
// experiments in a benchmark run. The file is fully offline-capable.
func GenerateSummaryHTML(reports []ExperimentReport, cfg *BenchmarkConfig, runDir string, startTime, endTime time.Time) string {
	var b strings.Builder

	elapsed := endTime.Sub(startTime)
	totalExperiments := len(reports)
	passedCount := 0
	failedCount := 0
	warnCount := 0
	totalCompleted := 0
	totalTasks := 0

	for _, r := range reports {
		if r.Error != nil {
			failedCount++
		} else if len(r.Warnings) > 0 {
			warnCount++
		} else {
			passedCount++
		}
		for _, m := range collectAllMetrics(r) {
			totalTasks += m.SubmissionAttempts
			totalCompleted += m.CompletedTasks
		}
	}

	b.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Benchmark Summary</title>
<style>
`)
	b.WriteString(htmlCSS())
	b.WriteString(`</style>
</head>
<body>
<div class="container">
`)

	// Navigation.
	b.WriteString(`<nav class="nav">
  <a href="#overview">Overview</a>
  <a href="#results">Results</a>
  <a href="#charts">Charts</a>
</nav>
`)

	// Header.
	b.WriteString(`<header class="header" id="overview">
  <h1>Benchmark Run Summary</h1>
  <div class="meta">
`)
	b.WriteString(fmt.Sprintf("    <span class=\"tag\">%s</span>\n", startTime.Format("2006-01-02 15:04:05")))
	b.WriteString(fmt.Sprintf("    <span class=\"tag\">Duration: %s</span>\n", formatDuration(elapsed)))
	if cfg != nil {
		b.WriteString(fmt.Sprintf("    <span class=\"tag\">Suite: %s</span>\n", escapeHTML(cfg.Suite)))
		b.WriteString(fmt.Sprintf("    <span class=\"tag\">Rev: %s</span>\n", escapeHTML(cfg.GitRevision)))
		b.WriteString(fmt.Sprintf("    <span class=\"tag\">Workers: %d</span>\n", cfg.WorkerCount))
	}
	b.WriteString("  </div>\n</header>\n\n")

	// Overview cards.
	b.WriteString("<div class=\"cards\">\n")
	writeHTMLCard(&b, fmt.Sprintf("%d", totalExperiments), "Experiments")
	writeHTMLCard(&b, fmt.Sprintf("%d", passedCount), "Passed")
	if failedCount > 0 {
		writeHTMLCardColored(&b, fmt.Sprintf("%d", failedCount), "Failed", "#991b1b")
	} else {
		writeHTMLCard(&b, "0", "Failed")
	}
	if warnCount > 0 {
		writeHTMLCardColored(&b, fmt.Sprintf("%d", warnCount), "Warnings", "#92400e")
	}
	writeHTMLCard(&b, fmt.Sprintf("%d", totalTasks), "Total Tasks")
	writeHTMLCard(&b, fmt.Sprintf("%d", totalCompleted), "Completed")
	writeHTMLCard(&b, formatDuration(elapsed), "Total Time")
	b.WriteString("</div>\n\n")

	// Run metadata table.
	if cfg != nil {
		b.WriteString("<section class=\"section\">\n<h2>Run Configuration</h2>\n")
		b.WriteString("<table>\n<thead><tr><th>Parameter</th><th>Value</th></tr></thead>\n<tbody>\n")
		b.WriteString(fmt.Sprintf("<tr><td>API URL</td><td>%s</td></tr>\n", escapeHTML(cfg.APIURL)))
		b.WriteString(fmt.Sprintf("<tr><td>Suite</td><td>%s</td></tr>\n", escapeHTML(cfg.Suite)))
		b.WriteString(fmt.Sprintf("<tr><td>Git Revision</td><td>%s</td></tr>\n", escapeHTML(cfg.GitRevision)))
		b.WriteString(fmt.Sprintf("<tr><td>Worker Count</td><td>%d</td></tr>\n", cfg.WorkerCount))
		b.WriteString(fmt.Sprintf("<tr><td>Results Directory</td><td>%s</td></tr>\n", escapeHTML(runDir)))
		b.WriteString("</tbody>\n</table>\n</section>\n\n")
	}

	// Experiment results table.
	b.WriteString("<section class=\"section\" id=\"results\">\n<h2>Experiment Results</h2>\n")
	b.WriteString("<table>\n<thead><tr>")
	for _, h := range []string{"#", "Name", "Status", "Tasks", "Completed", "Throughput", "P50", "P95", "Report"} {
		b.WriteString(fmt.Sprintf("<th>%s</th>", h))
	}
	b.WriteString("</tr></thead>\n<tbody>\n")

	for i, r := range reports {
		statusHTML := `<span class="status-pass">✓</span>`
		if r.Error != nil {
			statusHTML = `<span class="status-error">✗</span>`
		} else if len(r.Warnings) > 0 {
			statusHTML = `<span class="status-warn">⚠</span>`
		}

		tasks := 0
		completed := 0
		throughput := 0.0
		p50 := time.Duration(0)
		p95 := time.Duration(0)

		allM := collectAllMetrics(r)
		for _, m := range allM {
			tasks += m.SubmissionAttempts
			completed += m.CompletedTasks
		}
		if len(allM) > 0 {
			throughput = allM[0].CompletedThroughput()
			p50 = allM[0].P50Latency()
			p95 = allM[0].P95Latency()
		}

		reportLink := fmt.Sprintf(`<a href="%s/%s.html">view</a>`, escapeHTML(runDir), escapeHTML(r.Name))
		b.WriteString(fmt.Sprintf("<tr><td>%d</td><td>%s</td><td>%s</td><td>%d</td><td>%d</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n",
			i+1,
			escapeHTML(r.Name),
			statusHTML,
			tasks,
			completed,
			formatRate(throughput),
			formatLatencyMs(p50),
			formatLatencyMs(p95),
			reportLink,
		))
	}
	b.WriteString("</tbody>\n</table>\n</section>\n\n")

	// Throughput chart across experiments.
	if len(reports) > 0 {
		b.WriteString("<section class=\"section\" id=\"charts\">\n<h2>Throughput Comparison</h2>\n")
		b.WriteString("<div class=\"chart-container\">\n")

		var labels []string
		var throughputs []float64
		for _, r := range reports {
			labels = append(labels, r.Name)
			allM := collectAllMetrics(r)
			t := 0.0
			if len(allM) > 0 {
				t = allM[0].CompletedThroughput()
			}
			throughputs = append(throughputs, t)
		}
		b.WriteString(svgBarChart("Completed Throughput by Experiment", labels, throughputs, "tasks/s", 700, 320))
		b.WriteString("</div>\n")

		// P50 latency comparison chart.
		b.WriteString("<h2 style=\"margin-top:24px\">P50 Latency Comparison</h2>\n")
		b.WriteString("<div class=\"chart-container\">\n")
		var p50vals []float64
		for _, r := range reports {
			allM := collectAllMetrics(r)
			v := 0.0
			if len(allM) > 0 {
				v = float64(allM[0].P50Latency().Nanoseconds()) / 1e6
			}
			p50vals = append(p50vals, v)
		}
		b.WriteString(svgBarChart("P50 Latency by Experiment", labels, p50vals, "ms", 700, 320))
		b.WriteString("</div>\n")

		b.WriteString("</section>\n\n")
	}

	// Failed experiments.
	if failedCount > 0 {
		b.WriteString("<section class=\"section\">\n<h2>Failed Experiments</h2>\n<ul class=\"warnings\">\n")
		for _, r := range reports {
			if r.Error != nil {
				b.WriteString(fmt.Sprintf("  <li><strong>%s</strong>: %s</li>\n", escapeHTML(r.Name), escapeHTML(r.Error.Error())))
			}
		}
		b.WriteString("</ul>\n</section>\n\n")
	}

	// Footer.
	b.WriteString(fmt.Sprintf("<footer>Generated %s</footer>\n", time.Now().Format("2006-01-02 15:04:05 MST")))

	b.WriteString("</div>\n</body>\n</html>\n")
	return b.String()
}

// writeHTMLCard writes a metric card element.
func writeHTMLCard(b *strings.Builder, value, label string) {
	b.WriteString(fmt.Sprintf("<div class=\"card\"><div class=\"value\">%s</div><div class=\"label\">%s</div></div>\n",
		escapeHTML(value), escapeHTML(label)))
}

// writeHTMLCardColored writes a metric card with a custom value colour.
func writeHTMLCardColored(b *strings.Builder, value, label, color string) {
	b.WriteString(fmt.Sprintf("<div class=\"card\"><div class=\"value\" style=\"color:%s\">%s</div><div class=\"label\">%s</div></div>\n",
		color, escapeHTML(value), escapeHTML(label)))
}
