package tunnel

import (
	"encoding/base64"
	"strings"
	"testing"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func TestGenerateKeyPair(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "generates valid key pair",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyPair, err := GenerateKeyPair()
			if err != nil {
				t.Fatalf("GenerateKeyPair() error = %v", err)
			}

			// Verify key pair is not nil
			if keyPair == nil {
				t.Fatal("GenerateKeyPair() returned nil key pair")
			}

			// Verify private key is not empty
			if keyPair.PrivateKey == "" {
				t.Error("GenerateKeyPair() returned empty private key")
			}

			// Verify public key is not empty
			if keyPair.PublicKey == "" {
				t.Error("GenerateKeyPair() returned empty public key")
			}

			// Verify private key is valid base64
			privateKeyBytes, err := base64.StdEncoding.DecodeString(keyPair.PrivateKey)
			if err != nil {
				t.Errorf("Private key is not valid base64: %v", err)
			}

			// WireGuard private keys are 32 bytes
			if len(privateKeyBytes) != 32 {
				t.Errorf("Private key length = %d, want 32", len(privateKeyBytes))
			}

			// Verify public key is valid base64
			publicKeyBytes, err := base64.StdEncoding.DecodeString(keyPair.PublicKey)
			if err != nil {
				t.Errorf("Public key is not valid base64: %v", err)
			}

			// WireGuard public keys are 32 bytes
			if len(publicKeyBytes) != 32 {
				t.Errorf("Public key length = %d, want 32", len(publicKeyBytes))
			}

			// Verify keys are different (private != public)
			if keyPair.PrivateKey == keyPair.PublicKey {
				t.Error("Private key and public key should be different")
			}

			// Verify the public key corresponds to the private key
			// by parsing the private key and deriving its public key
			privateKey, err := wgtypes.ParseKey(keyPair.PrivateKey)
			if err != nil {
				t.Fatalf("Failed to parse private key: %v", err)
			}

			derivedPublicKey := privateKey.PublicKey()
			if derivedPublicKey.String() != keyPair.PublicKey {
				t.Error("Public key does not match the private key's derived public key")
			}
		})
	}
}

func TestGenerateKeyPair_Unique(t *testing.T) {
	// Generate multiple key pairs and ensure they are unique
	keyPairs := make(map[string]bool)
	numKeys := 10

	for i := 0; i < numKeys; i++ {
		keyPair, err := GenerateKeyPair()
		if err != nil {
			t.Fatalf("GenerateKeyPair() iteration %d error = %v", i, err)
		}

		// Check private key uniqueness
		if keyPairs[keyPair.PrivateKey] {
			t.Errorf("Duplicate private key generated at iteration %d", i)
		}
		keyPairs[keyPair.PrivateKey] = true

		// Check public key uniqueness
		if keyPairs[keyPair.PublicKey] {
			t.Errorf("Duplicate public key generated at iteration %d", i)
		}
		keyPairs[keyPair.PublicKey] = true
	}

	// Verify we have the expected number of unique keys (2 per key pair)
	if len(keyPairs) != numKeys*2 {
		t.Errorf("Expected %d unique keys, got %d", numKeys*2, len(keyPairs))
	}
}

func TestParseKey(t *testing.T) {
	// First generate a valid key to use in tests
	validKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate test key pair: %v", err)
	}

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "valid private key",
			key:     validKeyPair.PrivateKey,
			wantErr: false,
		},
		{
			name:    "valid public key",
			key:     validKeyPair.PublicKey,
			wantErr: false,
		},
		{
			name:    "empty key",
			key:     "",
			wantErr: true,
		},
		{
			name:    "invalid base64",
			key:     "not-valid-base64!!!",
			wantErr: true,
		},
		{
			name:    "wrong length - too short",
			key:     base64.StdEncoding.EncodeToString([]byte("short")),
			wantErr: true,
		},
		{
			name:    "wrong length - too long",
			key:     base64.StdEncoding.EncodeToString(make([]byte, 64)),
			wantErr: true,
		},
		{
			name:    "valid base64 but wrong length (31 bytes)",
			key:     base64.StdEncoding.EncodeToString(make([]byte, 31)),
			wantErr: true,
		},
		{
			name:    "valid base64 but wrong length (33 bytes)",
			key:     base64.StdEncoding.EncodeToString(make([]byte, 33)),
			wantErr: true},
		{
			name:    "key with padding issues",
			key:     strings.TrimRight(validKeyPair.PublicKey, "="),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedKey, err := ParseKey(tt.key)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseKey() error = nil, wantErr = true")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseKey() error = %v, wantErr = false", err)
				return
			}

			// Verify the parsed key can be converted back to string
			if parsedKey.String() != tt.key {
				t.Errorf("ParseKey() round-trip failed: got %s, want %s", parsedKey.String(), tt.key)
			}

			// Verify key length is 32 bytes
			if len(parsedKey[:]) != 32 {
				t.Errorf("Parsed key length = %d, want 32", len(parsedKey[:]))
			}
		})
	}
}

