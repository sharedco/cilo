//go:build darwin

package tunnel

import (
	"fmt"
	"net"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// Manager handles WireGuard tunnel operations for macOS userspace implementation
type Manager struct {
	deviceName string
	wgDevice   *device.Device
}

// NewManager creates a new WireGuard manager for macOS
func NewManager(deviceName string) (*Manager, error) {
	wgDev, exists := getDevice(deviceName)
	if !exists {
		return nil, fmt.Errorf("device %s not found in registry", deviceName)
	}

	return &Manager{
		deviceName: deviceName,
		wgDevice:   wgDev,
	}, nil
}

// Close closes the WireGuard manager
func (m *Manager) Close() error {
	// The device is managed by the registry in interface_darwin.go
	// We just clear our reference
	m.wgDevice = nil
	return nil
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

	uapiConfig := buildUAPISetConfig(config)
	return m.wgDevice.IpcSet(uapiConfig)
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

	uapiConfig := buildUAPISetConfig(config)
	return m.wgDevice.IpcSet(uapiConfig)
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

	uapiConfig := buildUAPISetConfig(config)
	return m.wgDevice.IpcSet(uapiConfig)
}

// GetDevice returns the WireGuard device configuration
func (m *Manager) GetDevice() (*wgtypes.Device, error) {
	// For macOS userspace, we need to use IpcGet to get the device config
	uapiConfig, err := m.wgDevice.IpcGet()
	if err != nil {
		return nil, fmt.Errorf("get device config: %w", err)
	}

	return parseUAPIGetConfig(uapiConfig, m.deviceName)
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

// buildUAPISetConfig converts wgtypes.Config to UAPI format string for device.IpcSet
func buildUAPISetConfig(config wgtypes.Config) string {
	var parts []string

	if config.PrivateKey != nil {
		parts = append(parts, fmt.Sprintf("private_key=%s", hexKey(config.PrivateKey[:])))
	}

	if config.ListenPort != nil {
		parts = append(parts, fmt.Sprintf("listen_port=%d", *config.ListenPort))
	}

	if config.FirewallMark != nil {
		parts = append(parts, fmt.Sprintf("fwmark=%d", *config.FirewallMark))
	}

	if config.ReplacePeers {
		parts = append(parts, "replace_peers=true")
	}

	for _, peer := range config.Peers {
		parts = append(parts, fmt.Sprintf("public_key=%s", hexKey(peer.PublicKey[:])))

		if peer.Remove {
			parts = append(parts, "remove=true")
			continue
		}

		if peer.PresharedKey != nil {
			parts = append(parts, fmt.Sprintf("preshared_key=%s", hexKey(peer.PresharedKey[:])))
		}

		if peer.Endpoint != nil {
			parts = append(parts, fmt.Sprintf("endpoint=%s", peer.Endpoint.String()))
		}

		if peer.PersistentKeepaliveInterval != nil {
			seconds := int(peer.PersistentKeepaliveInterval.Seconds())
			parts = append(parts, fmt.Sprintf("persistent_keepalive_interval=%d", seconds))
		}

		if peer.ReplaceAllowedIPs {
			parts = append(parts, "replace_allowed_ips=true")
		}

		for _, ipNet := range peer.AllowedIPs {
			parts = append(parts, fmt.Sprintf("allowed_ip=%s", ipNet.String()))
		}
	}

	return strings.Join(parts, "\n") + "\n"
}

// parseUAPIGetConfig parses UAPI format response from device.IpcGet into wgtypes.Device
func parseUAPIGetConfig(config string, deviceName string) (*wgtypes.Device, error) {
	// This is a simplified parser - in production, you'd want more robust parsing
	device := &wgtypes.Device{
		Name: deviceName,
	}

	lines := strings.Split(config, "\n")
	var currentPeer *wgtypes.Peer

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key, value := parts[0], parts[1]

		switch key {
		case "private_key":
			if k, err := parseHexKey(value); err == nil {
				device.PrivateKey = k
			}
		case "public_key":
			// Save previous peer if exists
			if currentPeer != nil {
				device.Peers = append(device.Peers, *currentPeer)
			}
			// Start new peer
			if k, err := parseHexKey(value); err == nil {
				currentPeer = &wgtypes.Peer{PublicKey: k}
			}
		case "listen_port":
			fmt.Sscanf(value, "%d", &device.ListenPort)
		case "fwmark":
			fmt.Sscanf(value, "%d", &device.FirewallMark)
		case "endpoint":
			if currentPeer != nil {
				currentPeer.Endpoint, _ = net.ResolveUDPAddr("udp", value)
			}
		case "allowed_ip":
			if currentPeer != nil {
				_, ipNet, err := net.ParseCIDR(value)
				if err == nil {
					currentPeer.AllowedIPs = append(currentPeer.AllowedIPs, *ipNet)
				}
			}
		case "persistent_keepalive_interval":
			if currentPeer != nil {
				var seconds int
				_, err := fmt.Sscanf(value, "%d", &seconds)
				if err == nil {
					currentPeer.PersistentKeepaliveInterval = time.Duration(seconds) * time.Second
				}
			}
		case "rx_bytes":
			if currentPeer != nil {
				fmt.Sscanf(value, "%d", &currentPeer.ReceiveBytes)
			}
		case "tx_bytes":
			if currentPeer != nil {
				fmt.Sscanf(value, "%d", &currentPeer.TransmitBytes)
			}
		case "last_handshake_time_sec":
			if currentPeer != nil {
				var sec int64
				_, err := fmt.Sscanf(value, "%d", &sec)
				if err == nil {
					currentPeer.LastHandshakeTime = time.Unix(sec, 0)
				}
			}
		}
	}

	// Don't forget the last peer
	if currentPeer != nil {
		device.Peers = append(device.Peers, *currentPeer)
	}

	return device, nil
}

// hexKey converts a byte slice to hex string (without encoding/hex import)
func hexKey(key []byte) string {
	const hexDigits = "0123456789abcdef"
	result := make([]byte, len(key)*2)
	for i, b := range key {
		result[i*2] = hexDigits[b>>4]
		result[i*2+1] = hexDigits[b&0x0f]
	}
	return string(result)
}

// parseHexKey parses a hex string into a wgtypes.Key
func parseHexKey(s string) (wgtypes.Key, error) {
	var key wgtypes.Key
	if len(s) != len(key)*2 {
		return key, fmt.Errorf("invalid key length")
	}

	for i := 0; i < len(key); i++ {
		b1 := hexValue(s[i*2])
		b2 := hexValue(s[i*2+1])
		if b1 < 0 || b2 < 0 {
			return key, fmt.Errorf("invalid hex character")
		}
		key[i] = byte(b1<<4 | b2)
	}

	return key, nil
}

// hexValue returns the numeric value of a hex digit
func hexValue(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c - 'a' + 10)
	case c >= 'A' && c <= 'F':
		return int(c - 'A' + 10)
	}
	return -1
}
