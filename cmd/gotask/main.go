package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/HaseethVardhan/go-task-queue/internal/handler"
	"github.com/HaseethVardhan/go-task-queue/internal/service"
	"github.com/HaseethVardhan/go-task-queue/internal/storage"
	"github.com/HaseethVardhan/go-task-queue/internal/task"
	"github.com/HaseethVardhan/go-task-queue/internal/worker"
)

func main() {

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	db, err := storage.OpenPostgres(dbCtx, dbURL)
	cancel()
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer db.Close()

	repo := storage.NewTaskRepository(db)
	taskService := service.NewTaskService(repo)

	registry := handler.NewRegistry()

	if err := registry.Register("demo", func(
		ctx context.Context,
		t task.Task,
	) error {
		var payload struct {
			Message string `json:"message"`
		}

		if err := json.Unmarshal(t.Payload, &payload); err != nil {
			return fmt.Errorf("decode demo payload: %w", err)
		}

		log.Printf("Executing task %s: %s", t.ID, payload.Message)
		return nil
	}); err != nil {
		log.Fatalf("register demo handler: %v", err)
	}

	w := worker.New(1, repo, registry)
	go w.Start(ctx)

	newTask := task.Task{
		ID:      fmt.Sprintf("demo-%d", time.Now().UnixNano()),
		Type:    "demo",
		Payload: json.RawMessage(`{"message":"Hello from GoTask"}`),
	}

	if err := taskService.Submit(ctx, newTask); err != nil {
		log.Fatalf("submit task: %v", err)
	}

	log.Printf("Submitted task %s", newTask.ID)
	log.Println("GoTask is running. Press Ctrl+C to stop.")

	<-ctx.Done()
	log.Println("Shutting down GoTask...")
}
