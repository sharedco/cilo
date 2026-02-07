package share

import (
	"context"
	"fmt"
	"time"

	"github.com/sharedco/cilo/pkg/models"
	"github.com/sharedco/cilo/pkg/runtime"
)

// Issue represents a problem detected with a shared service
type Issue struct {
	Service string
	Type    string // "orphaned", "missing", "stale_grace"
	Detail  string
}

// CheckSharedServices validates shared service state and identifies issues
func CheckSharedServices(st *models.State, provider runtime.Provider, ctx context.Context) ([]Issue, error) {
	var issues []Issue

	if st.SharedServices == nil {
		return issues, nil
	}

	// Check each shared service in state
	for key, sharedSvc := range st.SharedServices {
		// Check if container exists
		exists, err := provider.ContainerExists(ctx, sharedSvc.Container)
		if err != nil {
			return nil, fmt.Errorf("failed to check container %s: %w", sharedSvc.Container, err)
		}

		// Issue 1: Orphaned - container exists but no environments are using it
		if exists && len(sharedSvc.UsedBy) == 0 && sharedSvc.DisconnectTimeout.IsZero() {
			issues = append(issues, Issue{
				Service: key,
				Type:    "orphaned",
				Detail:  fmt.Sprintf("Container %s is running but not used by any environment", sharedSvc.Container),
			})
		}

		// Issue 2: Missing - environments reference it but container doesn't exist
		if !exists && len(sharedSvc.UsedBy) > 0 {
			issues = append(issues, Issue{
				Service: key,
				Type:    "missing",
				Detail:  fmt.Sprintf("Container %s missing but referenced by %d environment(s): %v", sharedSvc.Container, len(sharedSvc.UsedBy), sharedSvc.UsedBy),
			})
		}

		// Issue 3: Stale grace period - grace period expired but container still exists
		if exists && len(sharedSvc.UsedBy) == 0 && !sharedSvc.DisconnectTimeout.IsZero() && time.Now().After(sharedSvc.DisconnectTimeout) {
			issues = append(issues, Issue{
				Service: key,
				Type:    "stale_grace",
				Detail:  fmt.Sprintf("Container %s grace period expired %s ago", sharedSvc.Container, time.Since(sharedSvc.DisconnectTimeout).Round(time.Second)),
			})
		}

		// Check if container is running when it should be
		if exists && len(sharedSvc.UsedBy) > 0 {
			status, err := provider.GetContainerStatus(ctx, sharedSvc.Container)
			if err != nil {
				continue // Skip status check on error
			}

			if status != "running" {
				issues = append(issues, Issue{
					Service: key,
					Type:    "stopped",
					Detail:  fmt.Sprintf("Container %s is %s but used by %d environment(s)", sharedSvc.Container, status, len(sharedSvc.UsedBy)),
				})
			}
		}
	}

	return issues, nil
}

// FixOrphanedServices stops and removes orphaned shared service containers
func FixOrphanedServices(st *models.State, provider runtime.Provider, ctx context.Context) (int, error) {
	fixed := 0

	for key, sharedSvc := range st.SharedServices {
		// Only fix truly orphaned services (no references and no grace period)
		if len(sharedSvc.UsedBy) == 0 && sharedSvc.DisconnectTimeout.IsZero() {
			exists, err := provider.ContainerExists(ctx, sharedSvc.Container)
			if err != nil {
				continue
			}

			if exists {
				// Stop container
				if err := provider.StopContainer(ctx, sharedSvc.Container); err != nil {
					fmt.Printf("Warning: failed to stop %s: %v\n", sharedSvc.Container, err)
					continue
				}

				// Remove container
				if err := provider.RemoveContainer(ctx, sharedSvc.Container); err != nil {
					fmt.Printf("Warning: failed to remove %s: %v\n", sharedSvc.Container, err)
					continue
				}

				// Remove from state
				delete(st.SharedServices, key)
				fixed++
			}
		}
	}

	return fixed, nil
}

// FixStaleGracePeriods cleans up services past their grace period
func FixStaleGracePeriods(st *models.State, provider runtime.Provider, ctx context.Context) (int, error) {
	fixed := 0

	for key, sharedSvc := range st.SharedServices {
		// Only fix services with expired grace periods
		if len(sharedSvc.UsedBy) == 0 && !sharedSvc.DisconnectTimeout.IsZero() && time.Now().After(sharedSvc.DisconnectTimeout) {
			exists, err := provider.ContainerExists(ctx, sharedSvc.Container)
			if err != nil {
				continue
			}

			if exists {
				// Stop container
				if err := provider.StopContainer(ctx, sharedSvc.Container); err != nil {
					fmt.Printf("Warning: failed to stop %s: %v\n", sharedSvc.Container, err)
					continue
				}

				// Remove container
				if err := provider.RemoveContainer(ctx, sharedSvc.Container); err != nil {
					fmt.Printf("Warning: failed to remove %s: %v\n", sharedSvc.Container, err)
					continue
				}
			}

			// Remove from state regardless of whether container existed
			delete(st.SharedServices, key)
			fixed++
		}
	}

	return fixed, nil
}

// FixMissingServices removes state entries for missing containers that are referenced
func FixMissingServices(st *models.State, provider runtime.Provider, ctx context.Context) (int, error) {
	fixed := 0

	for key, sharedSvc := range st.SharedServices {
		if len(sharedSvc.UsedBy) > 0 {
			exists, err := provider.ContainerExists(ctx, sharedSvc.Container)
			if err != nil {
				continue
			}

			// If container doesn't exist but is referenced, remove from state
			// The environments will need to recreate it when they come up
			if !exists {
				delete(st.SharedServices, key)
				fixed++
			}
		}
	}

	return fixed, nil
}

