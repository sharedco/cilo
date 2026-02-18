// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

//go:build linux

package tunnel

import (
	"fmt"
	"net"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// Manager handles WireGuard tunnel operations
type Manager struct {
	client     *wgctrl.Client
	deviceName string
}

// NewManager creates a new WireGuard manager
func NewManager(deviceName string) (*Manager, error) {
	client, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("create wgctrl client: %w", err)
	}

	return &Manager{
		client:     client,
		deviceName: deviceName,
	}, nil
}

// Close closes the WireGuard control client
func (m *Manager) Close() error {
	return m.client.Close()
}

// Configure configures the WireGuard device with private key and listen port
func (m *Manager) Configure(privateKey string, listenPort int) error {
	key, err := wgtypes.ParseKey(privateKey)
	if err != nil {
		return fmt.Errorf("parse private key: %w", err)
	}

	config := wgtypes.Config{
		PrivateKey:   &key,
		ListenPort:   &listenPort,
		ReplacePeers: false,
	}

	return m.client.ConfigureDevice(m.deviceName, config)
}

// AddPeer adds or updates a peer
func (m *Manager) AddPeer(publicKey string, endpoint string, allowedIPs []string, keepalive time.Duration) error {
	pubKey, err := wgtypes.ParseKey(publicKey)
	if err != nil {
		return fmt.Errorf("parse public key: %w", err)
	}

	var udpEndpoint *net.UDPAddr
	if endpoint != "" {
		udpEndpoint, err = net.ResolveUDPAddr("udp", endpoint)
		if err != nil {
			return fmt.Errorf("resolve endpoint: %w", err)
		}
	}

	allowedIPNets := make([]net.IPNet, len(allowedIPs))
	for i, cidr := range allowedIPs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return fmt.Errorf("parse allowed IP %s: %w", cidr, err)
		}
		allowedIPNets[i] = *ipNet
	}

	peer := wgtypes.PeerConfig{
		PublicKey:                   pubKey,
		Endpoint:                    udpEndpoint,
		AllowedIPs:                  allowedIPNets,
		ReplaceAllowedIPs:           true,
		PersistentKeepaliveInterval: &keepalive,
	}

	config := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{peer},
	}

	return m.client.ConfigureDevice(m.deviceName, config)
}

// RemovePeer removes a peer by public key
func (m *Manager) RemovePeer(publicKey string) error {
	pubKey, err := wgtypes.ParseKey(publicKey)
	if err != nil {
		return fmt.Errorf("parse public key: %w", err)
	}

	peer := wgtypes.PeerConfig{
		PublicKey: pubKey,
		Remove:    true,
	}

	config := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{peer},
	}

	return m.client.ConfigureDevice(m.deviceName, config)
}

// GetDevice returns the WireGuard device configuration
func (m *Manager) GetDevice() (*wgtypes.Device, error) {
	return m.client.Device(m.deviceName)
}

// GetPeerStats returns statistics for all peers
func (m *Manager) GetPeerStats() ([]PeerStats, error) {
	device, err := m.GetDevice()
	if err != nil {
		return nil, err
	}

	stats := make([]PeerStats, len(device.Peers))
	for i, peer := range device.Peers {
		var endpoint string
		if peer.Endpoint != nil {
			endpoint = peer.Endpoint.String()
		}

		allowedIPs := make([]string, len(peer.AllowedIPs))
		for j, ip := range peer.AllowedIPs {
			allowedIPs[j] = ip.String()
		}

		stats[i] = PeerStats{
			PublicKey:     peer.PublicKey.String(),
			Endpoint:      endpoint,
			AllowedIPs:    allowedIPs,
			LastHandshake: peer.LastHandshakeTime,
			RxBytes:       peer.ReceiveBytes,
			TxBytes:       peer.TransmitBytes,
		}
	}

	return stats, nil
}
