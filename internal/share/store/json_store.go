// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"github.com/sharedco/cilo/internal/models"
)

const jsonStoreLockTimeout = 30 * time.Second

type sharedServicesFile struct {
	SharedServices map[string]*models.SharedService `json:"shared_services"`
}

type JSONSharedServiceStore struct {
	mu       sync.Mutex
	filePath string
	lockPath string
}

func NewJSONSharedServiceStore(filePath string) (*JSONSharedServiceStore, error) {
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create shared services directory: %w", err)
	}

	store := &JSONSharedServiceStore{
		filePath: filePath,
		lockPath: filePath + ".lock",
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		if err := store.writeUnlocked(&sharedServicesFile{SharedServices: map[string]*models.SharedService{}}); err != nil {
			return nil, err
		}
	}

	return store, nil
}

func (s *JSONSharedServiceStore) Get(ctx context.Context, key string) (*models.SharedService, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fileData, err := s.withFileLock(ctx, func() (*sharedServicesFile, error) {
		return s.readUnlocked()
	})
	if err != nil {
		return nil, false, err
	}

	svc, ok := fileData.SharedServices[key]
	if !ok || svc == nil {
		return nil, false, nil
	}

	return CloneSharedService(svc), true, nil
}

func (s *JSONSharedServiceStore) Update(ctx context.Context, key string, fn func(existing *models.SharedService, exists bool) (next *models.SharedService, nextExists bool, err error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.withFileLock(ctx, func() (*sharedServicesFile, error) {
		fileData, err := s.readUnlocked()
		if err != nil {
			return nil, err
		}

		existing, exists := fileData.SharedServices[key]
		next, nextExists, err := fn(CloneSharedService(existing), exists)
		if err != nil {
			return nil, err
		}

		if !nextExists || next == nil {
			delete(fileData.SharedServices, key)
		} else {
			fileData.SharedServices[key] = CloneSharedService(next)
		}

		if err := s.writeUnlocked(fileData); err != nil {
			return nil, err
		}

		return fileData, nil
	})
	return err
}

func (s *JSONSharedServiceStore) Delete(ctx context.Context, key string) error {
	return s.Update(ctx, key, func(_ *models.SharedService, _ bool) (*models.SharedService, bool, error) {
		return nil, false, nil
	})
}

func (s *JSONSharedServiceStore) withFileLock(ctx context.Context, fn func() (*sharedServicesFile, error)) (*sharedServicesFile, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	lockCtx, cancel := context.WithTimeout(ctx, jsonStoreLockTimeout)
	defer cancel()

	fileLock := flock.New(s.lockPath)
	locked, err := fileLock.TryLockContext(lockCtx, 100*time.Millisecond)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire shared services lock: %w", err)
	}
	if !locked {
		return nil, fmt.Errorf("shared services lock timeout after %v", jsonStoreLockTimeout)
	}
	defer fileLock.Unlock()

	return fn()
}

func (s *JSONSharedServiceStore) readUnlocked() (*sharedServicesFile, error) {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &sharedServicesFile{SharedServices: map[string]*models.SharedService{}}, nil
		}
		return nil, fmt.Errorf("failed to read shared services file: %w", err)
	}

	var fileData sharedServicesFile
	if err := json.Unmarshal(data, &fileData); err != nil {
		return nil, fmt.Errorf("failed to parse shared services file: %w", err)
	}
	if fileData.SharedServices == nil {
		fileData.SharedServices = make(map[string]*models.SharedService)
	}

	return &fileData, nil
}

func (s *JSONSharedServiceStore) writeUnlocked(data *sharedServicesFile) error {
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal shared services file: %w", err)
	}

	tmpFile := s.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, encoded, 0644); err != nil {
		return fmt.Errorf("failed to write temp shared services file: %w", err)
	}

	if err := os.Rename(tmpFile, s.filePath); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to replace shared services file: %w", err)
	}

	return nil
}
