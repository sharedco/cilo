// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: BUSL-1.1
// See LICENSES/BUSL-1.1.txt and LICENSE.enterprise for full license text

package vm

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// PoolConfig contains pool configuration
type PoolConfig struct {
	MinReady int
	MaxTotal int
	VMSize   string
	Region   string
	ImageID  string
}

// Pool manages a pool of VMs
type Pool struct {
	provider Provider
	config   PoolConfig
	store    PoolStore
	mu       sync.RWMutex
}

// PoolStore interface for persisting pool state
type PoolStore interface {
	SaveMachine(ctx context.Context, machine *Machine) error
	GetMachine(ctx context.Context, id string) (*Machine, error)
	ListMachines(ctx context.Context) ([]*Machine, error)
	UpdateMachineStatus(ctx context.Context, id, status string) error
	AssignMachine(ctx context.Context, id, envID string) error
	ReleaseMachine(ctx context.Context, id string) error
	DeleteMachine(ctx context.Context, id string) error
}

// NewPool creates a new VM pool
func NewPool(provider Provider, config PoolConfig, store PoolStore) *Pool {
	return &Pool{
		provider: provider,
		config:   config,
		store:    store,
	}
}

// AssignMachine assigns a ready machine to an environment
func (p *Pool) AssignMachine(ctx context.Context, envID string) (*Machine, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	machines, err := p.store.ListMachines(ctx)
	if err != nil {
		return nil, fmt.Errorf("list machines: %w", err)
	}

	// Find a ready machine
	for _, m := range machines {
		if m.Status == MachineStatusReady && m.AssignedEnv == "" {
			// Assign the machine
			if err := p.store.AssignMachine(ctx, m.ID, envID); err != nil {
				return nil, fmt.Errorf("assign machine: %w", err)
			}
			m.Status = MachineStatusAssigned
			m.AssignedEnv = envID
			return m, nil
		}
	}

	// No ready machines, provision a new one
	machine, err := p.provisionMachine(ctx)
	if err != nil {
		return nil, fmt.Errorf("provision machine: %w", err)
	}

	// Assign immediately
	if err := p.store.AssignMachine(ctx, machine.ID, envID); err != nil {
		return nil, fmt.Errorf("assign new machine: %w", err)
	}
	machine.Status = MachineStatusAssigned
	machine.AssignedEnv = envID

	return machine, nil
}

// ReleaseMachine releases a machine back to the pool
func (p *Pool) ReleaseMachine(ctx context.Context, machineID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	machine, err := p.store.GetMachine(ctx, machineID)
	if err != nil {
		return fmt.Errorf("get machine: %w", err)
	}

	// Check if we have too many machines
	machines, err := p.store.ListMachines(ctx)
	if err != nil {
		return fmt.Errorf("list machines: %w", err)
	}

	if len(machines) > p.config.MaxTotal {
		// Destroy excess machine
		return p.destroyMachine(ctx, machine)
	}

	// Release back to pool
	if err := p.store.ReleaseMachine(ctx, machineID); err != nil {
		return fmt.Errorf("release machine: %w", err)
	}

	return nil
}

// Reconcile ensures the pool has the right number of ready machines
func (p *Pool) Reconcile(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	machines, err := p.store.ListMachines(ctx)
	if err != nil {
		return fmt.Errorf("list machines: %w", err)
	}

	// Count ready machines
	readyCount := 0
	for _, m := range machines {
		if m.Status == MachineStatusReady {
			readyCount++
		}
	}

	// Provision more if needed
	for readyCount < p.config.MinReady && len(machines) < p.config.MaxTotal {
		if _, err := p.provisionMachine(ctx); err != nil {
			log.Printf("Failed to provision machine: %v", err)
			break
		}
		readyCount++
	}

	return nil
}

// StartReconcileLoop starts a background reconciliation loop
func (p *Pool) StartReconcileLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				if err := p.Reconcile(ctx); err != nil {
					log.Printf("Pool reconcile error: %v", err)
				}
			}
		}
	}()
}

// provisionMachine provisions a new machine (must be called with lock held)
func (p *Pool) provisionMachine(ctx context.Context) (*Machine, error) {
	config := ProvisionConfig{
		Name:    fmt.Sprintf("cilo-vm-%s", uuid.New().String()[:8]),
		Size:    p.config.VMSize,
		Region:  p.config.Region,
		ImageID: p.config.ImageID,
		Labels: map[string]string{
			"pool": "cilo-workers",
		},
	}

	machine, err := p.provider.Provision(ctx, config)
	if err != nil {
		return nil, err
	}

	// Save to store
	if err := p.store.SaveMachine(ctx, machine); err != nil {
		// Try to destroy the machine
		p.provider.Destroy(ctx, machine.ProviderID)
		return nil, fmt.Errorf("save machine: %w", err)
	}

	return machine, nil
}

// destroyMachine destroys a machine (must be called with lock held)
func (p *Pool) destroyMachine(ctx context.Context, machine *Machine) error {
	// Update status
	if err := p.store.UpdateMachineStatus(ctx, machine.ID, MachineStatusDestroying); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	// Destroy via provider
	if err := p.provider.Destroy(ctx, machine.ProviderID); err != nil {
		return fmt.Errorf("destroy machine: %w", err)
	}

	// Remove from store
	if err := p.store.DeleteMachine(ctx, machine.ID); err != nil {
		return fmt.Errorf("delete machine: %w", err)
	}

	return nil
}

// HealthCheckAll checks health of all machines
func (p *Pool) HealthCheckAll(ctx context.Context) error {
	machines, err := p.store.ListMachines(ctx)
	if err != nil {
		return fmt.Errorf("list machines: %w", err)
	}

	for _, machine := range machines {
		healthy, err := p.provider.HealthCheck(ctx, machine.ProviderID)
		if err != nil {
			log.Printf("Health check failed for %s: %v", machine.ID, err)
			continue
		}

		if !healthy && machine.Status == MachineStatusReady {
			log.Printf("Machine %s is unhealthy, marking as failed", machine.ID)
			p.store.UpdateMachineStatus(ctx, machine.ID, MachineStatusFailed)
		}
	}

	return nil
}
