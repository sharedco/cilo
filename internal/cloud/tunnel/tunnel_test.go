// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package tunnel

import (
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "creates tunnel with valid config",
			cfg: Config{
				Interface:  "utun100",
				ListenPort: 51820,
				Address:    "10.225.0.1/24",
			},
			wantErr: false,
		},
		{
			name: "creates tunnel with different interface",
			cfg: Config{
				Interface:  "wg0",
				ListenPort: 51821,
				Address:    "192.168.100.1/24",
			},
			wantErr: false,
		},
		{
			name: "creates tunnel with IPv6 address",
			cfg: Config{
				Interface:  "utun101",
				ListenPort: 51822,
				Address:    "fd00:1234::1/64",
			},
			wantErr: false,
		},
		{
			name: "creates tunnel with zero listen port",
			cfg: Config{
				Interface:  "utun102",
				ListenPort: 0,
				Address:    "10.225.0.2/24",
			},
			wantErr: false,
		},
		{
			name: "creates tunnel with empty address",
			cfg: Config{
				Interface:  "utun103",
				ListenPort: 51823,
				Address:    "",
			},
			wantErr: false,
		},
		{
			name: "creates tunnel with high port number",
			cfg: Config{
				Interface:  "utun104",
				ListenPort: 65535,
				Address:    "10.225.0.3/24",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tunnel, err := New(tt.cfg)

			if tt.wantErr {
				if err == nil {
					t.Errorf("New() error = nil, wantErr = true")
				}
				return
			}

			if err != nil {
				t.Errorf("New() error = %v, wantErr = false", err)
				return
			}

			// Verify tunnel is not nil
			if tunnel == nil {
				t.Fatal("New() returned nil tunnel")
			}

			// Verify Interface is set correctly
			if tunnel.Interface != tt.cfg.Interface {
				t.Errorf("Interface = %s, want %s", tunnel.Interface, tt.cfg.Interface)
			}

			// Verify ListenPort is set correctly
			if tunnel.ListenPort != tt.cfg.ListenPort {
				t.Errorf("ListenPort = %d, want %d", tunnel.ListenPort, tt.cfg.ListenPort)
			}

			// Verify Address is set correctly
			if tunnel.Address != tt.cfg.Address {
				t.Errorf("Address = %s, want %s", tunnel.Address, tt.cfg.Address)
			}

			// Verify keys are generated (not empty)
			if tunnel.PrivateKey == "" {
				t.Error("PrivateKey is empty")
			}

			if tunnel.PublicKey == "" {
				t.Error("PublicKey is empty")
			}

			// Verify keys are different
			if tunnel.PrivateKey == tunnel.PublicKey {
				t.Error("PrivateKey and PublicKey should be different")
			}

			// Verify manager is initially nil (not set until Setup)
			if tunnel.manager != nil {
				t.Error("manager should be nil before Setup() is called")
			}
		})
	}
}

func TestNew_GeneratesUniqueKeys(t *testing.T) {
	// Create multiple tunnels and verify they have unique keys
	cfg := Config{
		Interface:  "utun200",
		ListenPort: 51830,
		Address:    "10.225.1.1/24",
	}

	tunnel1, err := New(cfg)
	if err != nil {
		t.Fatalf("First New() error = %v", err)
	}

	cfg.Interface = "utun201"
	cfg.ListenPort = 51831

	tunnel2, err := New(cfg)
	if err != nil {
		t.Fatalf("Second New() error = %v", err)
	}

	// Verify keys are unique between tunnels
	if tunnel1.PrivateKey == tunnel2.PrivateKey {
		t.Error("Different tunnels should have different private keys")
	}

	if tunnel1.PublicKey == tunnel2.PublicKey {
		t.Error("Different tunnels should have different public keys")
	}
}

func TestTunnel_StructFields(t *testing.T) {
	// Test direct struct field assignment
	tunnel := &Tunnel{
		Interface:  "test-interface",
		PrivateKey: "test-private-key",
		PublicKey:  "test-public-key",
		ListenPort: 12345,
		Address:    "10.0.0.1/24",
	}

	if tunnel.Interface != "test-interface" {
		t.Errorf("Interface field = %s, want test-interface", tunnel.Interface)
	}

	if tunnel.PrivateKey != "test-private-key" {
		t.Errorf("PrivateKey field = %s, want test-private-key", tunnel.PrivateKey)
	}

	if tunnel.PublicKey != "test-public-key" {
		t.Errorf("PublicKey field = %s, want test-public-key", tunnel.PublicKey)
	}

	if tunnel.ListenPort != 12345 {
		t.Errorf("ListenPort field = %d, want 12345", tunnel.ListenPort)
	}

	if tunnel.Address != "10.0.0.1/24" {
		t.Errorf("Address field = %s, want 10.0.0.1/24", tunnel.Address)
	}
}

func TestTunnel_KeyFormat(t *testing.T) {
	cfg := Config{
		Interface:  "utun300",
		ListenPort: 51840,
		Address:    "10.225.2.1/24",
	}

	tunnel, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Verify keys are base64 encoded (contain only valid base64 characters)
	base64Pattern := "^[A-Za-z0-9+/=_-]+$"

	// Check private key format
	if !isValidBase64(tunnel.PrivateKey) {
		t.Errorf("PrivateKey is not valid base64: %s", tunnel.PrivateKey)
	}

	// Check public key format
	if !isValidBase64(tunnel.PublicKey) {
		t.Errorf("PublicKey is not valid base64: %s", tunnel.PublicKey)
	}

	// Verify key lengths (WireGuard keys are 32 bytes = 44 chars in base64 with padding)
	if len(tunnel.PrivateKey) != 44 {
		t.Errorf("PrivateKey length = %d, want 44", len(tunnel.PrivateKey))
	}

	if len(tunnel.PublicKey) != 44 {
		t.Errorf("PublicKey length = %d, want 44", len(tunnel.PublicKey))
	}

	_ = base64Pattern // Used for documentation
}

