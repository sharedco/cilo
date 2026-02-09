// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: BUSL-1.1
// See LICENSES/BUSL-1.1.txt and LICENSE.enterprise for full license text

package wireguard

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/curve25519"
)

// KeyPair represents a WireGuard public/private key pair
type KeyPair struct {
	PrivateKey string
	PublicKey  string
}

// GenerateKeyPair generates a new WireGuard key pair
// WireGuard uses Curve25519 for key generation
func GenerateKeyPair() (*KeyPair, error) {
	// Generate 32 random bytes for private key
	privateKeyBytes := make([]byte, 32)
	if _, err := rand.Read(privateKeyBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Clamp the private key (WireGuard requirement)
	// Set bits: 0, 1, 2 = 0; bit 254 = 1; bit 255 = 0
	privateKeyBytes[0] &= 248
	privateKeyBytes[31] &= 127
	privateKeyBytes[31] |= 64

	// Derive public key from private key using Curve25519
	publicKeyBytes, err := curve25519.X25519(privateKeyBytes, curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("failed to derive public key: %w", err)
	}

	// Encode keys to base64
	privateKey := base64.StdEncoding.EncodeToString(privateKeyBytes)
	publicKey := base64.StdEncoding.EncodeToString(publicKeyBytes)

	return &KeyPair{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

// DerivePublicKey derives public key from private key
// Useful for validating or recovering public keys from private keys
func DerivePublicKey(privateKey string) (string, error) {
	// Decode base64 private key
	privateKeyBytes, err := base64.StdEncoding.DecodeString(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to decode private key: %w", err)
	}

	// Validate private key length
	if len(privateKeyBytes) != 32 {
		return "", fmt.Errorf("invalid private key length: expected 32 bytes, got %d", len(privateKeyBytes))
	}

	// Derive public key using Curve25519
	publicKeyBytes, err := curve25519.X25519(privateKeyBytes, curve25519.Basepoint)
	if err != nil {
		return "", fmt.Errorf("failed to derive public key: %w", err)
	}

	// Encode public key to base64
	publicKey := base64.StdEncoding.EncodeToString(publicKeyBytes)
	return publicKey, nil
}

// ValidateKeyPair validates that a public key matches a private key
func ValidateKeyPair(privateKey, publicKey string) (bool, error) {
	derivedPublicKey, err := DerivePublicKey(privateKey)
	if err != nil {
		return false, err
	}
	return derivedPublicKey == publicKey, nil
}
