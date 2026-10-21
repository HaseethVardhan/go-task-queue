package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/HaseethVardhan/go-task-queue/internal/handler"
	"github.com/HaseethVardhan/go-task-queue/internal/queue"
)

type Worker struct {
	ID       int
	Queue    *queue.Queue
	Registry *handler.Registry
}

func New(id int, q *queue.Queue, registry *handler.Registry) *Worker {
	return &Worker{
		ID:       id,
		Queue:    q,
		Registry: registry,
	}
}

func (w *Worker) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("Worker %d shutting down\n", w.ID)
			return
		default:
		}

		t, ok := w.Queue.Dequeue()
		if !ok {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		fmt.Printf("Worker %d started task %s (%s)\n", w.ID, t.ID, t.Type)

		h, err := w.Registry.Get(t.Type)
		if err != nil {
			fmt.Printf("Worker %d: cannot execute task %s: %v\n", w.ID, t.ID, err)
			continue
		}

		if err := h(ctx, t); err != nil {
			fmt.Printf("Worker %d: task %s failed: %v\n", w.ID, t.ID, err)
			continue
		}

		fmt.Printf("Worker %d completed task %s\n", w.ID, t.ID)
	}
}
