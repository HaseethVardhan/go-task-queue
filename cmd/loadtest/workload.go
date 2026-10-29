package main

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
)

// (WorkloadProfile is defined in config.go)

// GenerateMixedWorkload creates a slice of TaskSpec entries distributed
// in round-robin order across email.send, report.generate, and image.process.
func GenerateMixedWorkload(count int, failProb float64, minMs, maxMs int) []TaskSpec {
	tasks := make([]TaskSpec, 0, count)
	for i := 0; i < count; i++ {
		switch i % 3 {
		case 0:
			tasks = append(tasks, TaskSpec{
				Type: "email.send",
				Payload: TaskPayload{
					Message:            fmt.Sprintf("Benchmark email %d", i),
					MinDurationMs:      minMs,
					MaxDurationMs:      maxMs,
					FailureProbability: failProb,
				},
			})
		case 1:
			tasks = append(tasks, TaskSpec{
				Type: "report.generate",
				Payload: TaskPayload{
					ReportType:         "benchmark-report",
					MinDurationMs:      minMs,
					MaxDurationMs:      maxMs,
					FailureProbability: failProb,
				},
			})
		case 2:
			tasks = append(tasks, TaskSpec{
				Type: "image.process",
				Payload: TaskPayload{
					ImageName:          fmt.Sprintf("benchmark-%d.jpg", i),
					MinDurationMs:      minMs,
					MaxDurationMs:      maxMs,
					FailureProbability: failProb,
				},
			})
		}
	}
	return tasks
}

// GenerateProfileWorkload creates tasks distributed according to the given
// WorkloadProfile percentages and shuffles them deterministically.
func GenerateProfileWorkload(count int, profile WorkloadProfile, failProb float64) []TaskSpec {
	emailCount := int(math.Round(float64(count) * profile.EmailPct))
	reportCount := int(math.Round(float64(count) * profile.ReportPct))
	imageCount := count - emailCount - reportCount

	tasks := make([]TaskSpec, 0, count)

	for i := 0; i < emailCount; i++ {
		tasks = append(tasks, TaskSpec{
			Type: "email.send",
			Payload: TaskPayload{
				Message:            fmt.Sprintf("Benchmark email %d", i),
				MinDurationMs:      profile.MinDurationMs,
				MaxDurationMs:      profile.MaxDurationMs,
				FailureProbability: failProb,
			},
		})
	}

	for i := 0; i < reportCount; i++ {
		tasks = append(tasks, TaskSpec{
			Type: "report.generate",
			Payload: TaskPayload{
				ReportType:         "benchmark-report",
				MinDurationMs:      profile.MinDurationMs,
				MaxDurationMs:      profile.MaxDurationMs,
				FailureProbability: failProb,
			},
		})
	}

	for i := 0; i < imageCount; i++ {
		tasks = append(tasks, TaskSpec{
			Type: "image.process",
			Payload: TaskPayload{
				ImageName:          fmt.Sprintf("benchmark-%d.jpg", i),
				MinDurationMs:      profile.MinDurationMs,
				MaxDurationMs:      profile.MaxDurationMs,
				FailureProbability: failProb,
			},
		})
	}

	// Deterministic seed derived from count and profile name hash.
	h := fnv.New64a()
	h.Write([]byte(profile.Name))
	seed := int64(count) + int64(h.Sum64())
	ShuffleTasks(tasks, seed)

	return tasks
}

// GenerateBurstWorkload creates a workload suitable for burst experiments.
// It delegates to GenerateMixedWorkload for task generation.
func GenerateBurstWorkload(count int, failProb float64, minMs, maxMs int) []TaskSpec {
	return GenerateMixedWorkload(count, failProb, minMs, maxMs)
}

// rateCounter is a package-level counter used by GenerateRateWorkload to
// cycle through task types when the "mixed" type is requested.
var rateCounter int

// GenerateRateWorkload produces a single TaskSpec of the specified type.
// When taskType is "mixed", it cycles through the three types in round-robin
// order across successive calls.
func GenerateRateWorkload(taskType string, failProb float64, minMs, maxMs int) TaskSpec {
	actualType := taskType
	if taskType == "mixed" {
		types := []string{"email.send", "report.generate", "image.process"}
		actualType = types[rateCounter%3]
		rateCounter++
	}

	switch actualType {
	case "email.send":
		return TaskSpec{
			Type: "email.send",
			Payload: TaskPayload{
				Message:            fmt.Sprintf("Benchmark email %d", rateCounter),
				MinDurationMs:      minMs,
				MaxDurationMs:      maxMs,
				FailureProbability: failProb,
			},
		}
	case "report.generate":
		return TaskSpec{
			Type: "report.generate",
			Payload: TaskPayload{
				ReportType:         "benchmark-report",
				MinDurationMs:      minMs,
				MaxDurationMs:      maxMs,
				FailureProbability: failProb,
			},
		}
	case "image.process":
		return TaskSpec{
			Type: "image.process",
			Payload: TaskPayload{
				ImageName:          fmt.Sprintf("benchmark-%d.jpg", rateCounter),
				MinDurationMs:      minMs,
				MaxDurationMs:      maxMs,
				FailureProbability: failProb,
			},
		}
	default:
		// Fall back to email.send for unrecognised types.
		return TaskSpec{
			Type: "email.send",
			Payload: TaskPayload{
				Message:            fmt.Sprintf("Benchmark email %d", rateCounter),
				MinDurationMs:      minMs,
				MaxDurationMs:      maxMs,
				FailureProbability: failProb,
			},
		}
	}
}

// PredefinedProfiles returns a map of named WorkloadProfile configurations
// covering common distribution patterns.
func PredefinedProfiles() map[string]WorkloadProfile {
	return map[string]WorkloadProfile{
		"email-heavy": {
			Name:          "email-heavy",
			EmailPct:      0.7,
			ReportPct:     0.15,
			ImagePct:      0.15,
			MinDurationMs: 100,
			MaxDurationMs: 500,
		},
		"report-heavy": {
			Name:          "report-heavy",
			EmailPct:      0.15,
			ReportPct:     0.7,
			ImagePct:      0.15,
			MinDurationMs: 100,
			MaxDurationMs: 500,
		},
		"image-heavy": {
			Name:          "image-heavy",
			EmailPct:      0.15,
			ReportPct:     0.15,
			ImagePct:      0.7,
			MinDurationMs: 100,
			MaxDurationMs: 500,
		},
		"mixed": {
			Name:          "mixed",
			EmailPct:      0.34,
			ReportPct:     0.33,
			ImagePct:      0.33,
			MinDurationMs: 100,
			MaxDurationMs: 500,
		},
	}
}

// ShuffleTasks performs an in-place Fisher-Yates shuffle on the given slice
// using a deterministic pseudo-random source seeded with the provided value.
func ShuffleTasks(tasks []TaskSpec, seed int64) {
	r := rand.New(rand.NewSource(seed))
	for i := len(tasks) - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		tasks[i], tasks[j] = tasks[j], tasks[i]
	}
}
