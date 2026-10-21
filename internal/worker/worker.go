package worker

import (
	"fmt"
	"time"
	"context"

	"github.com/HaseethVardhan/go-task-queue/internal/queue"
)

type Worker struct {
	ID int
	Queue *queue.Queue
}

func New(id int, q *queue.Queue) *Worker {
	return &Worker{
		ID:    id,
		Queue: q,
	}
}

func (w *Worker) Start(ctx context.Context) {
	for {
		select {
		case <- ctx.Done():
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

		fmt.Printf("Payload: %s\n", t.Payload)

		time.Sleep(1 * time.Second)

		fmt.Printf("Worker %d completed task %s\n", w.ID, t.ID)
	}
}