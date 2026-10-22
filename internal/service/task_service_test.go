package service

import (
	"context"
	"errors"
	"testing"

	"github.com/HaseethVardhan/go-task-queue/internal/task"
)

type mockTaskRepository struct {
	createFunc func(context.Context, task.Task) error
	called     bool
	created    task.Task
}

func (m *mockTaskRepository) Create(
	ctx context.Context,
	t task.Task,
) error {
	m.called = true
	m.created = t

	if m.createFunc != nil {
		return m.createFunc(ctx, t)
	}

	return nil
}

func TestTaskService_Submit_Success(t *testing.T) {
	repo := &mockTaskRepository{}
	svc := NewTaskService(repo)

	input := task.Task{
		ID:      "task-001",
		Type:    "email",
		Payload: []byte(`{"to":"test@example.com"}`),
	}

	err := svc.Submit(context.Background(), input)
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}

	if !repo.called {
		t.Fatal("expected repository Create to be called")
	}

	if repo.created.ID != input.ID {
		t.Errorf("created ID = %q, want %q", repo.created.ID, input.ID)
	}

	if repo.created.Type != input.Type {
		t.Errorf("created type = %q, want %q", repo.created.Type, input.Type)
	}
}

func TestTaskService_Submit_EmptyID(t *testing.T) {
	repo := &mockTaskRepository{}
	svc := NewTaskService(repo)

	err := svc.Submit(context.Background(), task.Task{
		Type: "email",
	})

	if !errors.Is(err, ErrInvalidTaskID) {
		t.Errorf("Submit() error = %v, want %v", err, ErrInvalidTaskID)
	}

	if repo.called {
		t.Fatal("repository should not be called for an invalid task ID")
	}
}

func TestTaskService_Submit_EmptyType(t *testing.T) {
	repo := &mockTaskRepository{}
	svc := NewTaskService(repo)

	err := svc.Submit(context.Background(), task.Task{
		ID: "task-002",
	})

	if !errors.Is(err, ErrInvalidTaskType) {
		t.Errorf("Submit() error = %v, want %v", err, ErrInvalidTaskType)
	}

	if repo.called {
		t.Fatal("repository should not be called for an invalid task type")
	}
}

func TestTaskService_Submit_RepositoryError(t *testing.T) {
	expectedErr := errors.New("database unavailable")

	repo := &mockTaskRepository{
		createFunc: func(context.Context, task.Task) error {
			return expectedErr
		},
	}
	svc := NewTaskService(repo)

	err := svc.Submit(context.Background(), task.Task{
		ID:   "task-003",
		Type: "email",
	})

	if !errors.Is(err, expectedErr) {
		t.Errorf("Submit() error = %v, want wrapped error %v", err, expectedErr)
	}
}

func TestTaskService_Submit_CancelledContext(t *testing.T) {
	repo := &mockTaskRepository{}
	svc := NewTaskService(repo)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := svc.Submit(ctx, task.Task{
		ID:   "task-004",
		Type: "email",
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Submit() error = %v, want %v", err, context.Canceled)
	}

	if repo.called {
		t.Fatal("repository should not be called with a cancelled context")
	}
}
