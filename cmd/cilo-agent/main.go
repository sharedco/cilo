// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sharedco/cilo/internal/agent"
	"github.com/sharedco/cilo/internal/agent/config"
	"github.com/sharedco/cilo/internal/version"
)

func main() {
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.Info())
		os.Exit(0)
	}

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
