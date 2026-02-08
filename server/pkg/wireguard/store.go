package wireguard

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Peer represents a stored peer record in the database
type Peer struct {
	ID            string
	MachineID     string
	EnvironmentID string
	UserID        string
	PublicKey     string
	AssignedIP    string
	ConnectedAt   time.Time
	LastSeen      time.Time
}

// Store provides database operations for WireGuard peer management
type Store struct {
	db *pgxpool.Pool
}

// NewStore creates a new Store instance
func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// CreatePeer inserts a new peer record into the database
func (s *Store) CreatePeer(ctx context.Context, peer *Peer) error {
	if peer.ID == "" {
		peer.ID = uuid.New().String()
	}

	query := `
		INSERT INTO wireguard_peers (id, machine_id, environment_id, user_id, public_key, assigned_ip, connected_at, last_seen)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := s.db.Exec(ctx, query,
		peer.ID,
		peer.MachineID,
		peer.EnvironmentID,
		peer.UserID,
		peer.PublicKey,
		peer.AssignedIP,
		peer.ConnectedAt,
		peer.LastSeen,
	)
	if err != nil {
		return fmt.Errorf("failed to create peer: %w", err)
	}

	return nil
}

// GetPeer retrieves a peer by public key
func (s *Store) GetPeer(ctx context.Context, publicKey string) (*Peer, error) {
	query := `
		SELECT id, machine_id, environment_id, user_id, public_key, assigned_ip, connected_at, last_seen
		FROM wireguard_peers
		WHERE public_key = $1
	`

	var peer Peer
	err := s.db.QueryRow(ctx, query, publicKey).Scan(
		&peer.ID,
		&peer.MachineID,
		&peer.EnvironmentID,
		&peer.UserID,
		&peer.PublicKey,
		&peer.AssignedIP,
		&peer.ConnectedAt,
		&peer.LastSeen,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("peer not found: %s", publicKey)
		}
		return nil, fmt.Errorf("failed to get peer: %w", err)
	}

	return &peer, nil
}

// GetPeersByMachine retrieves all peers connected to a machine
func (s *Store) GetPeersByMachine(ctx context.Context, machineID string) ([]*Peer, error) {
	query := `
		SELECT id, machine_id, environment_id, user_id, public_key, assigned_ip, connected_at, last_seen
		FROM wireguard_peers
		WHERE machine_id = $1
		ORDER BY connected_at DESC
	`

	rows, err := s.db.Query(ctx, query, machineID)
	if err != nil {
		return nil, fmt.Errorf("failed to query peers: %w", err)
	}
	defer rows.Close()

	var peers []*Peer
	for rows.Next() {
		var peer Peer
		err := rows.Scan(
			&peer.ID,
			&peer.MachineID,
			&peer.EnvironmentID,
			&peer.UserID,
			&peer.PublicKey,
			&peer.AssignedIP,
			&peer.ConnectedAt,
			&peer.LastSeen,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan peer: %w", err)
		}
		peers = append(peers, &peer)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating peers: %w", err)
	}

	return peers, nil
}

// GetPeersByEnvironment retrieves all peers connected to an environment
func (s *Store) GetPeersByEnvironment(ctx context.Context, environmentID string) ([]*Peer, error) {
	query := `
		SELECT id, machine_id, environment_id, user_id, public_key, assigned_ip, connected_at, last_seen
		FROM wireguard_peers
		WHERE environment_id = $1
		ORDER BY connected_at DESC
	`

	rows, err := s.db.Query(ctx, query, environmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query peers: %w", err)
	}
	defer rows.Close()

	var peers []*Peer
	for rows.Next() {
		var peer Peer
		err := rows.Scan(
			&peer.ID,
			&peer.MachineID,
			&peer.EnvironmentID,
			&peer.UserID,
			&peer.PublicKey,
			&peer.AssignedIP,
			&peer.ConnectedAt,
			&peer.LastSeen,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan peer: %w", err)
		}
		peers = append(peers, &peer)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating peers: %w", err)
	}

	return peers, nil
}

// DeletePeer removes a peer from the database by public key
func (s *Store) DeletePeer(ctx context.Context, publicKey string) error {
	query := `DELETE FROM wireguard_peers WHERE public_key = $1`

	result, err := s.db.Exec(ctx, query, publicKey)
	if err != nil {
		return fmt.Errorf("failed to delete peer: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("peer not found: %s", publicKey)
	}

	return nil
}

// UpdateLastSeen updates the last_seen timestamp for a peer
func (s *Store) UpdateLastSeen(ctx context.Context, publicKey string) error {
	query := `
		UPDATE wireguard_peers
		SET last_seen = NOW()
		WHERE public_key = $1
	`

	result, err := s.db.Exec(ctx, query, publicKey)
	if err != nil {
		return fmt.Errorf("failed to update last_seen: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("peer not found: %s", publicKey)
	}

	return nil
}

// GetNextPeerIP allocates the next available IP address for a peer in a machine
// IPs are allocated from 10.225.0.0/16 subnet, starting at 10.225.0.1
func (s *Store) GetNextPeerIP(ctx context.Context, machineID string) (string, error) {
	// Define the peer subnet (10.225.0.0/16)
	_, peerSubnet, err := net.ParseCIDR("10.225.0.0/16")
	if err != nil {
		return "", fmt.Errorf("failed to parse peer subnet: %w", err)
	}

	// Get all existing peer IPs for this machine
	query := `
		SELECT assigned_ip
		FROM wireguard_peers
		WHERE machine_id = $1
		ORDER BY assigned_ip
	`

	rows, err := s.db.Query(ctx, query, machineID)
	if err != nil {
		return "", fmt.Errorf("failed to query existing IPs: %w", err)
	}
	defer rows.Close()

	// Track used IPs
	usedIPs := make(map[string]bool)
	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			return "", fmt.Errorf("failed to scan IP: %w", err)
		}
		usedIPs[ip] = true
	}

	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("error iterating IPs: %w", err)
	}

	// Find the next available IP
	// Start from 10.225.0.1 (skip .0 as network address)
	ip := net.ParseIP("10.225.0.1")
	for {
		if !peerSubnet.Contains(ip) {
			return "", fmt.Errorf("no available IPs in peer subnet")
		}

		ipStr := ip.String()
		if !usedIPs[ipStr] {
			return ipStr, nil
		}

		// Increment IP
		ip = incrementIP(ip)
	}
}

// incrementIP increments an IP address by 1
func incrementIP(ip net.IP) net.IP {
	// Make a copy
	newIP := make(net.IP, len(ip))
	copy(newIP, ip)

	// Increment from right to left
	for i := len(newIP) - 1; i >= 0; i-- {
		newIP[i]++
		if newIP[i] > 0 {
			break
		}
	}

	return newIP
}