func TestParseKey_Deterministic(t *testing.T) {
	// Test that parsing the same key multiple times produces the same result
	validKeyPair, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate test key pair: %v", err)
	}

	key1, err := ParseKey(validKeyPair.PublicKey)
	if err != nil {
		t.Fatalf("First ParseKey() failed: %v", err)
	}

	key2, err := ParseKey(validKeyPair.PublicKey)
	if err != nil {
		t.Fatalf("Second ParseKey() failed: %v", err)
	}

	if key1.String() != key2.String() {
		t.Error("ParseKey() should produce deterministic results for same input")
	}

	// Verify the byte representations are identical
	for i := 0; i < 32; i++ {
		if key1[i] != key2[i] {
			t.Errorf("Key bytes differ at index %d: %d vs %d", i, key1[i], key2[i])
		}
	}
}

func TestGeneratePresharedKey(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "generates valid preshared key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			psk, err := GeneratePresharedKey()
			if err != nil {
				t.Fatalf("GeneratePresharedKey() error = %v", err)
			}

			// Verify PSK is not empty
			if psk == "" {
				t.Error("GeneratePresharedKey() returned empty key")
			}

			// Verify PSK is valid base64
			pskBytes, err := base64.StdEncoding.DecodeString(psk)
			if err != nil {
				t.Errorf("Preshared key is not valid base64: %v", err)
			}

			// WireGuard keys are 32 bytes
			if len(pskBytes) != 32 {
				t.Errorf("Preshared key length = %d, want 32", len(pskBytes))
			}

			// Verify the key can be parsed by wgtypes
			parsedKey, err := wgtypes.ParseKey(psk)
			if err != nil {
				t.Errorf("Generated PSK cannot be parsed by wgtypes: %v", err)
			}

			// Verify round-trip
			if parsedKey.String() != psk {
				t.Error("PSK round-trip failed")
			}
		})
	}
}

func TestGeneratePresharedKey_Unique(t *testing.T) {
	// Generate multiple PSKs and ensure they are unique
	psks := make(map[string]bool)
	numKeys := 10

	for i := 0; i < numKeys; i++ {
		psk, err := GeneratePresharedKey()
		if err != nil {
			t.Fatalf("GeneratePresharedKey() iteration %d error = %v", i, err)
		}

		if psks[psk] {
			t.Errorf("Duplicate PSK generated at iteration %d", i)
		}
		psks[psk] = true
	}

	if len(psks) != numKeys {
		t.Errorf("Expected %d unique PSKs, got %d", numKeys, len(psks))
	}
}

func TestGeneratePresharedKey_CanBeUsedAsKey(t *testing.T) {
	// Verify that generated PSK can be used with ParseKey
	psk, err := GeneratePresharedKey()
	if err != nil {
		t.Fatalf("GeneratePresharedKey() error = %v", err)
	}

	// Should be parseable by our ParseKey function
	parsedKey, err := ParseKey(psk)
	if err != nil {
		t.Errorf("Generated PSK cannot be parsed by ParseKey(): %v", err)
	}

	// Verify it has the correct properties
	if len(parsedKey[:]) != 32 {
		t.Errorf("Parsed PSK length = %d, want 32", len(parsedKey[:]))
	}
}

func TestKeyPair_StructFields(t *testing.T) {
	// Test that KeyPair struct fields are properly accessible
	keyPair := &KeyPair{
		PrivateKey: "test-private-key",
		PublicKey:  "test-public-key",
	}

	if keyPair.PrivateKey != "test-private-key" {
		t.Error("KeyPair.PrivateKey field not accessible")
	}

	if keyPair.PublicKey != "test-public-key" {
		t.Error("KeyPair.PublicKey field not accessible")
	}
}

// Benchmark tests

func BenchmarkGenerateKeyPair(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := GenerateKeyPair()
		if err != nil {
			b.Fatalf("GenerateKeyPair() error = %v", err)
		}
	}
}

func BenchmarkGeneratePresharedKey(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := GeneratePresharedKey()
		if err != nil {
			b.Fatalf("GeneratePresharedKey() error = %v", err)
		}
	}
}

func BenchmarkParseKey(b *testing.B) {
	// Generate a valid key for benchmarking
	keyPair, err := GenerateKeyPair()
	if err != nil {
		b.Fatalf("Failed to generate test key: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseKey(keyPair.PublicKey)
		if err != nil {
			b.Fatalf("ParseKey() error = %v", err)
		}
	}
}
