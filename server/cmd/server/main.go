package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sharedco/cilo/server/pkg/api"
	"github.com/sharedco/cilo/server/pkg/config"
	"github.com/sharedco/cilo/server/pkg/store"
	"github.com/spf13/cobra"
)

// serveCmd runs the API server
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the API server",
	Long:  `Starts the Cilo API server that handles environment management and WireGuard networking.`,
	RunE:  runServer,
}

func init() {
	// Add serve command to root
	rootCmd.AddCommand(serveCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Fatal error: %v", err)
	}
}

func runServer(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Connect to database
	st, err := store.Connect(cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer st.Close()

	// Run migrations
	if err := store.RunMigrations(cfg.Database.URL); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Println("Database migrations completed successfully")

	// Create API server
	srv, err := api.NewServer(cfg, st)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Start server in goroutine
	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("Starting server on %s", cfg.Server.ListenAddr)
		serverErrors <- srv.Start()
	}()

	// Wait for interrupt signal or server error
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)
	case sig := <-shutdown:
		log.Printf("Received signal %v, starting graceful shutdown", sig)

		// Give outstanding requests 30 seconds to complete
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}

		log.Println("Server stopped gracefully")
	}

	return nil
}
