package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/HaseethVardhan/go-task-queue/internal/task"
)

var (
	ErrInvalidTaskID   = errors.New("task ID cannot be empty")
	ErrInvalidTaskType = errors.New("task type cannot be empty")
)

type TaskRepository interface {
	Create(ctx context.Context, t task.Task) error
}

type TaskService struct {
	repo TaskRepository
}

func NewTaskService(repo TaskRepository) *TaskService {
	if repo == nil {
		panic("task repository cannot be nil")
	}

	return &TaskService{
		repo: repo,
	}
}

func (s *TaskService) Submit(ctx context.Context, t task.Task) error {
	if t.ID == "" {
		return ErrInvalidTaskID
	}

	if t.Type == "" {
		return ErrInvalidTaskType
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := s.repo.Create(ctx, t); err != nil {
		return fmt.Errorf("submit task:%w", err)
	}

	return nil
}
