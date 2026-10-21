package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/HaseethVardhan/go-task-queue/internal/task"
)

func TestRegisterAndGet(t *testing.T) {
	r := NewRegistry()

	expected := Handler(func(ctx context.Context, task task.Task) error {
		return nil
	})

	if err := r.Register("email", expected); err != nil {
		t.Fatalf("Register() returned unexpected error: %v", err)
	}

	got, err := r.Get("email")
	if err != nil {
		t.Fatalf("Get() returned unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("Get() returned a nil handler")
	}
}

func TestGetUnregisteredHandler(t *testing.T) {
	r := NewRegistry()

	_, err := r.Get("email")
	if !errors.Is(err, ErrHandlerNotFound) {
		t.Fatalf("got error %v, want %v", err, ErrHandlerNotFound)
	}
}

func TestRegisterDuplicateHandler(t *testing.T) {
	r := NewRegistry()

	h := Handler(func(ctx context.Context, task task.Task) error {
		return nil
	})

	if err := r.Register("email", h); err != nil {
		t.Fatalf("first Register() returned unexpected error: %v", err)
	}

	if err := r.Register("email", h); err == nil {
		t.Fatal("expected an error when registering a duplicate handler")
	}
}

func TestRegisterInvalidHandler(t *testing.T) {
	r := NewRegistry()

	if err := r.Register("", func(ctx context.Context, task task.Task) error {
		return nil
	}); err == nil {
		t.Fatal("expected an error for an empty task type")
	}

	if err := r.Register("email", nil); err == nil {
		t.Fatal("expected an error for a nil handler")
	}
}
