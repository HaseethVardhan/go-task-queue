package worker

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/HaseethVardhan/go-task-queue/internal/handler"
	"github.com/HaseethVardhan/go-task-queue/internal/task"
)

type mockTaskRepository struct {
	mu          sync.Mutex
	tasks       []task.Task
	completed   []string
	failed      []string
	claimErr    error
	completeErr error
	failErr     error
}

func (m *mockTaskRepository) ClaimNext(
	ctx context.Context,
) (task.Task, bool, error) {
	if err := ctx.Err(); err != nil {
		return task.Task{}, false, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.claimErr != nil {
		return task.Task{}, false, m.claimErr
	}

	if len(m.tasks) == 0 {
		return task.Task{}, false, nil
	}

	t := m.tasks[0]
	m.tasks = m.tasks[1:]

	return t, true, nil
}

func (m *mockTaskRepository) Complete(
	ctx context.Context,
	id string,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.completeErr != nil {
		return m.completeErr
	}

	m.completed = append(m.completed, id)
	return nil
}

func (m *mockTaskRepository) Fail(
	ctx context.Context,
	id string,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.failErr != nil {
		return m.failErr
	}

	m.failed = append(m.failed, id)
	return nil
}

func (m *mockTaskRepository) completedIDs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]string(nil), m.completed...)
}

func (m *mockTaskRepository) failedIDs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]string(nil), m.failed...)
}

func newTestWorker(
	t *testing.T,
	repo TaskRepository,
) (*Worker, *handler.Registry) {
	t.Helper()

	registry := handler.NewRegistry()
	w := New(1, repo, registry)

	return w, registry
}

func TestWorker_ProcessTask_Success(t *testing.T) {
	repo := &mockTaskRepository{}
	w, registry := newTestWorker(t, repo)

	var handlerCalled bool

	err := registry.Register("email", func(
		ctx context.Context,
		got task.Task,
	) error {
		handlerCalled = true

		if got.ID != "task-001" {
			t.Errorf("handler received task ID %q, want %q", got.ID, "task-001")
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	taskToProcess := task.Task{
		ID:   "task-001",
		Type: "email",
	}

	w.processTask(context.Background(), taskToProcess)

	if !handlerCalled {
		t.Fatal("handler was not called")
	}

	got := repo.completedIDs()
	want := []string{"task-001"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("completed IDs = %v, want %v", got, want)
	}
}

func TestWorker_ProcessTask_HandlerError(t *testing.T) {
	repo := &mockTaskRepository{}
	w, registry := newTestWorker(t, repo)

	handlerErr := errors.New("email delivery failed")

	err := registry.Register("email", func(
		ctx context.Context,
		got task.Task,
	) error {
		return handlerErr
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	w.processTask(context.Background(), task.Task{
		ID:   "task-002",
		Type: "email",
	})

	if got := repo.completedIDs(); len(got) != 0 {
		t.Errorf("completed IDs = %v, want no completed tasks", got)
	}

	gotFailed := repo.failedIDs()
	wantFailed := []string{"task-002"}
	if !reflect.DeepEqual(gotFailed, wantFailed) {
		t.Errorf("failed IDs = %v, want %v", gotFailed, wantFailed)
	}
}

func TestWorker_ProcessTask_HandlerNotFound(t *testing.T) {
	repo := &mockTaskRepository{}
	w, _ := newTestWorker(t, repo)

	w.processTask(context.Background(), task.Task{
		ID:   "task-003",
		Type: "unknown",
	})

	if got := repo.completedIDs(); len(got) != 0 {
		t.Errorf("completed IDs = %v, want no completed tasks", got)
	}

	gotFailed := repo.failedIDs()
	wantFailed := []string{"task-003"}
	if !reflect.DeepEqual(gotFailed, wantFailed) {
		t.Errorf("failed IDs = %v, want %v", gotFailed, wantFailed)
	}
}

func TestWorker_ProcessTask_FailError(t *testing.T) {
	repo := &mockTaskRepository{
		failErr: errors.New("database unavailable"),
	}
	w, registry := newTestWorker(t, repo)

	err := registry.Register("email", func(
		ctx context.Context,
		got task.Task,
	) error {
		return errors.New("email delivery failed")
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	w.processTask(context.Background(), task.Task{
		ID:   "task-006",
		Type: "email",
	})

	if got := repo.completedIDs(); len(got) != 0 {
		t.Errorf("completed IDs = %v, want no completed tasks", got)
	}

	if got := repo.failedIDs(); len(got) != 0 {
		t.Errorf("failed IDs = %v, want no recorded failures", got)
	}
}

func TestWorker_ProcessTask_CompleteError(t *testing.T) {
	repo := &mockTaskRepository{
		completeErr: errors.New("database unavailable"),
	}
	w, registry := newTestWorker(t, repo)

	err := registry.Register("email", func(
		ctx context.Context,
		got task.Task,
	) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	w.processTask(context.Background(), task.Task{
		ID:   "task-004",
		Type: "email",
	})

	if got := repo.completedIDs(); len(got) != 0 {
		t.Errorf("completed IDs = %v, want no recorded completions", got)
	}
}

func TestWorker_Start_CancelledContext(t *testing.T) {
	repo := &mockTaskRepository{}
	w, _ := newTestWorker(t, repo)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})

	go func() {
		defer close(done)
		w.Start(ctx)
	}()

	select {
	case <-done:
		// Worker exited as expected.
	case <-time.After(time.Second):
		t.Fatal("worker did not shut down after context cancellation")
	}
}

func TestWorker_Start_ClaimsAndProcessesTask(t *testing.T) {
	repo := &mockTaskRepository{
		tasks: []task.Task{
			{
				ID:   "task-005",
				Type: "email",
			},
		},
	}

	w, registry := newTestWorker(t, repo)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := registry.Register("email", func(
		ctx context.Context,
		got task.Task,
	) error {
		cancel()
		return nil
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	done := make(chan struct{})

	go func() {
		defer close(done)
		w.Start(ctx)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop after processing the task")
	}

	got := repo.completedIDs()
	want := []string{"task-005"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("completed IDs = %v, want %v", got, want)
	}
}

func TestWorker_Start_ClaimsAndFailsTask(t *testing.T) {
	repo := &mockTaskRepository{
		tasks: []task.Task{
			{
				ID:   "task-007",
				Type: "email",
			},
		},
	}

	w, registry := newTestWorker(t, repo)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := registry.Register("email", func(
		ctx context.Context,
		got task.Task,
	) error {
		defer cancel()
		return errors.New("simulated failure")
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	done := make(chan struct{})

	go func() {
		defer close(done)
		w.Start(ctx)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop after failing the task")
	}

	if got := repo.completedIDs(); len(got) != 0 {
		t.Errorf("completed IDs = %v, want no completed tasks", got)
	}

	gotFailed := repo.failedIDs()
	wantFailed := []string{"task-007"}

	if !reflect.DeepEqual(gotFailed, wantFailed) {
		t.Errorf("failed IDs = %v, want %v", gotFailed, wantFailed)
	}
}
