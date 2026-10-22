package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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

func (r *TaskRepository) ClaimNext(ctx context.Context) (task.Task, bool, error) {
	var t task.Task
	var payload []byte

	err := r.db.QueryRowContext(
		ctx,
		`
		UPDATE tasks
		SET status = 'processing'
		WHERE id = (
			SELECT id
			FROM tasks
			WHERE status = 'pending'
			ORDER BY created_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING id, type, payload
		`,
	).Scan(&t.ID, &t.Type, &payload)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return task.Task{}, false, nil
		}
		return task.Task{}, false, fmt.Errorf("claim task: %w", err)
	}

	t.Payload = payload
	return t, true, nil
}

func (r *TaskRepository) Complete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(
		ctx,
		`
		UPDATE tasks
		SET status = 'completed'
		WHERE id = $1 AND status = 'processing'
		`,
		id,
	)

	if err != nil {
		return fmt.Errorf("complete task: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check completed task: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf(
			"complete task %q: task not found or not processing",
			id,
		)
	}

	return nil
}
