package handler

import (
	"context"
	"errors"
	"fmt"

	"github.com/HaseethVardhan/go-task-queue/internal/task"
)

var ErrHandlerNotFound = errors.New("Handler not found")

type Handler func(ctx context.Context, t task.Task) error

type Registry struct {
	handlers map[string]Handler
}

func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]Handler),
	}
}

func (r *Registry) Register(taskType string, h Handler) error {
	if taskType == "" {
		return errors.New("task type cannot be empty")
	}

	if h == nil {
		return errors.New("handler cannot be nil")
	}

	if _, exists := r.handlers[taskType]; exists {
		return fmt.Errorf("handler already registered for task type %q", taskType)
	}

	r.handlers[taskType] = h
	return nil
}

func (r *Registry) Get(taskType string) (Handler, error) {
	h, exists := r.handlers[taskType]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrHandlerNotFound, taskType)
	}

	return h, nil
}
