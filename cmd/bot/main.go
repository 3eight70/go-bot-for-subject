package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	version := os.Getenv("APP_VERSION")
	if version == "" {
		version = "dev"
	}
	fmt.Printf("go-bot starting (version: %s)\n", version)

	// Simulate background work loop until shutdown signal arrives.
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("shutdown signal received, exiting gracefully...")
			// Place for cleanup if needed (closing DB, flush logs, etc.)
			fmt.Println("goodbye!")
			return
		case t := <-ticker.C:
			fmt.Printf("[%s] bot tick...\n", t.Format(time.RFC3339))
		}
	}
}
