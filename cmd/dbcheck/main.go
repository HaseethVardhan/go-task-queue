
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/HaseethVardhan/go-task-queue/internal/storage"
)

func main() {
	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := storage.OpenPostgres(ctx, connString)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	fmt.Println("Successfully connected to PostgreSQL!")
}
