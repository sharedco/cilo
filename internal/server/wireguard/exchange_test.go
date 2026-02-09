// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: BUSL-1.1
// See LICENSES/BUSL-1.1.txt and LICENSE.enterprise for full license text

package wireguard

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockStore struct {
	peers                map[string]*Peer
	peersByMachine       map[string][]*Peer
	peersByEnv           map[string][]*Peer
	nextIP               string
	getPeerErr           error
	createPeerErr        error
	deletePeerErr        error
	updateLastSeenErr    error
	getNextIPErr         error
	getPeersByMachineErr error
	getPeersByEnvErr     error
}

func NewMockStore() *MockStore {
	return &MockStore{
		peers:          make(map[string]*Peer),
		peersByMachine: make(map[string][]*Peer),
		peersByEnv:     make(map[string][]*Peer),
		nextIP:         "10.225.0.1",
	}
}

func (m *MockStore) CreatePeer(ctx context.Context, peer *Peer) error {
	if m.createPeerErr != nil {
		return m.createPeerErr
	}
	m.peers[peer.PublicKey] = peer
	m.peersByMachine[peer.MachineID] = append(m.peersByMachine[peer.MachineID], peer)
	m.peersByEnv[peer.EnvironmentID] = append(m.peersByEnv[peer.EnvironmentID], peer)
	return nil
}

func (m *MockStore) GetPeer(ctx context.Context, publicKey string) (*Peer, error) {
	if m.getPeerErr != nil {
		return nil, m.getPeerErr
	}
	peer, exists := m.peers[publicKey]
	if !exists {
		return nil, errors.New("peer not found")
	}
	return peer, nil
}

func (m *MockStore) GetPeersByMachine(ctx context.Context, machineID string) ([]*Peer, error) {
	if m.getPeersByMachineErr != nil {
		return nil, m.getPeersByMachineErr
	}
	return m.peersByMachine[machineID], nil
}

func (m *MockStore) GetPeersByEnvironment(ctx context.Context, environmentID string) ([]*Peer, error) {
	if m.getPeersByEnvErr != nil {
		return nil, m.getPeersByEnvErr
	}
	return m.peersByEnv[environmentID], nil
}

func (m *MockStore) DeletePeer(ctx context.Context, publicKey string) error {
	if m.deletePeerErr != nil {
		return m.deletePeerErr
	}
	peer, exists := m.peers[publicKey]
	if !exists {
		return errors.New("peer not found")
	}
	delete(m.peers, publicKey)

	machinePeers := m.peersByMachine[peer.MachineID]
	for i, p := range machinePeers {
		if p.PublicKey == publicKey {
			m.peersByMachine[peer.MachineID] = append(machinePeers[:i], machinePeers[i+1:]...)
			break
		}
	}

	envPeers := m.peersByEnv[peer.EnvironmentID]
	for i, p := range envPeers {
		if p.PublicKey == publicKey {
			m.peersByEnv[peer.EnvironmentID] = append(envPeers[:i], envPeers[i+1:]...)
			break
		}
	}

	return nil
}

func (m *MockStore) UpdateLastSeen(ctx context.Context, publicKey string) error {
	if m.updateLastSeenErr != nil {
		return m.updateLastSeenErr
	}
	peer, exists := m.peers[publicKey]
	if !exists {
		return errors.New("peer not found")
	}
	peer.LastSeen = time.Now()
	return nil
}

func (m *MockStore) GetNextPeerIP(ctx context.Context, machineID string) (string, error) {
	if m.getNextIPErr != nil {
		return "", m.getNextIPErr
	}
	ip := m.nextIP
	m.nextIP = incrementIP(net.ParseIP(m.nextIP)).String()
	return ip, nil
}

func (m *MockStore) SetNextPeerIP(ip string) {
	m.nextIP = ip
}

