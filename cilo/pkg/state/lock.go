package state

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cilo/cilo/pkg/models"
	"github.com/gofrs/flock"
)

const lockTimeout = 30 * time.Second

// WithLock executes fn with exclusive state lock
func WithLock(fn func(*models.State) error) error {
	statePath := getStatePath()
	lockPath := statePath + ".lock"
	fileLock := flock.New(lockPath)

	ctx, cancel := context.WithTimeout(context.Background(), lockTimeout)
	defer cancel()

	locked, err := fileLock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return fmt.Errorf("failed to acquire state lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("state lock timeout after %v", lockTimeout)
	}
	defer fileLock.Unlock()

	// Load current state (without lock - we already have exclusive lock)
	state, err := loadStateUnsafe()
	if err != nil {
		return err
	}

	// Execute mutation
	if err := fn(state); err != nil {
		return err
	}

	// Save atomically
	return atomicWriteState(state)
}

// loadStateUnsafe loads state without acquiring lock (caller must hold lock)
func loadStateUnsafe() (*models.State, error) {
	path := getStatePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("cilo not initialized (run 'cilo init')")
		}
		return nil, err
	}

	var state models.State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state: %w", err)
	}

	// Initialize map if nil (for backwards compatibility)
	if state.Environments == nil {
		state.Environments = make(map[string]*models.Environment)
	}

	return &state, nil
}

// atomicWriteState writes state atomically using temp file + rename
func atomicWriteState(state *models.State) error {
	path := getStatePath()
	dir := filepath.Dir(path)

	// Create temp file in same directory for atomic rename
	tmpFile, err := os.CreateTemp(dir, ".state-*.json.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Write to temp file
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return err
	}

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write state: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename state file: %w", err)
	}

	return nil
}
