// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package store

import (
	"context"
	"fmt"

	"github.com/sharedco/cilo/internal/models"
)

type SharedServiceStore interface {
	Get(ctx context.Context, key string) (*models.SharedService, bool, error)
	Update(ctx context.Context, key string, fn func(existing *models.SharedService, exists bool) (next *models.SharedService, nextExists bool, err error)) error
	Delete(ctx context.Context, key string) error
}

func Key(project, serviceName string) string {
	return fmt.Sprintf("%s/%s", project, serviceName)
}

func CloneSharedService(svc *models.SharedService) *models.SharedService {
	if svc == nil {
		return nil
	}

	copySvc := *svc
	if svc.UsedBy != nil {
		copySvc.UsedBy = append([]string(nil), svc.UsedBy...)
	}
	return &copySvc
}
