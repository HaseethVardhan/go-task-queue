package main

import (
	"flag"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// BenchmarkConfig is the top-level configuration for the entire benchmark run.
type BenchmarkConfig struct {
	APIURL      string
	WorkerCount int    // metadata: the worker count the server was started with
	Suite       string // "quick", "comprehensive", or "custom"
	Experiments []string
	ResultsDir  string
	Verbose     bool
	GitRevision string

	// Overrides applied to every experiment when set via flags.
	taskCountOverride       int
	concurrencyOverride     int
	pollConcurrencyOverride int
	pollIntervalOverride    time.Duration
	timeoutOverride         time.Duration
	repetitionsOverride     int
	failureProbability      float64
}

// ExperimentConfig is the base configuration shared by all experiments.
type ExperimentConfig struct {
	Name                  string
	TaskCount             int
	SubmissionConcurrency int
	PollingConcurrency    int
	PollInterval          time.Duration
	Timeout               time.Duration
	FailureProbability    float64
	Repetitions           int
}

// BaselineConfig wraps ExperimentConfig for the baseline throughput experiment.
type BaselineConfig struct {
	ExperimentConfig
	MinDurationMs int
	MaxDurationMs int
}

// WorkerScalingConfig holds configuration for the worker-scaling experiment.
type WorkerScalingConfig struct {
	ExperimentConfig
	ReportedWorkerCount int // metadata only
	MinDurationMs       int
	MaxDurationMs       int
}

// BurstConfig holds configuration for the burst-submission experiment.
type BurstConfig struct {
	ExperimentConfig
	BatchSizes    []int // e.g., [100, 300, 600]
	MinDurationMs int
	MaxDurationMs int
}

// WorkloadProfile describes a single mix of task types.
type WorkloadProfile struct {
	Name          string
	EmailPct      float64 // percentage 0-1
	ReportPct     float64
	ImagePct      float64
	MinDurationMs int
	MaxDurationMs int
}

// WorkloadProfileConfig holds configuration for the workload-profile experiment.
type WorkloadProfileConfig struct {
	ExperimentConfig
	Profiles []WorkloadProfile
}

// SustainedRateConfig holds configuration for the sustained-rate experiment.
type SustainedRateConfig struct {
	ExperimentConfig
	Rates         []float64 // tasks per second
	Duration      time.Duration
	MinDurationMs int
	MaxDurationMs int
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

// Validate checks that the BenchmarkConfig fields are sensible.
func (c *BenchmarkConfig) Validate() error {
	if c.APIURL == "" {
		return fmt.Errorf("config: APIURL must not be empty")
	}
	if c.WorkerCount < 1 {
		return fmt.Errorf("config: WorkerCount must be >= 1, got %d", c.WorkerCount)
	}
	switch c.Suite {
	case "quick", "comprehensive", "custom":
		// ok
	default:
		return fmt.Errorf("config: Suite must be \"quick\", \"comprehensive\", or \"custom\", got %q", c.Suite)
	}
	if c.ResultsDir == "" {
		return fmt.Errorf("config: ResultsDir must not be empty")
	}
	return nil
}

// Validate checks that the ExperimentConfig fields are sensible.
func (c *ExperimentConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("experiment config: Name must not be empty")
	}
	if c.TaskCount < 1 {
		return fmt.Errorf("experiment %q: TaskCount must be >= 1, got %d", c.Name, c.TaskCount)
	}
	if c.SubmissionConcurrency < 1 {
		return fmt.Errorf("experiment %q: SubmissionConcurrency must be >= 1, got %d", c.Name, c.SubmissionConcurrency)
	}
	if c.PollingConcurrency < 1 {
		return fmt.Errorf("experiment %q: PollingConcurrency must be >= 1, got %d", c.Name, c.PollingConcurrency)
	}
	if c.PollInterval <= 0 {
		return fmt.Errorf("experiment %q: PollInterval must be > 0, got %s", c.Name, c.PollInterval)
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("experiment %q: Timeout must be > 0, got %s", c.Name, c.Timeout)
	}
	if c.FailureProbability < 0 || c.FailureProbability > 1 {
		return fmt.Errorf("experiment %q: FailureProbability must be in [0,1], got %f", c.Name, c.FailureProbability)
	}
	if c.Repetitions < 1 {
		return fmt.Errorf("experiment %q: Repetitions must be >= 1, got %d", c.Name, c.Repetitions)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Default BenchmarkConfig factories
// ---------------------------------------------------------------------------

// DefaultQuickConfig returns a BenchmarkConfig suitable for a fast smoke-test.
func DefaultQuickConfig() *BenchmarkConfig {
	return &BenchmarkConfig{
		APIURL:             "http://localhost:8080",
		WorkerCount:        3,
		Suite:              "quick",
		ResultsDir:         "results",
		Verbose:            false,
		GitRevision:        detectGitRevision(),
		failureProbability: 0.1,
	}
}

// DefaultComprehensiveConfig returns a BenchmarkConfig for a thorough run.
func DefaultComprehensiveConfig() *BenchmarkConfig {
	return &BenchmarkConfig{
		APIURL:             "http://localhost:8080",
		WorkerCount:        3,
		Suite:              "comprehensive",
		ResultsDir:         "results",
		Verbose:            false,
		GitRevision:        detectGitRevision(),
		failureProbability: 0.1,
	}
}

// ---------------------------------------------------------------------------
// Default experiment config factories
// ---------------------------------------------------------------------------

func quickExperimentBase(name string) ExperimentConfig {
	return ExperimentConfig{
		Name:                  name,
		TaskCount:             20,
		SubmissionConcurrency: 5,
		PollingConcurrency:    10,
		PollInterval:          500 * time.Millisecond,
		Timeout:               2 * time.Minute,
		FailureProbability:    0.1,
		Repetitions:           1,
	}
}

func comprehensiveExperimentBase(name string) ExperimentConfig {
	return ExperimentConfig{
		Name:                  name,
		TaskCount:             100,
		SubmissionConcurrency: 10,
		PollingConcurrency:    20,
		PollInterval:          300 * time.Millisecond,
		Timeout:               5 * time.Minute,
		FailureProbability:    0.1,
		Repetitions:           3,
	}
}

func experimentBase(name string, quick bool) ExperimentConfig {
	if quick {
		return quickExperimentBase(name)
	}
	return comprehensiveExperimentBase(name)
}

// DefaultBaselineConfig returns a BaselineConfig with sensible defaults.
func DefaultBaselineConfig(quick bool) *BaselineConfig {
	return &BaselineConfig{
		ExperimentConfig: experimentBase("baseline", quick),
		MinDurationMs:    100,
		MaxDurationMs:    500,
	}
}

// DefaultBurstConfig returns a BurstConfig with sensible defaults.
func DefaultBurstConfig(quick bool) *BurstConfig {
	base := experimentBase("burst", quick)
	var batches []int
	if quick {
		batches = []int{20, 50}
	} else {
		batches = []int{100, 300, 600}
	}
	return &BurstConfig{
		ExperimentConfig: base,
		BatchSizes:       batches,
		MinDurationMs:    100,
		MaxDurationMs:    500,
	}
}

// DefaultWorkerScalingConfig returns a WorkerScalingConfig with sensible defaults.
func DefaultWorkerScalingConfig(quick bool) *WorkerScalingConfig {
	base := experimentBase("worker_scaling", quick)
	if quick {
		base.TaskCount = 30
	} else {
		base.TaskCount = 150
	}
	return &WorkerScalingConfig{
		ExperimentConfig:    base,
		ReportedWorkerCount: 3,
		MinDurationMs:       100,
		MaxDurationMs:       500,
	}
}

// DefaultWorkloadProfileConfig returns a WorkloadProfileConfig with sensible defaults.
func DefaultWorkloadProfileConfig(quick bool) *WorkloadProfileConfig {
	base := experimentBase("workload_profile", quick)
	if quick {
		base.TaskCount = 30
	} else {
		base.TaskCount = 120
	}
	return &WorkloadProfileConfig{
		ExperimentConfig: base,
		Profiles: []WorkloadProfile{
			{
				Name:          "email_heavy",
				EmailPct:      0.7,
				ReportPct:     0.2,
				ImagePct:      0.1,
				MinDurationMs: 100,
				MaxDurationMs: 500,
			},
			{
				Name:          "report_heavy",
				EmailPct:      0.2,
				ReportPct:     0.6,
				ImagePct:      0.2,
				MinDurationMs: 100,
				MaxDurationMs: 500,
			},
			{
				Name:          "balanced",
				EmailPct:      0.34,
				ReportPct:     0.33,
				ImagePct:      0.33,
				MinDurationMs: 100,
				MaxDurationMs: 500,
			},
		},
	}
}

// DefaultSustainedRateConfig returns a SustainedRateConfig with sensible defaults.
func DefaultSustainedRateConfig(quick bool) *SustainedRateConfig {
	base := experimentBase("sustained_rate", quick)
	var rates []float64
	var dur time.Duration
	if quick {
		rates = []float64{5, 10, 20}
		dur = 15 * time.Second
		base.TaskCount = 50
		base.Timeout = 1 * time.Minute
	} else {
		rates = []float64{5, 10, 25, 50}
		dur = 30 * time.Second
		base.TaskCount = 200
		base.Timeout = 3 * time.Minute
	}
	return &SustainedRateConfig{
		ExperimentConfig: base,
		Rates:            rates,
		Duration:         dur,
		MinDurationMs:    100,
		MaxDurationMs:    500,
	}
}

// ---------------------------------------------------------------------------
// Flag parsing
// ---------------------------------------------------------------------------

// ParseBenchmarkFlags parses command-line flags and returns a BenchmarkConfig.
func ParseBenchmarkFlags() *BenchmarkConfig {
	url := flag.String("url", "http://localhost:8080", "Base URL of the task queue API")
	workers := flag.Int("workers", 3, "Worker count the server was started with (metadata for reports)")
	suite := flag.String("suite", "comprehensive", "Benchmark suite: quick, comprehensive, or custom")
	experiments := flag.String("experiments", "", "Comma-separated experiment names to run (empty = all in suite)")
	resultsDir := flag.String("results-dir", "results", "Directory to write result files")
	verbose := flag.Bool("verbose", false, "Enable verbose output")

	tasks := flag.Int("tasks", 0, "Override task count for all experiments (0 = use suite default)")
	concurrency := flag.Int("concurrency", 0, "Override submission concurrency (0 = use suite default)")
	pollConcurrency := flag.Int("poll-concurrency", 0, "Override polling concurrency (0 = use suite default)")
	pollInterval := flag.Duration("poll-interval", 0, "Override poll interval (0 = use suite default)")
	timeout := flag.Duration("timeout", 0, "Override timeout per experiment (0 = use suite default)")
	repetitions := flag.Int("repetitions", 0, "Override repetitions (0 = use suite default)")
	failureProb := flag.Float64("failure-probability", 0.1, "Probability of simulated task failure (0-1)")

	flag.Parse()

	var expList []string
	if *experiments != "" {
		for _, e := range strings.Split(*experiments, ",") {
			trimmed := strings.TrimSpace(e)
			if trimmed != "" {
				expList = append(expList, trimmed)
			}
		}
	}

	cfg := &BenchmarkConfig{
		APIURL:                  *url,
		WorkerCount:             *workers,
		Suite:                   *suite,
		Experiments:             expList,
		ResultsDir:              *resultsDir,
		Verbose:                 *verbose,
		GitRevision:             detectGitRevision(),
		taskCountOverride:       *tasks,
		concurrencyOverride:     *concurrency,
		pollConcurrencyOverride: *pollConcurrency,
		pollIntervalOverride:    *pollInterval,
		timeoutOverride:         *timeout,
		repetitionsOverride:     *repetitions,
		failureProbability:      *failureProb,
	}

	// If any override was specified, switch to "custom" suite automatically.
	if cfg.taskCountOverride > 0 || cfg.concurrencyOverride > 0 ||
		cfg.pollConcurrencyOverride > 0 || cfg.pollIntervalOverride > 0 ||
		cfg.timeoutOverride > 0 || cfg.repetitionsOverride > 0 {
		if cfg.Suite != "custom" && len(expList) == 0 {
			// keep the chosen suite as the baseline; overrides are layered on top
		}
	}

	return cfg
}

// ApplyOverrides applies any flag-level overrides from the BenchmarkConfig onto
// an ExperimentConfig, returning the modified copy.
func (c *BenchmarkConfig) ApplyOverrides(ec ExperimentConfig) ExperimentConfig {
	if c.taskCountOverride > 0 {
		ec.TaskCount = c.taskCountOverride
	}
	if c.concurrencyOverride > 0 {
		ec.SubmissionConcurrency = c.concurrencyOverride
	}
	if c.pollConcurrencyOverride > 0 {
		ec.PollingConcurrency = c.pollConcurrencyOverride
	}
	if c.pollIntervalOverride > 0 {
		ec.PollInterval = c.pollIntervalOverride
	}
	if c.timeoutOverride > 0 {
		ec.Timeout = c.timeoutOverride
	}
	if c.repetitionsOverride > 0 {
		ec.Repetitions = c.repetitionsOverride
	}
	ec.FailureProbability = c.failureProbability
	return ec
}

// SelectedExperiments returns the slice of experiment names that should run.
// If explicitly specified in config, that list is returned.
// Otherwise, it returns the default experiments for the selected suite.
func (c *BenchmarkConfig) SelectedExperiments() []string {
	if len(c.Experiments) > 0 {
		return c.Experiments
	}
	if c.IsQuick() {
		return []string{"baseline", "worker-scaling", "burst", "workload-profiles", "sustained-load"}
	}
	// Comprehensive suite
	return []string{"baseline", "worker-scaling", "burst", "workload-profiles", "sustained-load", "large-table"}
}

// SubmissionConcurrency returns the global submission concurrency limit.
func (c *BenchmarkConfig) SubmissionConcurrency() int {
	if c.concurrencyOverride > 0 {
		return c.concurrencyOverride
	}
	if c.IsQuick() {
		return 5
	}
	return 10
}

// PollingConcurrency returns the global polling concurrency limit.
func (c *BenchmarkConfig) PollingConcurrency() int {
	if c.pollConcurrencyOverride > 0 {
		return c.pollConcurrencyOverride
	}
	if c.IsQuick() {
		return 10
	}
	return 20
}

// IsQuick reports whether the suite is "quick".
func (c *BenchmarkConfig) IsQuick() bool {
	return c.Suite == "quick"
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// detectGitRevision attempts to determine the current short git revision.
// Returns "unknown" if git is unavailable or the working directory is not a repo.
func detectGitRevision() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	rev := strings.TrimSpace(string(out))
	if rev == "" {
		return "unknown"
	}
	return rev
}
