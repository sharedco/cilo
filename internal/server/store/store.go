// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: BUSL-1.1
// See LICENSES/BUSL-1.1.txt and LICENSE.enterprise for full license text

package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store provides database access
type Store struct {
	pool *pgxpool.Pool
}

// Connect creates a new database connection pool
func Connect(databaseURL string) (*Store, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Configure pool settings
	config.MaxConns = 25
	config.MinConns = 5

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Store{pool: pool}, nil
}

// Close closes the database connection pool
func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

// Pool returns the underlying connection pool for direct queries
func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}

// CreateEnvironment creates a new environment
func (s *Store) CreateEnvironment(ctx context.Context, env *Environment) error {
	servicesJSON, _ := json.Marshal(env.Services)
	peersJSON, _ := json.Marshal(env.Peers)

	_, err := s.pool.Exec(ctx, `
		INSERT INTO environments (id, team_id, name, project, format, machine_id, status, subnet, services, peers, created_by, source)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, env.ID, env.TeamID, env.Name, env.Project, env.Format, env.MachineID, env.Status, env.Subnet, servicesJSON, peersJSON, env.CreatedBy, env.Source)
	return err
}

// GetEnvironment retrieves an environment by ID
func (s *Store) GetEnvironment(ctx context.Context, id string) (*Environment, error) {
	var env Environment
	var servicesJSON, peersJSON []byte

	err := s.pool.QueryRow(ctx, `
		SELECT id, team_id, name, project, format, machine_id, status, subnet, services, peers, created_at, created_by, source
		FROM environments
		WHERE id = $1
	`, id).Scan(&env.ID, &env.TeamID, &env.Name, &env.Project, &env.Format, &env.MachineID, &env.Status, &env.Subnet, &servicesJSON, &peersJSON, &env.CreatedAt, &env.CreatedBy, &env.Source)
	if err != nil {
		return nil, err
	}

	if len(servicesJSON) > 0 {
		json.Unmarshal(servicesJSON, &env.Services)
	}
	if len(peersJSON) > 0 {
		json.Unmarshal(peersJSON, &env.Peers)
	}

	return &env, nil
}

// GetEnvironmentByName retrieves an environment by team ID and name
func (s *Store) GetEnvironmentByName(ctx context.Context, teamID, name string) (*Environment, error) {
	var env Environment
	var servicesJSON, peersJSON []byte

	err := s.pool.QueryRow(ctx, `
		SELECT id, team_id, name, project, format, machine_id, status, subnet, services, peers, created_at, created_by, source
		FROM environments
		WHERE team_id = $1 AND name = $2
	`, teamID, name).Scan(&env.ID, &env.TeamID, &env.Name, &env.Project, &env.Format, &env.MachineID, &env.Status, &env.Subnet, &servicesJSON, &peersJSON, &env.CreatedAt, &env.CreatedBy, &env.Source)
	if err != nil {
		return nil, err
	}

	if len(servicesJSON) > 0 {
		json.Unmarshal(servicesJSON, &env.Services)
	}
	if len(peersJSON) > 0 {
		json.Unmarshal(peersJSON, &env.Peers)
	}

	return &env, nil
}

// ListEnvironments retrieves all environments for a team
func (s *Store) ListEnvironments(ctx context.Context, teamID string) ([]*Environment, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, team_id, name, project, format, machine_id, status, subnet, services, peers, created_at, created_by, source
		FROM environments
		WHERE team_id = $1
		ORDER BY created_at DESC
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var environments []*Environment
	for rows.Next() {
		var env Environment
		var servicesJSON, peersJSON []byte

		err := rows.Scan(&env.ID, &env.TeamID, &env.Name, &env.Project, &env.Format, &env.MachineID, &env.Status, &env.Subnet, &servicesJSON, &peersJSON, &env.CreatedAt, &env.CreatedBy, &env.Source)
		if err != nil {
			return nil, err
		}

		if len(servicesJSON) > 0 {
			json.Unmarshal(servicesJSON, &env.Services)
		}
		if len(peersJSON) > 0 {
			json.Unmarshal(peersJSON, &env.Peers)
		}

		environments = append(environments, &env)
	}

	return environments, rows.Err()
}

// UpdateEnvironmentStatus updates the status of an environment
func (s *Store) UpdateEnvironmentStatus(ctx context.Context, id, status string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE environments
		SET status = $2
		WHERE id = $1
	`, id, status)
	return err
}

// UpdateEnvironmentServices updates the services of an environment
func (s *Store) UpdateEnvironmentServices(ctx context.Context, id string, services []EnvironmentService) error {
	servicesJSON, _ := json.Marshal(services)
	_, err := s.pool.Exec(ctx, `
		UPDATE environments
		SET services = $2
		WHERE id = $1
	`, id, servicesJSON)
	return err
}