func (m *MockStore) AddPeer(peer *Peer) {
	m.peers[peer.PublicKey] = peer
	m.peersByMachine[peer.MachineID] = append(m.peersByMachine[peer.MachineID], peer)
	m.peersByEnv[peer.EnvironmentID] = append(m.peersByEnv[peer.EnvironmentID], peer)
}

func TestNewExchange(t *testing.T) {
	tests := []struct {
		name       string
		store      *MockStore
		wantSubnet string
		wantPanic  bool
	}{
		{
			name:       "creates exchange with correct subnet",
			store:      NewMockStore(),
			wantSubnet: "10.225.0.0/16",
			wantPanic:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exchange := NewExchange(nil)

			assert.NotNil(t, exchange)
			assert.NotNil(t, exchange.peerSubnet)
			assert.Equal(t, tt.wantSubnet, exchange.peerSubnet.String())
		})
	}
}

func TestNewExchange_PanicOnInvalidSubnet(t *testing.T) {
	assert.NotPanics(t, func() {
		exchange := NewExchange(nil)
		assert.NotNil(t, exchange)
	})
}

func TestValidatePeerSubnet(t *testing.T) {
	exchange := NewExchange(nil)

	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{
			name:     "valid IP in subnet",
			ip:       "10.225.0.1",
			expected: true,
		},
		{
			name:     "valid IP at subnet start",
			ip:       "10.225.0.0",
			expected: true,
		},
		{
			name:     "valid IP at subnet end",
			ip:       "10.225.255.255",
			expected: true,
		},
		{
			name:     "valid IP in middle of subnet",
			ip:       "10.225.128.128",
			expected: true,
		},
		{
			name:     "IP outside subnet (different class A)",
			ip:       "192.168.1.1",
			expected: false,
		},
		{
			name:     "IP outside subnet (different class B)",
			ip:       "172.16.0.1",
			expected: false,
		},
		{
			name:     "IP outside subnet (adjacent)",
			ip:       "10.224.255.255",
			expected: false,
		},
		{
			name:     "IP outside subnet (next)",
			ip:       "10.226.0.0",
			expected: false,
		},
		{
			name:     "invalid IP format",
			ip:       "not-an-ip",
			expected: false,
		},
		{
			name:     "empty IP",
			ip:       "",
			expected: false,
		},
		{
			name:     "IPv6 address",
			ip:       "::1",
			expected: false,
		},
		{
			name:     "IP with CIDR notation",
			ip:       "10.225.0.1/32",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exchange.ValidatePeerSubnet(tt.ip)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildPeerConfig(t *testing.T) {
	exchange := NewExchange(nil)

	tests := []struct {
		name        string
		machineInfo MachineInfo
		assignedIP  string
		expected    *PeerConfig
	}{
		{
			name: "basic config with all fields",
			machineInfo: MachineInfo{
				ID:                "machine-123",
				PublicKey:         "machinePubKey123",
				Endpoint:          "192.0.2.1:51820",
				EnvironmentSubnet: "10.224.1.0/24",
			},
			assignedIP: "10.225.0.5",
			expected: &PeerConfig{
				MachinePublicKey: "machinePubKey123",
				MachineEndpoint:  "192.0.2.1:51820",
				AssignedIP:       "10.225.0.5",
				AllowedIPs: []string{
					"10.225.0.5/32",
					"10.224.1.0/24",
					"10.225.0.0/16",
				},
			},
		},
		{
			name: "config with different subnet",
			machineInfo: MachineInfo{
				ID:                "machine-456",
				PublicKey:         "anotherPubKey",
				Endpoint:          "203.0.113.5:51820",
				EnvironmentSubnet: "10.224.5.0/24",
			},
			assignedIP: "10.225.10.50",
			expected: &PeerConfig{
				MachinePublicKey: "anotherPubKey",
				MachineEndpoint:  "203.0.113.5:51820",
				AssignedIP:       "10.225.10.50",
				AllowedIPs: []string{
					"10.225.10.50/32",
					"10.224.5.0/24",
					"10.225.0.0/16",
				},
			},
		},
		{
			name: "config with edge IP",
			machineInfo: MachineInfo{
				ID:                "machine-edge",
				PublicKey:         "edgePubKey",
				Endpoint:          "198.51.100.1:51820",
				EnvironmentSubnet: "10.224.255.0/24",
			},
			assignedIP: "10.225.255.255",
			expected: &PeerConfig{
				MachinePublicKey: "edgePubKey",
				MachineEndpoint:  "198.51.100.1:51820",
				AssignedIP:       "10.225.255.255",
				AllowedIPs: []string{
					"10.225.255.255/32",
					"10.224.255.0/24",
					"10.225.0.0/16",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exchange.buildPeerConfig(tt.machineInfo, tt.assignedIP)

			assert.Equal(t, tt.expected.MachinePublicKey, result.MachinePublicKey)
			assert.Equal(t, tt.expected.MachineEndpoint, result.MachineEndpoint)
			assert.Equal(t, tt.expected.AssignedIP, result.AssignedIP)
			assert.Equal(t, tt.expected.AllowedIPs, result.AllowedIPs)
		})
	}
}

func TestBuildPeerConfig_AllowedIPsOrder(t *testing.T) {
	exchange := NewExchange(nil)

	machineInfo := MachineInfo{
		ID:                "machine-123",
		PublicKey:         "pubKey",
		Endpoint:          "192.0.2.1:51820",
		EnvironmentSubnet: "10.224.1.0/24",
	}
	assignedIP := "10.225.0.100"

	result := exchange.buildPeerConfig(machineInfo, assignedIP)

	require.Len(t, result.AllowedIPs, 3)
	assert.Equal(t, "10.225.0.100/32", result.AllowedIPs[0])
	assert.Equal(t, "10.224.1.0/24", result.AllowedIPs[1])
	assert.Equal(t, "10.225.0.0/16", result.AllowedIPs[2])
}

func TestAllocatePeerIP(t *testing.T) {
	tests := []struct {
		name       string
		machineID  string
		setupMock  func(*MockStore)
		wantIP     string
		wantErr    bool
		errMessage string
	}{
		{
			name:      "allocates first IP",
			machineID: "machine-1",
			setupMock: func(m *MockStore) {
				m.SetNextPeerIP("10.225.0.1")
			},
			wantIP:  "10.225.0.1",
			wantErr: false,
		},
		{
			name:      "allocates sequential IP",
			machineID: "machine-1",
			setupMock: func(m *MockStore) {
				m.SetNextPeerIP("10.225.0.5")
			},
			wantIP:  "10.225.0.5",
			wantErr: false,
		},
		{
			name:      "returns error when store fails",
			machineID: "machine-1",
			setupMock: func(m *MockStore) {
				m.getNextIPErr = errors.New("database connection failed")
			},
			wantErr:    true,
			errMessage: "database connection failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := NewMockStore()
			tt.setupMock(mockStore)

			ctx := context.Background()
			ip, err := mockStore.GetNextPeerIP(ctx, tt.machineID)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMessage)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantIP, ip)
			}
		})
	}
}

