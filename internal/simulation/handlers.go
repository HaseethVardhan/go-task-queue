package simulation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/HaseethVardhan/go-task-queue/internal/handler"
	"github.com/HaseethVardhan/go-task-queue/internal/task"
)

const maxDurationMs = 60_000

type config struct {
	MinDurationMs *int `json:"minDurationMs"`
	MaxDurationMs    *int     `json:"maxDurationMs"`
	FailureProbability *float64 `json:"failureProbability"`
}

type simulation struct {
	name       string
	minMs      int
	maxMs      int
	stages     []string
}

func RegisterHandlers(registry *handler.Registry) error {
	handlers := []struct {
		taskType string
		handler  handler.Handler
	}{
		{
			taskType: "email.send",
			handler: simulation{
				name:   "email.send",
				minMs:  100,
				maxMs:  500,
				stages: []string{"sending email"},
			}.Handle,
		},
		{
			taskType: "report.generate",
			handler: simulation{
				name:  "report.generate",
				minMs: 1000,
				maxMs: 3000,
				stages: []string{
					"collecting data",
					"calculating results",
					"generating report",
				},
			}.Handle,
		},
		{
			taskType: "image.process",
			handler: simulation{
				name:  "image.process",
				minMs: 500,
				maxMs: 2000,
				stages: []string{
					"loading image",
					"processing image",
					"generating thumbnail",
				},
			}.Handle,
		},
	}

	for _, entry := range handlers {
		if err := registry.Register(entry.taskType, entry.handler); err != nil {
			return fmt.Errorf("register %s handler: %w", entry.taskType, err)
		}
	}

	return nil
}

func (s simulation) Handle(ctx context.Context, t task.Task) error {
	cfg := config{}

	if len(t.Payload) > 0 {
		if err := json.Unmarshal(t.Payload, &cfg); err != nil {
			return fmt.Errorf("decode %s payload: %w", s.name, err)
		}
	}

	minMs := s.minMs
	maxMs := s.maxMs
	failureProbability := 0.0

	if cfg.MinDurationMs != nil {
		minMs = *cfg.MinDurationMs
	}
	if cfg.MaxDurationMs != nil {
		maxMs = *cfg.MaxDurationMs
	}
	if cfg.FailureProbability != nil {
		failureProbability = *cfg.FailureProbability
	}

	if minMs < 0 || maxMs < minMs || maxMs > maxDurationMs {
		return fmt.Errorf(
			"invalid duration range: min=%dms max=%dms",
			minMs,
			maxMs,
		)
	}

	if failureProbability < 0 || failureProbability > 1 {
		return errors.New("failureProbability must be between 0 and 1")
	}

	durationMs := minMs
	if maxMs > minMs {
		durationMs += rand.Intn(maxMs - minMs + 1)
	}

	log.Printf(
		"Task %s: starting %s (simulated duration: %dms)",
		t.ID,
		s.name,
		durationMs,
	)

	stageDuration := time.Duration(durationMs) * time.Millisecond /
		time.Duration(len(s.stages))

	for _, stage := range s.stages {
		log.Printf("Task %s: %s", t.ID, stage)

		if err := sleep(ctx, stageDuration); err != nil {
			return fmt.Errorf("%s cancelled during %q: %w", s.name, stage, err)
		}
	}

	// Simulate a failure after the work has been performed.
	if rand.Float64() < failureProbability {
		return fmt.Errorf("%s: simulated execution failure", s.name)
	}

	log.Printf("Task %s: %s completed successfully", t.ID, s.name)
	return nil
}

func sleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}