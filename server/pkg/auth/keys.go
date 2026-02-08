package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	// Key prefix for identification
	KeyPrefix = "cilo_"
	// Length of the random part (before base64)
	KeyRandomBytes = 32
)

// GenerateAPIKey generates a new API key and returns the key and its hash
func GenerateAPIKey() (key string, hash string, prefix string, err error) {
	// Generate random bytes
	randomBytes := make([]byte, KeyRandomBytes)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", "", "", fmt.Errorf("generate random bytes: %w", err)
	}

	// Encode as base64 and create full key
	randomPart := base64.RawURLEncoding.EncodeToString(randomBytes)
	key = KeyPrefix + randomPart

	// Extract prefix for lookup (first 12 chars after "cilo_")
	prefix = key[:len(KeyPrefix)+8]

	// Hash the key
	hashBytes, err := bcrypt.GenerateFromPassword([]byte(key), bcrypt.DefaultCost)
	if err != nil {
		return "", "", "", fmt.Errorf("hash key: %w", err)
	}
	hash = string(hashBytes)

	return key, hash, prefix, nil
}

// ValidateAPIKey checks if a key matches a hash
func ValidateAPIKey(key, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(key))
	return err == nil
}

// ExtractPrefix extracts the prefix from an API key for lookup
func ExtractPrefix(key string) string {
	if len(key) < len(KeyPrefix)+8 {
		return ""
	}
	return key[:len(KeyPrefix)+8]
}

// IsValidKeyFormat checks if a key has the correct format
func IsValidKeyFormat(key string) bool {
	return strings.HasPrefix(key, KeyPrefix) && len(key) > len(KeyPrefix)+8
}