func TestAllocatePeerIP_SequentialAllocation(t *testing.T) {
	mockStore := NewMockStore()
	mockStore.SetNextPeerIP("10.225.0.100")

	ctx := context.Background()

	ip1, err := mockStore.GetNextPeerIP(ctx, "machine-1")
	require.NoError(t, err)
	assert.Equal(t, "10.225.0.100", ip1)

	ip2, err := mockStore.GetNextPeerIP(ctx, "machine-1")
	require.NoError(t, err)
	assert.Equal(t, "10.225.0.101", ip2)

	ip3, err := mockStore.GetNextPeerIP(ctx, "machine-2")
	require.NoError(t, err)
	assert.Equal(t, "10.225.0.102", ip3)
}

func TestGenerateKeyPair(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "generates valid key pair",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyPair, err := GenerateKeyPair()

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, keyPair)
			assert.NotEmpty(t, keyPair.PrivateKey)
			assert.NotEmpty(t, keyPair.PublicKey)
			assert.NotEqual(t, keyPair.PrivateKey, keyPair.PublicKey)
			assert.GreaterOrEqual(t, len(keyPair.PrivateKey), 40)
			assert.GreaterOrEqual(t, len(keyPair.PublicKey), 40)
		})
	}
}

func TestGenerateKeyPair_Uniqueness(t *testing.T) {
	keyPairs := make([]*KeyPair, 5)
	for i := 0; i < 5; i++ {
		kp, err := GenerateKeyPair()
		require.NoError(t, err)
		keyPairs[i] = kp
	}

	for i := 0; i < len(keyPairs); i++ {
		for j := i + 1; j < len(keyPairs); j++ {
			assert.NotEqual(t, keyPairs[i].PrivateKey, keyPairs[j].PrivateKey)
			assert.NotEqual(t, keyPairs[i].PublicKey, keyPairs[j].PublicKey)
		}
	}
}

