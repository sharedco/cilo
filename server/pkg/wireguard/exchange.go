package wireguard

import (
	"context"
	"fmt"
	"net"
	"time"
)

// PeerRegistration represents a peer wanting to connect to a machine
type PeerRegistration struct {
	EnvironmentID string
	UserID        string
	PublicKey     string
	AssignedIP    string // Unique per peer: 10.225.0.1, 10.225.0.2, etc.
}

// PeerConfig is returned to the client for WireGuard setup
type PeerConfig struct {
	MachinePublicKey string   // Machine's WG public key
	MachineEndpoint  string   // Machine's public IP:port
	AssignedIP       string   // Client's assigned IP in WG network
	AllowedIPs       []string // Routes to send through tunnel
}

// MachineInfo represents machine details needed for peer registration
type MachineInfo struct {
	ID                string
	PublicKey         string
	Endpoint          string
	EnvironmentSubnet string // e.g., "10.224.1.0/24" for environment services
}

// Exchange handles multi-peer WireGuard key exchange
type Exchange struct {
	store      *Store
	peerSubnet *net.IPNet // 10.225.0.0/16 for peer IPs
}

// NewExchange creates a new Exchange instance
func NewExchange(store *Store) *Exchange {
	// Parse the peer subnet (10.225.0.0/16)
	_, peerSubnet, err := net.ParseCIDR("10.225.0.0/16")
	if err != nil {
		// This should never fail with a valid CIDR
		panic(fmt.Sprintf("invalid peer subnet: %v", err))
	}

	return &Exchange{
		store:      store,
		peerSubnet: peerSubnet,
	}
}

// RegisterPeer adds a new peer to a machine's WireGuard configuration
// Returns the machine's WG public key and endpoint for the client to connect
func (e *Exchange) RegisterPeer(ctx context.Context, machineInfo MachineInfo, peer PeerRegistration) (*PeerConfig, error) {
	// Validate peer public key
	if peer.PublicKey == "" {
		return nil, fmt.Errorf("peer public key is required")
	}

	// Allocate IP if not provided
	assignedIP := peer.AssignedIP
	if assignedIP == "" {
		ip, err := e.AllocatePeerIP(ctx, machineInfo.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to allocate peer IP: %w", err)
		}
		assignedIP = ip
	}

	// Validate assigned IP is in peer subnet
	if !e.peerSubnet.Contains(net.ParseIP(assignedIP)) {
		return nil, fmt.Errorf("assigned IP %s is not in peer subnet %s", assignedIP, e.peerSubnet.String())
	}

	// Check if peer already exists
	existingPeer, err := e.store.GetPeer(ctx, peer.PublicKey)
	if err == nil {
		// Peer exists, update last_seen
		if err := e.store.UpdateLastSeen(ctx, peer.PublicKey); err != nil {
			return nil, fmt.Errorf("failed to update peer last_seen: %w", err)
		}

		// Return existing configuration
		return e.buildPeerConfig(machineInfo, existingPeer.AssignedIP), nil
	}

	// Create new peer record
	now := time.Now()
	newPeer := &Peer{
		MachineID:     machineInfo.ID,
		EnvironmentID: peer.EnvironmentID,
		UserID:        peer.UserID,
		PublicKey:     peer.PublicKey,
		AssignedIP:    assignedIP,
		ConnectedAt:   now,
		LastSeen:      now,
	}

	if err := e.store.CreatePeer(ctx, newPeer); err != nil {
		return nil, fmt.Errorf("failed to create peer: %w", err)
	}

	// Return peer configuration
	return e.buildPeerConfig(machineInfo, assignedIP), nil
}

// RemovePeer removes a peer from a machine's WireGuard configuration
func (e *Exchange) RemovePeer(ctx context.Context, machineID string, publicKey string) error {
	// Verify peer belongs to this machine
	peer, err := e.store.GetPeer(ctx, publicKey)
	if err != nil {
		return fmt.Errorf("peer not found: %w", err)
	}

	if peer.MachineID != machineID {
		return fmt.Errorf("peer does not belong to machine %s", machineID)
	}

	// Delete peer
	if err := e.store.DeletePeer(ctx, publicKey); err != nil {
		return fmt.Errorf("failed to delete peer: %w", err)
	}

	return nil
}

// GetPeersForMachine returns all peers connected to a machine
func (e *Exchange) GetPeersForMachine(ctx context.Context, machineID string) ([]PeerRegistration, error) {
	peers, err := e.store.GetPeersByMachine(ctx, machineID)
	if err != nil {
		return nil, fmt.Errorf("failed to get peers: %w", err)
	}

	registrations := make([]PeerRegistration, len(peers))
	for i, peer := range peers {
		registrations[i] = PeerRegistration{
			EnvironmentID: peer.EnvironmentID,
			UserID:        peer.UserID,
			PublicKey:     peer.PublicKey,
			AssignedIP:    peer.AssignedIP,
		}
	}

	return registrations, nil
}

// GetPeersForEnvironment returns all peers connected to an environment
func (e *Exchange) GetPeersForEnvironment(ctx context.Context, environmentID string) ([]PeerRegistration, error) {
	peers, err := e.store.GetPeersByEnvironment(ctx, environmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get peers: %w", err)
	}

	registrations := make([]PeerRegistration, len(peers))
	for i, peer := range peers {
		registrations[i] = PeerRegistration{
			EnvironmentID: peer.EnvironmentID,
			UserID:        peer.UserID,
			PublicKey:     peer.PublicKey,
			AssignedIP:    peer.AssignedIP,
		}
	}

	return registrations, nil
}

// AllocatePeerIP assigns a unique IP to a new peer from the peer subnet
func (e *Exchange) AllocatePeerIP(ctx context.Context, machineID string) (string, error) {
	return e.store.GetNextPeerIP(ctx, machineID)
}

// buildPeerConfig constructs the peer configuration response
func (e *Exchange) buildPeerConfig(machineInfo MachineInfo, assignedIP string) *PeerConfig {
	// AllowedIPs determines which traffic goes through the WireGuard tunnel
	// Include:
	// - The peer's own IP (for heartbeat/keepalive)
	// - The environment's service subnet (e.g., 10.224.1.0/24)
	// - The peer subnet (10.225.0.0/16) for peer-to-peer communication
	allowedIPs := []string{
		assignedIP + "/32",            // Peer's own IP
		machineInfo.EnvironmentSubnet, // Environment services
		e.peerSubnet.String(),         // All peers
	}

	return &PeerConfig{
		MachinePublicKey: machineInfo.PublicKey,
		MachineEndpoint:  machineInfo.Endpoint,
		AssignedIP:       assignedIP,
		AllowedIPs:       allowedIPs,
	}
}

// ValidatePeerSubnet checks if an IP is within the peer subnet
func (e *Exchange) ValidatePeerSubnet(ip string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}
	return e.peerSubnet.Contains(parsedIP)
}
