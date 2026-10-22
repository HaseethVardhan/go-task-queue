package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/HaseethVardhan/go-task-queue/internal/task"
)

type TaskRepository struct {
	db *sql.DB
}

func NewTaskRepository(db *sql.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) Create(ctx context.Context, t task.Task) error {
	payload := t.Payload
	if payload == nil {
		payload = json.RawMessage(`{}`)
	}

	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO tasks (id, type, payload)
		VALUES ($1, $2, $3)`,
		t.ID,
		t.Type,
		payload,
	)

	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}

	return nil
}

func (r *TaskRepository) GetByID(ctx context.Context, id string) (task.Task, error) {
	var t task.Task
	var payload []byte

	err := r.db.QueryRowContext(
		ctx,
		`SELECT id, type, payload FROM tasks WHERE id = $1`,
		id,
	).Scan(&t.ID, &t.Type, &payload)

	if err != nil {
		if err == sql.ErrNoRows {
			return task.Task{}, fmt.Errorf("task %q not found: %w", id, err)
		}

		return task.Task{}, fmt.Errorf("get task: %w", err)
	}

	t.Payload = json.RawMessage(payload)
	return t, nil
}
