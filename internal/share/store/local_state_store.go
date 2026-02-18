// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package store

import (
	"context"

	"github.com/sharedco/cilo/internal/models"
	"github.com/sharedco/cilo/internal/state"
)

type LocalStateStore struct{}

func NewLocalStateStore() *LocalStateStore {
	return &LocalStateStore{}
}

func (s *LocalStateStore) Get(_ context.Context, key string) (*models.SharedService, bool, error) {
	st, err := state.LoadState()
	if err != nil {
		return nil, false, err
	}
	if st.SharedServices == nil {
		return nil, false, nil
	}
	svc, ok := st.SharedServices[key]
	if !ok || svc == nil {
		return nil, false, nil
	}
	return CloneSharedService(svc), true, nil
}

func (s *LocalStateStore) Update(_ context.Context, key string, fn func(existing *models.SharedService, exists bool) (next *models.SharedService, nextExists bool, err error)) error {
	return state.WithLock(func(st *models.State) error {
		if st.SharedServices == nil {
			st.SharedServices = make(map[string]*models.SharedService)
		}

		existing, exists := st.SharedServices[key]
		next, nextExists, err := fn(CloneSharedService(existing), exists)
		if err != nil {
			return err
		}

		if !nextExists || next == nil {
			delete(st.SharedServices, key)
			return nil
		}

		st.SharedServices[key] = CloneSharedService(next)
		return nil
	})
}

func (s *LocalStateStore) Delete(ctx context.Context, key string) error {
	return s.Update(ctx, key, func(_ *models.SharedService, _ bool) (*models.SharedService, bool, error) {
		return nil, false, nil
	})
}
