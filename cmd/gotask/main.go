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

	"github.com/HaseethVardhan/go-task-queue/internal/service"
	"github.com/HaseethVardhan/go-task-queue/internal/storage"
	"github.com/HaseethVardhan/go-task-queue/internal/task"
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
	defer cancel()

	db, err := storage.OpenPostgres(dbCtx, dbURL)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}

	repo := storage.NewTaskRepository(db)
	taskService := service.NewTaskService(repo)

	newTask := task.Task{
		ID: fmt.Sprintf("demo-%d", time.Now().UnixNano()),
		Type: "demo",
		Payload: json.RawMessage(`{"message":"Hello from GoTask"}`),
	}

	if err := taskService.Submit(ctx, newTask); err != nil {
		log.Fatalf("submit task: %v", err)
	}

	log.Printf("Task %s submitted successfully", newTask.ID)

	// q, err := queue.New(10)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// ctx, cancel := context.WithCancel(context.Background())
	// defer cancel()

	// var wg sync.WaitGroup

	// registry := handler.NewRegistry()

	// err = registry.Register("demo", func(ctx context.Context, t task.Task) error {
	// 	fmt.Printf("Executing task %s with payload: %s\n", t.ID, t.Payload)
	// 	time.Sleep(time.Second)
	// 	return nil
	// })

	// if err != nil {
	// 	log.Fatal(err)
	// }

	// for i := 1; i <= 3; i++ {
	// 	w := worker.New(i, q, registry)

	// 	wg.Add(1)
	// 	go func() {
	// 		defer wg.Done()
	// 		w.Start(ctx)
	// 	}()
	// }

	// for i := 1; i <= 10; i++ {
	// 	payload, err := json.Marshal(map[string]string{
	// 		"message": fmt.Sprintf("Hello from task %d", i),
	// 	})

	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}

	// 	t := task.Task{
	// 		ID:      fmt.Sprintf("task-%d", i),
	// 		Type:    "demo",
	// 		Payload: payload,
	// 	}

	// 	if err := q.Enqueue(t); err != nil {
	// 		log.Printf("Failed to enqueue %s: %v", t.ID, err)
	// 	}
	// }

	// fmt.Println("Submitted all tasks.")

	// for q.Len() > 0 {
	// 	time.Sleep(100 * time.Millisecond)
	// }

	// cancel()
	// wg.Wait()

	// fmt.Println("All workers stopped.")
}