func isValidBase64(s string) bool {
	// WireGuard base64 keys only use standard base64 characters
	for _, c := range s {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '+' || c == '/' || c == '=' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

func TestConfig_Struct(t *testing.T) {
	// Test Config struct field assignment
	cfg := Config{
		Interface:  "wg-test",
		ListenPort: 51850,
		Address:    "172.16.0.1/16",
	}

	if cfg.Interface != "wg-test" {
		t.Errorf("Config.Interface = %s, want wg-test", cfg.Interface)
	}

	if cfg.ListenPort != 51850 {
		t.Errorf("Config.ListenPort = %d, want 51850", cfg.ListenPort)
	}

	if cfg.Address != "172.16.0.1/16" {
		t.Errorf("Config.Address = %s, want 172.16.0.1/16", cfg.Address)
	}
}

func TestNew_InterfaceNameVariations(t *testing.T) {
	// Test various interface name formats
	interfaceNames := []string{
		"utun100",
		"utun999",
		"wg0",
		"wg99",
		"cilo0",
		"cilo-tunnel",
		"tunnel_1",
	}

	for i, name := range interfaceNames {
		cfg := Config{
			Interface:  name,
			ListenPort: 51860 + i,
			Address:    "10.225.3.1/24",
		}

		tunnel, err := New(cfg)
		if err != nil {
			t.Errorf("New() with interface %s error = %v", name, err)
			continue
		}

		if tunnel.Interface != name {
			t.Errorf("Interface = %s, want %s", tunnel.Interface, name)
		}
	}
}

func TestNew_AddressFormats(t *testing.T) {
	// Test various address formats
	addresses := []string{
		"10.0.0.1/8",
		"172.16.0.1/16",
		"192.168.1.1/24",
		"10.225.0.1/24",
		"fd00::1/64",
		"2001:db8::1/32",
		"",
	}

	for i, addr := range addresses {
		cfg := Config{
			Interface:  "utun400",
			ListenPort: 51870 + i,
			Address:    addr,
		}

		tunnel, err := New(cfg)
		if err != nil {
			t.Errorf("New() with address %s error = %v", addr, err)
			continue
		}

		if tunnel.Address != addr {
			t.Errorf("Address = %s, want %s", tunnel.Address, addr)
		}
	}
}

func TestNew_PublicKeyDerivation(t *testing.T) {
	cfg := Config{
		Interface:  "utun500",
		ListenPort: 51880,
		Address:    "10.225.4.1/24",
	}

	tunnel, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Parse the private key and derive its public key
	privateKey, err := ParseKey(tunnel.PrivateKey)
	if err != nil {
		t.Fatalf("Failed to parse private key: %v", err)
	}

	derivedPublicKey := privateKey.PublicKey().String()

	// Verify the tunnel's public key matches the derived one
	if tunnel.PublicKey != derivedPublicKey {
		t.Error("Tunnel's PublicKey does not match the derived public key from PrivateKey")
	}
}

func TestTunnel_MultipleCreations(t *testing.T) {
	// Test creating multiple tunnels doesn't interfere with each other
	tunnels := make([]*Tunnel, 5)

	for i := 0; i < 5; i++ {
		cfg := Config{
			Interface:  "utun600",
			ListenPort: 51890 + i,
			Address:    "10.225.5.1/24",
		}

		tunnel, err := New(cfg)
		if err != nil {
			t.Fatalf("New() iteration %d error = %v", i, err)
		}

		tunnels[i] = tunnel
	}

	// Verify all tunnels have unique keys
	keys := make(map[string]bool)
	for i, tunnel := range tunnels {
		if keys[tunnel.PrivateKey] {
			t.Errorf("Duplicate private key at index %d", i)
		}
		keys[tunnel.PrivateKey] = true

		if keys[tunnel.PublicKey] {
			t.Errorf("Duplicate public key at index %d", i)
		}
		keys[tunnel.PublicKey] = true
	}
}

func TestNew_EmptyInterface(t *testing.T) {
	// Test that empty interface name is accepted (validation happens elsewhere)
	cfg := Config{
		Interface:  "",
		ListenPort: 51900,
		Address:    "10.225.6.1/24",
	}

	tunnel, err := New(cfg)
	if err != nil {
		t.Fatalf("New() with empty interface error = %v", err)
	}

	if tunnel.Interface != "" {
		t.Errorf("Interface = %s, want empty string", tunnel.Interface)
	}
}

func TestNew_NegativePort(t *testing.T) {
	// Test that negative port is accepted (validation happens elsewhere)
	cfg := Config{
		Interface:  "utun700",
		ListenPort: -1,
		Address:    "10.225.7.1/24",
	}

	tunnel, err := New(cfg)
	if err != nil {
		t.Fatalf("New() with negative port error = %v", err)
	}

	if tunnel.ListenPort != -1 {
		t.Errorf("ListenPort = %d, want -1", tunnel.ListenPort)
	}
}

// Benchmark tests

func BenchmarkNew(b *testing.B) {
	cfg := Config{
		Interface:  "utun800",
		ListenPort: 51910,
		Address:    "10.225.8.1/24",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := New(cfg)
		if err != nil {
			b.Fatalf("New() error = %v", err)
		}
	}
}

func BenchmarkNew_Parallel(b *testing.B) {
	cfg := Config{
		Interface:  "utun900",
		ListenPort: 51920,
		Address:    "10.225.9.1/24",
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := New(cfg)
			if err != nil {
				b.Fatalf("New() error = %v", err)
			}
		}
	})
}
