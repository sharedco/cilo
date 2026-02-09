// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

// Package tunnel provides WireGuard tunnel management for Cilo Cloud.
package tunnel

import (
	"fmt"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// KeyPair represents a WireGuard key pair
type KeyPair struct {
	PrivateKey string
	PublicKey  string
}

// GenerateKeyPair generates a new WireGuard key pair
func GenerateKeyPair() (*KeyPair, error) {
	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("generate private key: %w", err)
	}

	return &KeyPair{
		PrivateKey: privateKey.String(),
		PublicKey:  privateKey.PublicKey().String(),
	}, nil
}

// ParseKey parses a base64-encoded WireGuard key
func ParseKey(key string) (wgtypes.Key, error) {
	return wgtypes.ParseKey(key)
}

// GeneratePresharedKey generates a pre-shared key for additional security
func GeneratePresharedKey() (string, error) {
	psk, err := wgtypes.GenerateKey()
	if err != nil {
		return "", fmt.Errorf("generate PSK: %w", err)
	}
	return psk.String(), nil
}
