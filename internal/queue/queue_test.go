package queue

import (
	"errors"
	"testing"
	"sync"
	"fmt"

	"github.com/HaseethVardhan/go-task-queue/internal/task"
)

func TestEnqueueAndDequeue(t *testing.T) {
	q, err := New(5)
	if err != nil {
		t.Fatalf("unexpected error while creating queue: %v", err)
	}

	want := task.Task{
		ID:   "task-1",
		Type: "email",
	}

	if err := q.Enqueue(want); err != nil {
		t.Fatalf("unexpected error while enqueueing: %v", err)
	}

	got, ok := q.Dequeue()
	if !ok {
		t.Fatal("expected a task, but queue was empty")
	}

	if got.ID != want.ID {
		t.Errorf("got ID %q, want %q", got.ID, want.ID)
	}

	if got.Type != want.Type {
		t.Errorf("got Type %q, want %q", got.Type, want.Type)
	}
}

func TestFIFOOrder(t *testing.T) {
	q, err := New(5)
	if err != nil {
		t.Fatalf("unexpected error while creating queue: %v", err)
	}

	tasks := []task.Task{
		{ID: "task-1", Type: "email"},
		{ID: "task-2", Type: "report"},
		{ID: "task-3", Type: "notification"},
	}

	for _, task := range tasks {
		if err := q.Enqueue(task); err != nil {
			t.Fatalf("unexpected error while enqueueing: %v", err)
		}
	}

	for _, want := range tasks {
		got, ok := q.Dequeue()
		if !ok {
			t.Fatal("expected a task, but queue was empty")
		}

		if got.ID != want.ID {
			t.Errorf("got ID %q, want %q", got.ID, want.ID)
		}
	}
}

func TestDequeueEmptyQueue(t *testing.T) {
	q, err := New(5)
	if err != nil {
		t.Fatalf("unexpected error while creating queue: %v", err)
	}

	_, ok := q.Dequeue()

	if ok {
		t.Fatal("expected no task from an empty queue")
	}
}

func TestQueueCapacity(t *testing.T) {
	q, err := New(1)
	if err != nil {
		t.Fatalf("unexpected error while creating queue: %v", err)
	}

	first := task.Task{ID: "task-1", Type: "email"}
	second := task.Task{ID: "task-2", Type: "report"}

	if err := q.Enqueue(first); err != nil {
		t.Fatalf("unexpected error while enqueueing first task: %v", err)
	}

	err = q.Enqueue(second)

	if !errors.Is(err, ErrQueueFull) {
		t.Fatalf("got error %v, want %v", err, ErrQueueFull)
	}
}


func TestConcurrentEnqueue(t *testing.T) {
	const goroutines = 100
	q, err := New(goroutines)
	if err != nil {
		t.Fatalf("unexpected error while creating queue: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()

			task := task.Task{
				ID:   fmt.Sprintf("task-%d", id),
				Type: "demo",
			}

			if err := q.Enqueue(task); err != nil {
				t.Errorf("failed to enqueue task: %v", err)
			}
		}(i)
	}

	wg.Wait()

	if got := q.Len(); got != goroutines {
		t.Errorf("queue length = %d, want %d", got, goroutines)
	}
}