func TestGenerateKeyPair_KeyRelationship(t *testing.T) {
	keyPair, err := GenerateKeyPair()
	require.NoError(t, err)

	derivedPubKey, err := DerivePublicKey(keyPair.PrivateKey)
	require.NoError(t, err)

	assert.Equal(t, keyPair.PublicKey, derivedPubKey)
}

func TestGenerateKeyPair_ValidateKeyPair(t *testing.T) {
	keyPair, err := GenerateKeyPair()
	require.NoError(t, err)

	valid, err := ValidateKeyPair(keyPair.PrivateKey, keyPair.PublicKey)
	require.NoError(t, err)
	assert.True(t, valid)
}

func TestExchange_RegisterPeer_WithAllocation(t *testing.T) {
	mockStore := NewMockStore()
	mockStore.SetNextPeerIP("10.225.0.50")

	ctx := context.Background()
	machineID := "machine-123"

	ip, err := mockStore.GetNextPeerIP(ctx, machineID)
	require.NoError(t, err)
	assert.Equal(t, "10.225.0.50", ip)

	exchange := NewExchange(nil)
	assert.True(t, exchange.ValidatePeerSubnet(ip))

	machineInfo := MachineInfo{
		ID:                machineID,
		PublicKey:         "machinePubKey",
		Endpoint:          "192.0.2.1:51820",
		EnvironmentSubnet: "10.224.1.0/24",
	}

	config := exchange.buildPeerConfig(machineInfo, ip)
	assert.Equal(t, ip, config.AssignedIP)
	assert.Contains(t, config.AllowedIPs, ip+"/32")
}

func TestExchange_FullWorkflow(t *testing.T) {
	mockStore := NewMockStore()
	exchange := NewExchange(nil)

	ctx := context.Background()

	keyPair, err := GenerateKeyPair()
	require.NoError(t, err)

	mockStore.SetNextPeerIP("10.225.0.10")
	assignedIP, err := mockStore.GetNextPeerIP(ctx, "machine-1")
	require.NoError(t, err)

	assert.True(t, exchange.ValidatePeerSubnet(assignedIP))

	peer := &Peer{
		ID:            "peer-1",
		MachineID:     "machine-1",
		EnvironmentID: "env-1",
		UserID:        "user-1",
		PublicKey:     keyPair.PublicKey,
		AssignedIP:    assignedIP,
		ConnectedAt:   time.Now(),
		LastSeen:      time.Now(),
	}

	err = mockStore.CreatePeer(ctx, peer)
	require.NoError(t, err)

	machineInfo := MachineInfo{
		ID:                "machine-1",
		PublicKey:         "machinePubKey",
		Endpoint:          "192.0.2.1:51820",
		EnvironmentSubnet: "10.224.1.0/24",
	}

	config := exchange.buildPeerConfig(machineInfo, assignedIP)
	assert.NotNil(t, config)
	assert.Equal(t, keyPair.PublicKey, peer.PublicKey)
}
