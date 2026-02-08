package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store handles auth-related database operations
type Store struct {
	pool *pgxpool.Pool
}

// NewStore creates a new auth store
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Team represents a team/organization
type Team struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

// APIKey represents an API key record
type APIKey struct {
	ID        string
	TeamID    string
	KeyHash   string
	Prefix    string
	Scope     string
	Name      string
	CreatedAt time.Time
	LastUsed  *time.Time
}

// CreateTeam creates a new team
func (s *Store) CreateTeam(ctx context.Context, name string) (*Team, error) {
	team := &Team{
		ID:        uuid.New().String(),
		Name:      name,
		CreatedAt: time.Now(),
	}

	_, err := s.pool.Exec(ctx,
		`INSERT INTO teams (id, name, created_at) VALUES ($1, $2, $3)`,
		team.ID, team.Name, team.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert team: %w", err)
	}

	return team, nil
}

// GetTeam retrieves a team by ID
func (s *Store) GetTeam(ctx context.Context, id string) (*Team, error) {
	team := &Team{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, created_at FROM teams WHERE id = $1`,
		id,
	).Scan(&team.ID, &team.Name, &team.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get team: %w", err)
	}
	return team, nil
}

// CreateAPIKey creates a new API key
func (s *Store) CreateAPIKey(ctx context.Context, teamID, name, scope, keyHash, prefix string) (*APIKey, error) {
	key := &APIKey{
		ID:        uuid.New().String(),
		TeamID:    teamID,
		KeyHash:   keyHash,
		Prefix:    prefix,
		Scope:     scope,
		Name:      name,
		CreatedAt: time.Now(),
	}

	_, err := s.pool.Exec(ctx,
		`INSERT INTO api_keys (id, team_id, key_hash, prefix, scope, name, created_at) 
         VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		key.ID, key.TeamID, key.KeyHash, key.Prefix, key.Scope, key.Name, key.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert api key: %w", err)
	}

	return key, nil
}

// GetAPIKeyByPrefix retrieves an API key by its prefix
func (s *Store) GetAPIKeyByPrefix(ctx context.Context, prefix string) (*APIKey, error) {
	key := &APIKey{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, team_id, key_hash, prefix, scope, name, created_at, last_used 
         FROM api_keys WHERE prefix = $1`,
		prefix,
	).Scan(&key.ID, &key.TeamID, &key.KeyHash, &key.Prefix, &key.Scope, &key.Name, &key.CreatedAt, &key.LastUsed)
	if err != nil {
		return nil, fmt.Errorf("get api key: %w", err)
	}
	return key, nil
}

// ListAPIKeys lists all API keys for a team
func (s *Store) ListAPIKeys(ctx context.Context, teamID string) ([]*APIKey, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, team_id, prefix, scope, name, created_at, last_used 
         FROM api_keys WHERE team_id = $1 ORDER BY created_at DESC`,
		teamID,
	)
	if err != nil {
		return nil, fmt.Errorf("query api keys: %w", err)
	}
	defer rows.Close()

	var keys []*APIKey
	for rows.Next() {
		key := &APIKey{}
		if err := rows.Scan(&key.ID, &key.TeamID, &key.Prefix, &key.Scope, &key.Name, &key.CreatedAt, &key.LastUsed); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		keys = append(keys, key)
	}

	return keys, nil
}

// DeleteAPIKey deletes an API key
func (s *Store) DeleteAPIKey(ctx context.Context, teamID, keyID string) error {
	result, err := s.pool.Exec(ctx,
		`DELETE FROM api_keys WHERE id = $1 AND team_id = $2`,
		keyID, teamID,
	)
	if err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("api key not found")
	}
	return nil
}

// UpdateLastUsed updates the last_used timestamp for a key
func (s *Store) UpdateLastUsed(ctx context.Context, keyID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE api_keys SET last_used = $1 WHERE id = $2`,
		time.Now(), keyID,
	)
	return err
}
