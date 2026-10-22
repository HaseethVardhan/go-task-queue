package worker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/HaseethVardhan/go-task-queue/internal/handler"
	"github.com/HaseethVardhan/go-task-queue/internal/task"
)

type TaskRepository interface {
	ClaimNext(ctx context.Context) (task.Task, bool, error)
	Complete(ctx context.Context, id string) error
}

type Worker struct {
	ID       int
	Repo     TaskRepository
	Registry *handler.Registry
}

func New(id int, repo TaskRepository, registry *handler.Registry) *Worker {
	if repo == nil {
		panic("task repository cannot be nil")
	}
	if registry == nil {
		panic("handler registry cannot be nil")
	}

	return &Worker{
		ID:       id,
		Repo:     repo,
		Registry: registry,
	}
}

func (w *Worker) Start(ctx context.Context) {
	fmt.Printf("Worker %d started\n", w.ID)

	for {
		if err := ctx.Err(); err != nil {
			fmt.Printf("Worker %d shutting down\n", w.ID)
			return
		}

		t, ok, err := w.Repo.ClaimNext(ctx)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, context.Canceled) {
				fmt.Printf("Worker %d shutting down\n", w.ID)
				return
			}

			fmt.Printf("Worker %d: claim task failed: %v\n", w.ID, err)

			if !sleep(ctx, time.Second) {
				fmt.Printf("Worker %d shutting down\n", w.ID)
				return
			}
			continue
		}

		if !ok {
			if !sleep(ctx, 100*time.Millisecond) {
				fmt.Printf("Worker %d shutting down\n", w.ID)
				return
			}
			continue
		}

		w.processTask(ctx, t)
	}
}

func (w *Worker) processTask(ctx context.Context, t task.Task) {
	fmt.Printf("Worker %d started task %s (%s)\n", w.ID, t.ID, t.Type)

	h, err := w.Registry.Get(t.Type)
	if err != nil {
		fmt.Printf("Worker %d: cannot execute task %s: %v\n", w.ID, t.ID, err)
		return
	}

	if err := h(ctx, t); err != nil {
		fmt.Printf("Worker %d: task %s failed: %v\n", w.ID, t.ID, err)
		return
	}

	if err := w.Repo.Complete(ctx, t.ID); err != nil {
		fmt.Printf("Worker %d: completing task %s failed: %v\n", w.ID, t.ID, err)
		return
	}

	fmt.Printf("Worker %d completed task %s\n", w.ID, t.ID)
}

func sleep(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}