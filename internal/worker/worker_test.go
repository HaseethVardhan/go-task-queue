package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/HaseethVardhan/go-task-queue/internal/handler"
	"github.com/HaseethVardhan/go-task-queue/internal/queue"
	"github.com/HaseethVardhan/go-task-queue/internal/task"
)

func TestWorkerExecutesRegisteredHandler(t *testing.T) {
	q, err := queue.New(5)
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}

	registry := handler.NewRegistry()

	executed := make(chan string, 1)

	err = registry.Register("email", func(ctx context.Context, task task.Task) error {
		executed <- task.ID
		return nil
	})
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	if err := q.Enqueue(task.Task{ID: "task-1", Type: "email"}); err != nil {
		t.Fatalf("failed to enqueue task: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	w := New(1, q, registry)

	done := make(chan struct{})
	go func() {
		defer close(done)
		w.Start(ctx)
	}()

	select {
	case id := <-executed:
		if id != "task-1" {
			t.Errorf("handler received task ID %q, want %q", id, "task-1")
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not execute the registered handler")
	}

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not shut down after cancellation")
	}
}

func TestWorkerHandlesHandlerError(t *testing.T) {
	q, err := queue.New(5)
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}
	registry := handler.NewRegistry()

	executed := make(chan struct{}, 1)

	err = registry.Register("email", func(ctx context.Context, task task.Task) error {
		executed <- struct{}{}
		return errors.New("simulated handler failure")
	})
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	if err := q.Enqueue(task.Task{ID: "task-1", Type: "email"}); err != nil {
		t.Fatalf("failed to enqueue task: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	w := New(1, q, registry)

	done := make(chan struct{})
	go func() {
		defer close(done)
		w.Start(ctx)
	}()

	select {
	case <-executed:
	case <-time.After(time.Second):
		cancel()
		t.Fatal("worker did not invoke the handler")
	}

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not shut down after cancellation")
	}
}

func TestWorkerHandlesUnknownTaskType(t *testing.T) {
	q, err := queue.New(5)
	if err != nil {
		t.Fatalf("failed to create queue: %v", err)
	}

	registry := handler.NewRegistry()

	if err := q.Enqueue(task.Task{ID: "task-1", Type: "unknown"}); err != nil {
		t.Fatalf("failed to enqueue task: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	w := New(1, q, registry)

	done := make(chan struct{})
	go func() {
		defer close(done)
		w.Start(ctx)
	}()

	deadline := time.After(time.Second)
	for q.Len() != 0 {
		select {
		case <-deadline:
			cancel()
			t.Fatal("worker did not dequeue the unknown task")
		case <-time.After(10 * time.Millisecond):
		}
	}

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not shut down after cancellation")
	}
}