// DeleteEnvironment deletes an environment by ID
func (s *Store) DeleteEnvironment(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM environments
		WHERE id = $1
	`, id)
	return err
}

// SaveMachine creates or updates a machine
func (s *Store) SaveMachine(ctx context.Context, machine *Machine) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO machines (id, provider_id, provider_type, public_ip, wg_public_key, wg_endpoint, status, assigned_env, ssh_host, ssh_user, region, size)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (id) DO UPDATE SET
			provider_id = EXCLUDED.provider_id,
			provider_type = EXCLUDED.provider_type,
			public_ip = EXCLUDED.public_ip,
			wg_public_key = EXCLUDED.wg_public_key,
			wg_endpoint = EXCLUDED.wg_endpoint,
			status = EXCLUDED.status,
			assigned_env = EXCLUDED.assigned_env,
			ssh_host = EXCLUDED.ssh_host,
			ssh_user = EXCLUDED.ssh_user,
			region = EXCLUDED.region,
			size = EXCLUDED.size
	`, machine.ID, machine.ProviderID, machine.ProviderType, machine.PublicIP, machine.WGPublicKey, machine.WGEndpoint, machine.Status, machine.AssignedEnv, machine.SSHHost, machine.SSHUser, machine.Region, machine.Size)
	return err
}

// GetMachine retrieves a machine by ID
func (s *Store) GetMachine(ctx context.Context, id string) (*Machine, error) {
	var machine Machine
	err := s.pool.QueryRow(ctx, `
		SELECT id, provider_id, provider_type, public_ip, wg_public_key, wg_endpoint, status, assigned_env, ssh_host, ssh_user, region, size, created_at
		FROM machines
		WHERE id = $1
	`, id).Scan(&machine.ID, &machine.ProviderID, &machine.ProviderType, &machine.PublicIP, &machine.WGPublicKey, &machine.WGEndpoint, &machine.Status, &machine.AssignedEnv, &machine.SSHHost, &machine.SSHUser, &machine.Region, &machine.Size, &machine.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &machine, nil
}

// ListMachines retrieves all machines
func (s *Store) ListMachines(ctx context.Context) ([]*Machine, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, provider_id, provider_type, public_ip, wg_public_key, wg_endpoint, status, assigned_env, ssh_host, ssh_user, region, size, created_at
		FROM machines
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var machines []*Machine
	for rows.Next() {
		var machine Machine
		err := rows.Scan(&machine.ID, &machine.ProviderID, &machine.ProviderType, &machine.PublicIP, &machine.WGPublicKey, &machine.WGEndpoint, &machine.Status, &machine.AssignedEnv, &machine.SSHHost, &machine.SSHUser, &machine.Region, &machine.Size, &machine.CreatedAt)
		if err != nil {
			return nil, err
		}
		machines = append(machines, &machine)
	}

	return machines, rows.Err()
}

// ListMachinesByStatus retrieves machines filtered by status
func (s *Store) ListMachinesByStatus(ctx context.Context, status string) ([]*Machine, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, provider_id, provider_type, public_ip, wg_public_key, wg_endpoint, status, assigned_env, ssh_host, ssh_user, region, size, created_at
		FROM machines
		WHERE status = $1
		ORDER BY created_at DESC
	`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var machines []*Machine
	for rows.Next() {
		var machine Machine
		err := rows.Scan(&machine.ID, &machine.ProviderID, &machine.ProviderType, &machine.PublicIP, &machine.WGPublicKey, &machine.WGEndpoint, &machine.Status, &machine.AssignedEnv, &machine.SSHHost, &machine.SSHUser, &machine.Region, &machine.Size, &machine.CreatedAt)
		if err != nil {
			return nil, err
		}
		machines = append(machines, &machine)
	}

	return machines, rows.Err()
}

// UpdateMachineStatus updates the status of a machine
func (s *Store) UpdateMachineStatus(ctx context.Context, id, status string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE machines
		SET status = $2
		WHERE id = $1
	`, id, status)
	return err
}

// UpdateMachineWireGuardKey updates the WireGuard public key of a machine
func (s *Store) UpdateMachineWireGuardKey(ctx context.Context, id, publicKey string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE machines
		SET wg_public_key = $2
		WHERE id = $1
	`, id, publicKey)
	return err
}

// AssignMachine assigns a machine to an environment
func (s *Store) AssignMachine(ctx context.Context, machineID, envID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE machines
		SET assigned_env = $2
		WHERE id = $1
	`, machineID, envID)
	return err
}

// ReleaseMachine removes the environment assignment from a machine
func (s *Store) ReleaseMachine(ctx context.Context, machineID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE machines
		SET assigned_env = NULL
		WHERE id = $1
	`, machineID)
	return err
}

// DeleteMachine deletes a machine by ID
func (s *Store) DeleteMachine(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM machines
		WHERE id = $1
	`, id)
	return err
}

func (s *Store) CountEnvironmentsByMachineID(ctx context.Context, machineID string) (int, error) {
	count := 0
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(1)
		FROM environments
		WHERE machine_id = $1
	`, machineID).Scan(&count)
	return count, err
}

// CreateAPIKey creates a new API key
func (s *Store) CreateAPIKey(ctx context.Context, key *APIKey) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO api_keys (id, team_id, key_hash, prefix, scope, name, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, key.ID, key.TeamID, key.KeyHash, key.Prefix, key.Scope, key.Name, key.CreatedAt)
	return err
}
