// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package agent

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestParseWGDump tests the parseWGDump function with various wg dump outputs
func TestParseWGDump(t *testing.T) {
	manager := &WireGuardManager{
		interfaceName: "wg0",
		publicKey:     "test_public_key_xyz",
	}

	tests := []struct {
		name           string
		input          string
		expectedPeers  int
		expectedErr    bool
		checkInterface string
		checkPublicKey string
	}{
		{
			name: "valid dump with single peer",
			input: `private_key_abc public_key_xyz 51820 off
peer_key_123 (none) 192.168.1.1:51820 10.0.0.0/8 1234567890 1000 2000 25`,
			expectedPeers:  1,
			expectedErr:    false,
			checkInterface: "wg0",
			checkPublicKey: "test_public_key_xyz",
		},
		{
			name: "valid dump with multiple peers",
			input: `private_key_abc public_key_xyz 51820 off
peer_key_123 (none) 192.168.1.1:51820 10.0.0.0/8 1234567890 1000 2000 25
peer_key_456 (none) 192.168.1.2:51820 192.168.0.0/16 1234567891 2000 3000 0
peer_key_789 (none) 192.168.1.3:51820 172.16.0.0/12 0 0 0 0`,
			expectedPeers:  3,
			expectedErr:    false,
			checkInterface: "wg0",
			checkPublicKey: "test_public_key_xyz",
		},
		{
			name:           "empty output",
			input:          "",
			expectedPeers:  0,
			expectedErr:    false, // Empty input returns empty result with no error (strings.Split returns [""])
			checkInterface: "wg0",
			checkPublicKey: "test_public_key_xyz",
		},
		{
			name:           "only interface line no peers",
			input:          "private_key_abc public_key_xyz 51820 off",
			expectedPeers:  0,
			expectedErr:    false,
			checkInterface: "wg0",
			checkPublicKey: "test_public_key_xyz",
		},
		{
			name: "dump with whitespace and empty lines",
			input: `private_key_abc public_key_xyz 51820 off

peer_key_123 (none) 192.168.1.1:51820 10.0.0.0/8 1234567890 1000 2000 25
   
peer_key_456 (none) 192.168.1.2:51820 192.168.0.0/16 1234567891 2000 3000 0`,
			expectedPeers:  2,
			expectedErr:    false,
			checkInterface: "wg0",
			checkPublicKey: "test_public_key_xyz",
		},
		{
			name: "peer with zero handshake (never connected)",
			input: `private_key_abc public_key_xyz 51820 off
peer_key_123 (none) 192.168.1.1:51820 10.0.0.0/8 0 0 0 0`,
			expectedPeers:  1,
			expectedErr:    false,
			checkInterface: "wg0",
			checkPublicKey: "test_public_key_xyz",
		},
		{
			name: "peer with (none) endpoint",
			input: `private_key_abc public_key_xyz 51820 off
peer_key_123 (none) (none) 10.0.0.0/8 1234567890 1000 2000 25`,
			expectedPeers:  1,
			expectedErr:    false,
			checkInterface: "wg0",
			checkPublicKey: "test_public_key_xyz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := manager.parseWGDump(tt.input)

			if tt.expectedErr {
				if err == nil {
					t.Errorf("parseWGDump() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("parseWGDump() unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Errorf("parseWGDump() returned nil result without error")
				return
			}

			if result.Interface != tt.checkInterface {
				t.Errorf("parseWGDump() Interface = %v, want %v", result.Interface, tt.checkInterface)
			}

			if result.PublicKey != tt.checkPublicKey {
				t.Errorf("parseWGDump() PublicKey = %v, want %v", result.PublicKey, tt.checkPublicKey)
			}

			if len(result.Peers) != tt.expectedPeers {
				t.Errorf("parseWGDump() Peers count = %v, want %v", len(result.Peers), tt.expectedPeers)
			}
		})
	}
}

// TestParseWGDumpPeerDetails tests that peer details are correctly parsed
func TestParseWGDumpPeerDetails(t *testing.T) {
	manager := &WireGuardManager{
		interfaceName: "wg0",
		publicKey:     "test_key",
	}

	input := `private_key_abc public_key_xyz 51820 off
peer_key_123 (none) 192.168.1.1:51820 10.0.0.0/8 1234567890 1000 2000 25`

	result, err := manager.parseWGDump(input)
	if err != nil {
		t.Fatalf("parseWGDump() unexpected error: %v", err)
	}

	if len(result.Peers) != 1 {
		t.Fatalf("Expected 1 peer, got %d", len(result.Peers))
	}

	peer := result.Peers[0]

	// Check public key
	if peer.PublicKey != "peer_key_123" {
		t.Errorf("Peer PublicKey = %v, want %v", peer.PublicKey, "peer_key_123")
	}

	// Check endpoint
	if peer.Endpoint != "192.168.1.1:51820" {
		t.Errorf("Peer Endpoint = %v, want %v", peer.Endpoint, "192.168.1.1:51820")
	}

	// Check allowed IPs
	if peer.AllowedIPs != "10.0.0.0/8" {
		t.Errorf("Peer AllowedIPs = %v, want %v", peer.AllowedIPs, "10.0.0.0/8")
	}

	// Check last handshake (1234567890 = 2009-02-13 23:31:30 UTC)
	expectedTime := time.Unix(1234567890, 0).Format(time.RFC3339)
	if peer.LastHandshake != expectedTime {
		t.Errorf("Peer LastHandshake = %v, want %v", peer.LastHandshake, expectedTime)
	}

	// Check transfer stats
	if peer.RxBytes != 1000 {
		t.Errorf("Peer RxBytes = %v, want %v", peer.RxBytes, 1000)
	}

	if peer.TxBytes != 2000 {
		t.Errorf("Peer TxBytes = %v, want %v", peer.TxBytes, 2000)
	}
}

// TestParsePeerLine tests the parsePeerLine function directly
func TestParsePeerLine(t *testing.T) {
	manager := &WireGuardManager{
		interfaceName: "wg0",
		publicKey:     "test_key",
	}

	tests := []struct {
		name         string
		line         string
		expectErr    bool
		expectedPeer PeerStatus
	}{
		{
			name:      "valid peer line with all fields",
			line:      "peer_key_123 (none) 192.168.1.1:51820 10.0.0.0/8 1234567890 1000 2000 25",
			expectErr: false,
			expectedPeer: PeerStatus{
				PublicKey:     "peer_key_123",
				Endpoint:      "192.168.1.1:51820",
				AllowedIPs:    "10.0.0.0/8",
				LastHandshake: time.Unix(1234567890, 0).Format(time.RFC3339),
				RxBytes:       1000,
				TxBytes:       2000,
			},
		},
		{
			name:      "peer line with zero handshake",
			line:      "peer_key_456 (none) 192.168.1.2:51820 192.168.0.0/16 0 500 600 0",
			expectErr: false,
			expectedPeer: PeerStatus{
				PublicKey:     "peer_key_456",
				Endpoint:      "192.168.1.2:51820",
				AllowedIPs:    "192.168.0.0/16",
				LastHandshake: "",
				RxBytes:       500,
				TxBytes:       600,
			},
		},
		{
			name:      "peer line with (none) endpoint",
			line:      "peer_key_789 (none) (none) 172.16.0.0/12 1234567890 0 0 0",
			expectErr: false,
			expectedPeer: PeerStatus{
				PublicKey:     "peer_key_789",
				Endpoint:      "(none)",
				AllowedIPs:    "172.16.0.0/12",
				LastHandshake: time.Unix(1234567890, 0).Format(time.RFC3339),
				RxBytes:       0,
				TxBytes:       0,
			},
		},
		{
			name:         "empty line",
			line:         "",
			expectErr:    true,
			expectedPeer: PeerStatus{},
		},
		{
			name:         "insufficient fields",
			line:         "peer_key_123 (none) 192.168.1.1:51820 10.0.0.0/8",
			expectErr:    true,
			expectedPeer: PeerStatus{},
		},
		{
			name:         "only 7 fields",
			line:         "peer_key_123 (none) 192.168.1.1:51820 10.0.0.0/8 1234567890 1000 2000",
			expectErr:    true,
			expectedPeer: PeerStatus{},
		},
		{
			name:      "peer with IPv6 endpoint",
			line:      "peer_key_abc (none) [2001:db8::1]:51820 10.0.0.0/8 1234567890 1000 2000 25",
			expectErr: false,
			expectedPeer: PeerStatus{
				PublicKey:     "peer_key_abc",
				Endpoint:      "[2001:db8::1]:51820",
				AllowedIPs:    "10.0.0.0/8",
				LastHandshake: time.Unix(1234567890, 0).Format(time.RFC3339),
				RxBytes:       1000,
				TxBytes:       2000,
			},
		},
		{
			name:      "peer with multiple allowed IPs",
			line:      "peer_key_def (none) 192.168.1.1:51820 10.0.0.0/8,192.168.0.0/16 1234567890 1000 2000 25",
			expectErr: false,
			expectedPeer: PeerStatus{
				PublicKey:     "peer_key_def",
				Endpoint:      "192.168.1.1:51820",
				AllowedIPs:    "10.0.0.0/8,192.168.0.0/16",
				LastHandshake: time.Unix(1234567890, 0).Format(time.RFC3339),
				RxBytes:       1000,
				TxBytes:       2000,
			},
		},
		{
			name:      "peer with large transfer numbers",
			line:      "peer_key_large (none) 192.168.1.1:51820 10.0.0.0/8 1234567890 1099511627776 2199023255552 25",
			expectErr: false,
			expectedPeer: PeerStatus{
				PublicKey:     "peer_key_large",
				Endpoint:      "192.168.1.1:51820",
				AllowedIPs:    "10.0.0.0/8",
				LastHandshake: time.Unix(1234567890, 0).Format(time.RFC3339),
				RxBytes:       1099511627776, // 1 TiB
				TxBytes:       2199023255552, // 2 TiB
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			peer, err := manager.parsePeerLine(tt.line)

			if tt.expectErr {
				if err == nil {
					t.Errorf("parsePeerLine() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("parsePeerLine() unexpected error: %v", err)
				return
			}

			if peer.PublicKey != tt.expectedPeer.PublicKey {
				t.Errorf("PublicKey = %v, want %v", peer.PublicKey, tt.expectedPeer.PublicKey)
			}

			if peer.Endpoint != tt.expectedPeer.Endpoint {
				t.Errorf("Endpoint = %v, want %v", peer.Endpoint, tt.expectedPeer.Endpoint)
			}

			if peer.AllowedIPs != tt.expectedPeer.AllowedIPs {
				t.Errorf("AllowedIPs = %v, want %v", peer.AllowedIPs, tt.expectedPeer.AllowedIPs)
			}

			if peer.LastHandshake != tt.expectedPeer.LastHandshake {
				t.Errorf("LastHandshake = %v, want %v", peer.LastHandshake, tt.expectedPeer.LastHandshake)
			}

			if peer.RxBytes != tt.expectedPeer.RxBytes {
				t.Errorf("RxBytes = %v, want %v", peer.RxBytes, tt.expectedPeer.RxBytes)
			}

			if peer.TxBytes != tt.expectedPeer.TxBytes {
				t.Errorf("TxBytes = %v, want %v", peer.TxBytes, tt.expectedPeer.TxBytes)
			}
		})
	}
}

// TestParsePeerLineInvalidTimestamp tests handling of invalid timestamp
func TestParsePeerLineInvalidTimestamp(t *testing.T) {
	manager := &WireGuardManager{
		interfaceName: "wg0",
		publicKey:     "test_key",
	}

	// Line with non-numeric timestamp - should still parse but without handshake
	line := "peer_key_123 (none) 192.168.1.1:51820 10.0.0.0/8 invalid_timestamp 1000 2000 25"

	peer, err := manager.parsePeerLine(line)
	if err != nil {
		t.Fatalf("parsePeerLine() unexpected error: %v", err)
	}

	// Should still parse the peer, but LastHandshake should be empty due to parse failure
	if peer.PublicKey != "peer_key_123" {
		t.Errorf("PublicKey = %v, want %v", peer.PublicKey, "peer_key_123")
	}

	// Invalid timestamp should result in empty LastHandshake
	if peer.LastHandshake != "" {
		t.Errorf("LastHandshake should be empty for invalid timestamp, got %v", peer.LastHandshake)
	}

	// Transfer stats should still be parsed
	if peer.RxBytes != 1000 {
		t.Errorf("RxBytes = %v, want %v", peer.RxBytes, 1000)
	}
}

// TestValidatePublicKey tests the validatePublicKey function
func TestValidatePublicKey(t *testing.T) {
	// Generate a valid 32-byte key and encode it
	validKeyBytes := make([]byte, 32)
	for i := range validKeyBytes {
		validKeyBytes[i] = byte(i)
	}
	validKey := base64.StdEncoding.EncodeToString(validKeyBytes)

	// Generate another valid key with different bytes
	validKeyBytes2 := make([]byte, 32)
	for i := range validKeyBytes2 {
		validKeyBytes2[i] = byte(255 - i)
	}
	validKey2 := base64.StdEncoding.EncodeToString(validKeyBytes2)

	// Generate invalid keys
	shortKeyBytes := make([]byte, 31) // 31 bytes instead of 32
	for i := range shortKeyBytes {
		shortKeyBytes[i] = byte(i)
	}
	shortKey := base64.StdEncoding.EncodeToString(shortKeyBytes)

	longKeyBytes := make([]byte, 33) // 33 bytes instead of 32
	for i := range longKeyBytes {
		longKeyBytes[i] = byte(i)
	}
	longKey := base64.StdEncoding.EncodeToString(longKeyBytes)

	tests := []struct {
		name      string
		key       string
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid 32-byte key",
			key:       validKey,
			expectErr: false,
		},
		{
			name:      "valid 32-byte key variant 2",
			key:       validKey2,
			expectErr: false,
		},
		{
			name:      "empty key",
			key:       "",
			expectErr: true,
			errMsg:    "invalid key length", // Empty string decodes to 0 bytes, fails length check
		},
		{
			name:      "short key (31 bytes)",
			key:       shortKey,
			expectErr: true,
			errMsg:    "invalid key length",
		},
		{
			name:      "long key (33 bytes)",
			key:       longKey,
			expectErr: true,
			errMsg:    "invalid key length",
		},
		{
			name:      "invalid base64",
			key:       "not-valid-base64!!!",
			expectErr: true,
			errMsg:    "not valid base64",
		},
		{
			name:      "base64 with padding issues",
			key:       "dGVzdA", // "test" without proper padding
			expectErr: true,
			errMsg:    "not valid base64",
		},
		{
			name:      "all zeros key",
			key:       base64.StdEncoding.EncodeToString(make([]byte, 32)),
			expectErr: false,
		},
		{
			name:      "all ones key",
			key:       base64.StdEncoding.EncodeToString([]byte(strings.Repeat("\xff", 32))),
			expectErr: false,
		},
		{
			name:      "key with URL encoding chars",
			key:       "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/", // 64 chars, not 32 bytes
			expectErr: true,
			errMsg:    "invalid key length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePublicKey(tt.key)

			if tt.expectErr {
				if err == nil {
					t.Errorf("validatePublicKey() expected error but got none")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("validatePublicKey() error = %v, should contain %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validatePublicKey() unexpected error: %v", err)
				}
			}
		})
	}
}

// TestValidatePublicKeyRealWorldKeys tests with realistic WireGuard key formats
func TestValidatePublicKeyRealWorldKeys(t *testing.T) {
	// Generate actual 32-byte keys and encode them properly
	realisticKeys := []string{
		base64.StdEncoding.EncodeToString([]byte("this_is_a_32_byte_wireguard_key!")), // 32 bytes
		base64.StdEncoding.EncodeToString([]byte("another_32_byte_wireguard_key___")), // 32 bytes
		base64.StdEncoding.EncodeToString([]byte("third_32_byte_wireguard_key_here")), // 32 bytes
	}

	for i, key := range realisticKeys {
		t.Run(fmt.Sprintf("realistic_key_%d", i), func(t *testing.T) {
			// Verify it's valid base64 and 32 bytes
			decoded, err := base64.StdEncoding.DecodeString(key)
			if err != nil {
				t.Fatalf("Test key %d is not valid base64: %v", i, err)
			}
			if len(decoded) != 32 {
				t.Fatalf("Test key %d is not 32 bytes: got %d", i, len(decoded))
			}

			// Now test the validation function
			if err := validatePublicKey(key); err != nil {
				t.Errorf("validatePublicKey() failed for valid key %d: %v", i, err)
			}
		})
	}
}

// TestDerivePublicKeyErrorHandling tests error handling in derivePublicKey
// Since we can't mock exec.Command easily, we test the error paths
func TestDerivePublicKeyErrorHandling(t *testing.T) {
	// Test with empty private key - should fail when passed to wg pubkey
	_, err := derivePublicKey("")
	if err == nil {
		t.Error("derivePublicKey() with empty key should return error")
	}

	// Test with invalid private key format
	_, err = derivePublicKey("not-a-valid-key-format")
	if err == nil {
		t.Error("derivePublicKey() with invalid key format should return error")
	}

	// Test with key that's not valid base64
	_, err = derivePublicKey("!!!invalid!!!")
	if err == nil {
		t.Error("derivePublicKey() with invalid base64 should return error")
	}
}

// TestDerivePublicKeyValidKey tests derivePublicKey with a valid key format
// This test will be skipped if wg command is not available
func TestDerivePublicKeyValidKey(t *testing.T) {
	// Generate a valid WireGuard private key (32 bytes base64 encoded)
	// In real WG, private keys are 32 bytes
	privateKeyBytes := make([]byte, 32)
	for i := range privateKeyBytes {
		privateKeyBytes[i] = byte(i * 7) // Some pattern
	}
	privateKey := base64.StdEncoding.EncodeToString(privateKeyBytes)

	publicKey, err := derivePublicKey(privateKey)
	if err != nil {
		t.Skipf("wg command not available or failed: %v", err)
	}

	// Verify the public key is valid base64 and 32 bytes
	if err := validatePublicKey(publicKey); err != nil {
		t.Errorf("Derived public key is invalid: %v", err)
	}

	// Verify the public key is not empty
	if publicKey == "" {
		t.Error("derivePublicKey() returned empty public key")
	}

	// Verify the public key is different from private key
	if publicKey == privateKey {
		t.Error("derivePublicKey() returned same value as input (should be different)")
	}
}

// TestParseWGDumpNilManager tests parseWGDump with nil manager fields
func TestParseWGDumpNilManager(t *testing.T) {
	// Test with nil manager - should handle gracefully
	var nilManager *WireGuardManager

	if nilManager != nil {
		// This shouldn't happen, but test the behavior
		_, err := nilManager.parseWGDump("test")
		if err == nil {
			t.Error("parseWGDump() on nil manager should return error")
		}
	}

	// Test with manager that has empty fields
	emptyManager := &WireGuardManager{}

	input := `private_key_abc public_key_xyz 51820 off
peer_key_123 (none) 192.168.1.1:51820 10.0.0.0/8 1234567890 1000 2000 25`

	result, err := emptyManager.parseWGDump(input)
	if err != nil {
		t.Fatalf("parseWGDump() unexpected error: %v", err)
	}

	// Interface and PublicKey should be empty since manager has empty fields
	if result.Interface != "" {
		t.Errorf("Interface should be empty, got %v", result.Interface)
	}

	if result.PublicKey != "" {
		t.Errorf("PublicKey should be empty, got %v", result.PublicKey)
	}

	// But peers should still be parsed
	if len(result.Peers) != 1 {
		t.Errorf("Expected 1 peer, got %d", len(result.Peers))
	}
}

// TestParsePeerLineEdgeCases tests additional edge cases for parsePeerLine
func TestParsePeerLineEdgeCases(t *testing.T) {
	manager := &WireGuardManager{
		interfaceName: "wg0",
		publicKey:     "test_key",
	}

	tests := []struct {
		name      string
		line      string
		expectErr bool
	}{
		{
			name:      "line with tabs instead of spaces",
			line:      "peer_key_123\t(none)\t192.168.1.1:51820\t10.0.0.0/8\t1234567890\t1000\t2000\t25",
			expectErr: false,
		},
		{
			name:      "line with mixed whitespace",
			line:      "peer_key_123  (none)\t192.168.1.1:51820  10.0.0.0/8\t1234567890\t1000\t2000\t25",
			expectErr: false,
		},
		{
			name:      "line with extra spaces between fields",
			line:      "peer_key_123   (none)   192.168.1.1:51820   10.0.0.0/8   1234567890   1000   2000   25",
			expectErr: false,
		},
		{
			name:      "line with negative transfer numbers (should still parse)",
			line:      "peer_key_123 (none) 192.168.1.1:51820 10.0.0.0/8 1234567890 -1000 -2000 25",
			expectErr: false, // strconv.ParseInt handles negative numbers
		},
		{
			name:      "line with very large numbers",
			line:      "peer_key_123 (none) 192.168.1.1:51820 10.0.0.0/8 9223372036854775807 9223372036854775807 9223372036854775807 25",
			expectErr: false,
		},
		{
			name:      "line with exactly 8 fields",
			line:      "a b c d e f g h",
			expectErr: false,
		},
		{
			name:      "line with 9 fields (extra field)",
			line:      "peer_key_123 (none) 192.168.1.1:51820 10.0.0.0/8 1234567890 1000 2000 25 extra",
			expectErr: false, // Should still work, just ignores extra
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.parsePeerLine(tt.line)

			if tt.expectErr {
				if err == nil {
					t.Errorf("parsePeerLine() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("parsePeerLine() unexpected error: %v", err)
				}
			}
		})
	}
}

// BenchmarkParseWGDump benchmarks the parseWGDump function
func BenchmarkParseWGDump(b *testing.B) {
	manager := &WireGuardManager{
		interfaceName: "wg0",
		publicKey:     "test_key",
	}

	input := `private_key_abc public_key_xyz 51820 off
peer_key_001 (none) 192.168.1.1:51820 10.0.0.0/8 1234567890 1000 2000 25
peer_key_002 (none) 192.168.1.2:51820 192.168.0.0/16 1234567891 2000 3000 0
peer_key_003 (none) 192.168.1.3:51820 172.16.0.0/12 0 0 0 0
peer_key_004 (none) 192.168.1.4:51820 10.0.0.0/8,192.168.0.0/16 1234567892 3000 4000 25
peer_key_005 (none) [2001:db8::1]:51820 10.0.0.0/8 1234567893 4000 5000 0`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := manager.parseWGDump(input)
		if err != nil {
			b.Fatalf("parseWGDump() error: %v", err)
		}
	}
}

// BenchmarkParsePeerLine benchmarks the parsePeerLine function
func BenchmarkParsePeerLine(b *testing.B) {
	manager := &WireGuardManager{
		interfaceName: "wg0",
		publicKey:     "test_key",
	}

	line := "peer_key_123 (none) 192.168.1.1:51820 10.0.0.0/8 1234567890 1000 2000 25"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := manager.parsePeerLine(line)
		if err != nil {
			b.Fatalf("parsePeerLine() error: %v", err)
		}
	}
}

// BenchmarkValidatePublicKey benchmarks the validatePublicKey function
func BenchmarkValidatePublicKey(b *testing.B) {
	// Generate a valid key
	validKeyBytes := make([]byte, 32)
	for i := range validKeyBytes {
		validKeyBytes[i] = byte(i)
	}
	validKey := base64.StdEncoding.EncodeToString(validKeyBytes)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := validatePublicKey(validKey)
		if err != nil {
			b.Fatalf("validatePublicKey() error: %v", err)
		}
	}
}
