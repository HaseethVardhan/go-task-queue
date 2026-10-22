package storage

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/HaseethVardhan/go-task-queue/internal/task"
)

func setupTestRepository(t *testing.T) (*TaskRepository, context.Context) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	db, err := OpenPostgres(
		ctx,
		"postgres://gotask:gotask_dev_password@localhost:5432/gotask?sslmode=disable",
	)
	if err != nil {
		cancel()
		t.Fatalf("open postgres: %v", err)
	}

	t.Cleanup(func() {
		cancel()
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})

	return NewTaskRepository(db), ctx
}

func TestTaskRepository_CreateAndGetByID(t *testing.T) {
	repo, ctx := setupTestRepository(t)

	original := task.Task{
		ID:      "test-task-001",
		Type:    "email",
		Payload: json.RawMessage(`{"to":"test@example.com","subject":"Hello"}`),
	}

	if err := repo.Create(ctx, original); err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Register cleanup immediately after successful insertion.
	t.Cleanup(func() {
		if _, err := repo.db.ExecContext(
			context.Background(),
			`DELETE FROM tasks WHERE id = $1`,
			original.ID,
		); err != nil {
			t.Errorf("delete test task: %v", err)
		}
	})

	got, err := repo.GetByID(ctx, original.ID)
	if err != nil {
		t.Fatalf("get task by ID: %v", err)
	}

	if got.ID != original.ID {
		t.Errorf("ID = %q, want %q", got.ID, original.ID)
	}

	if got.Type != original.Type {
		t.Errorf("Type = %q, want %q", got.Type, original.Type)
	}

	var gotPayload, wantPayload any

	if err := json.Unmarshal(got.Payload, &gotPayload); err != nil {
		t.Fatalf("decode retrieved payload: %v", err)
	}

	if err := json.Unmarshal(original.Payload, &wantPayload); err != nil {
		t.Fatalf("decode original payload: %v", err)
	}

	gotJSON, err := json.Marshal(gotPayload)
	if err != nil {
		t.Fatalf("marshal retrieved payload: %v", err)
	}

	wantJSON, err := json.Marshal(wantPayload)
	if err != nil {
		t.Fatalf("marshal original payload: %v", err)
	}

	if string(gotJSON) != string(wantJSON) {
		t.Errorf("Payload = %s, want %s", gotJSON, wantJSON)
	}
}

func TestTaskRepository_GetByID_NotFound(t *testing.T) {
	repo, ctx := setupTestRepository(t)

	_, err := repo.GetByID(ctx, "nonexistent-task-999")
	if err == nil {
		t.Fatal("expected an error for a nonexistent task, got nil")
	}
}