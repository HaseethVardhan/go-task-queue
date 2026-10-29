package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	"net/http"

	"github.com/HaseethVardhan/go-task-queue/internal/handler"
	"github.com/HaseethVardhan/go-task-queue/internal/storage"
	"github.com/HaseethVardhan/go-task-queue/internal/worker"
	"github.com/HaseethVardhan/go-task-queue/internal/api"
	"github.com/HaseethVardhan/go-task-queue/internal/service"
	"github.com/HaseethVardhan/go-task-queue/internal/simulation"
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

	registry := handler.NewRegistry()

	if err := simulation.RegisterHandlers(registry); err != nil {
		log.Fatalf("register simulation handlers: %v", err)
    }

	w := worker.New(1, repo, registry)
	go w.Start(ctx)

	mux := http.NewServeMux()

	taskService := service.NewTaskService(repo)
	apiHandler := api.NewHandler(taskService, repo)
	apiHandler.RegisterRoutes(mux)

	server := api.NewServer(mux)

	serverErr := make(chan error, 1)

	go func() {
		log.Printf("GoTask API listening on %s", server.Addr)
		serverErr <- server.ListenAndServe()
	}()

	select {
		case <- ctx.Done():
			log.Println("Shutting down GoTask...")
		
		case err := <-serverErr:
			if err != nil && err != http.ErrServerClosed {
				log.Fatalf("HTTP server failed: %v", err)
			}
	}

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown failed: %v", err)		}
}
