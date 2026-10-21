package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"context"
	"time"

	"github.com/HaseethVardhan/go-task-queue/internal/queue"
	"github.com/HaseethVardhan/go-task-queue/internal/task"
	"github.com/HaseethVardhan/go-task-queue/internal/worker"
)

func main() {
	q, err := queue.New(10)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	for i := 1; i <= 3; i++ {
		w := worker.New(i, q)

		wg.Add(1)
		go func() {
			defer wg.Done()
			w.Start(ctx)
		}()
	}

	for i := 1; i <= 10; i++ {
		payload, err := json.Marshal(map[string]string{
			"message": fmt.Sprintf("Hello from task %d", i),
		})

		if err != nil {
			log.Fatal(err)
		}

		t := task.Task{
			ID: fmt.Sprintf("task-%d", i),
			Type: "demo",
			Payload: payload,
		}

		if err := q.Enqueue(t); err != nil {
			log.Printf("Failed to enqueue %s: %v", t.ID, err)
		}
	}

	fmt.Println("Submitted all tasks.")

	for q.Len() > 0 {
		time.Sleep(100*time.Millisecond)
	}

	cancel()
	wg.Wait()

	fmt.Println("All workers stopped.")
}