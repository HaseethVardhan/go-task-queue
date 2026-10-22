package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/HaseethVardhan/go-task-queue/internal/task"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	connString := os.Getenv("TEST_DATABASE_URL")
	if connString == "" {
		t.Fatal("TEST_DATABASE_URL must point to the dedicated test database")
	}

	u, err := url.Parse(connString)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}

	if u.Path != "/gotask_test" {
		t.Fatalf("refusing to run tests against database %q; expected gotask_test", u.Path)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := OpenPostgres(ctx, connString)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})

	return db
}

func insertTestTask(
	t *testing.T,
	db *sql.DB,
	id string,
	status string,
) {
	t.Helper()

	_, err := db.ExecContext(
		context.Background(),
		`INSERT INTO tasks (id, type, payload, status)
		 VALUES ($1, $2, $3, $4)`,
		id,
		"demo",
		[]byte(`{"message":"test"}`),
		status,
	)
	if err != nil {
		t.Fatalf("insert test task: %v", err)
	}

	t.Cleanup(func() {
		_, err := db.ExecContext(
			context.Background(),
			`DELETE FROM tasks WHERE id = $1`,
			id,
		)
		if err != nil {
			t.Errorf("clean up test task %q: %v", id, err)
		}
	})
}

func TestTaskRepository_CreateAndGetByID(t *testing.T) {
	db := openTestDB(t)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	want := task.Task{
		ID:      "test-create-get-001",
		Type:    "demo",
		Payload: []byte(`{"message":"hello"}`),
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM tasks WHERE id = $1`, want.ID)
	})

	if err := repo.Create(ctx, want); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := repo.GetByID(ctx, want.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if got.ID != want.ID {
		t.Errorf("ID = %q, want %q", got.ID, want.ID)
	}
	if got.Type != want.Type {
		t.Errorf("Type = %q, want %q", got.Type, want.Type)
	}
	var gotJSON, wantJSON any

	if err := json.Unmarshal(got.Payload, &gotJSON); err != nil {
		t.Fatalf("unmarshal retrieved payload: %v", err)
	}

	if err := json.Unmarshal(want.Payload, &wantJSON); err != nil {
		t.Fatalf("unmarshal expected payload: %v", err)
	}

	if !reflect.DeepEqual(gotJSON, wantJSON) {
		t.Errorf("Payload = %s, want %s", got.Payload, want.Payload)
	}
}

func TestTaskRepository_GetByID_NotFound(t *testing.T) {
	db := openTestDB(t)
	repo := NewTaskRepository(db)

	_, err := repo.GetByID(context.Background(), "missing-task-001")
	if err == nil {
		t.Fatal("GetByID() expected an error for a missing task")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetByID() error = %v, want an error wrapping sql.ErrNoRows", err)
	}
}

func TestTaskRepository_ClaimNext(t *testing.T) {
	db := openTestDB(t)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	insertTestTask(t, db, "claim-test-001", "pending")

	got, ok, err := repo.ClaimNext(ctx)
	if err != nil {
		t.Fatalf("ClaimNext() error = %v", err)
	}
	if !ok {
		t.Fatal("ClaimNext() returned no task, want one")
	}
	if got.ID != "claim-test-001" {
		t.Fatalf("claimed ID = %q, want %q", got.ID, "claim-test-001")
	}

	var status string
	if err := db.QueryRowContext(
		ctx,
		`SELECT status FROM tasks WHERE id = $1`,
		got.ID,
	).Scan(&status); err != nil {
		t.Fatalf("query claimed task status: %v", err)
	}

	if status != "processing" {
		t.Errorf("status = %q, want %q", status, "processing")
	}
}

func TestTaskRepository_ClaimNext_NoPendingTasks(t *testing.T) {
	db := openTestDB(t)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	// This test requires a clean test database with no pending tasks.
	var count int
	err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM tasks WHERE status = 'pending'`,
	).Scan(&count)
	if err != nil {
		t.Fatalf("count pending tasks: %v", err)
	}
	if count != 0 {
		t.Fatalf(
			"test database has %d pending tasks; clean the test database before running this test",
			count,
		)
	}

	_, ok, err := repo.ClaimNext(ctx)
	if err != nil {
		t.Fatalf("ClaimNext() error = %v", err)
	}
	if ok {
		t.Fatal("ClaimNext() returned a task; want no task")
	}
}

func TestTaskRepository_Complete(t *testing.T) {
	db := openTestDB(t)
	repo := NewTaskRepository(db)
	ctx := context.Background()

	insertTestTask(t, db, "complete-test-001", "processing")

	if err := repo.Complete(ctx, "complete-test-001"); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	var status string
	err := db.QueryRowContext(
		ctx,
		`SELECT status FROM tasks WHERE id = $1`,
		"complete-test-001",
	).Scan(&status)
	if err != nil {
		t.Fatalf("query task status: %v", err)
	}

	if status != "completed" {
		t.Errorf("status = %q, want %q", status, "completed")
	}
}

func TestTaskRepository_Complete_InvalidStatus(t *testing.T) {
	db := openTestDB(t)
	repo := NewTaskRepository(db)

	insertTestTask(t, db, "complete-pending-001", "pending")

	err := repo.Complete(context.Background(), "complete-pending-001")
	if err == nil {
		t.Fatal("Complete() expected an error for a pending task")
	}
}

func TestTaskRepository_Complete_NotFound(t *testing.T) {
	db := openTestDB(t)
	repo := NewTaskRepository(db)

	err := repo.Complete(context.Background(), "complete-missing-001")
	if err == nil {
		t.Fatal("Complete() expected an error for a missing task")
	}
}

func TestTaskRepository_ClaimNext_DatabaseError(t *testing.T) {
	// This checks a repository error without requiring a live database.
	// A nil receiver is not a valid way to test database errors, so this
	// test is intentionally omitted until we introduce an injectable DB layer.
	t.Skip("requires an injectable database layer")
}

var _ = fmt.Sprintf
