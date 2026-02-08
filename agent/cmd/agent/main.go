package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sharedco/cilo/agent/pkg/agent"
	"github.com/sharedco/cilo/agent/pkg/config"
)

func main() {
	cfg := config.Load()

	srv, err := agent.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("Shutdown error: %v", err)
		}
	}()

	log.Printf("Starting cilo-agent on %s", cfg.ListenAddr)
	if err := srv.Start(); err != nil {
		log.Fatalf("Agent error: %v", err)
	}
}
