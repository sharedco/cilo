// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// This binary is designed to be installed setuid root.
// It provides a minimal interface for WireGuard operations
// that can be called by the unprivileged cilo CLI.

// Allowed operations (whitelist approach for security)
const (
	OpCreateInterface = "create-interface"
	OpDeleteInterface = "delete-interface"
	OpAddPeer         = "add-peer"
	OpRemovePeer      = "remove-peer"
	OpSetKey          = "set-key"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	op := os.Args[1]

	var err error
	switch op {
	case OpCreateInterface:
		err = createInterface(os.Args[2:])
	case OpDeleteInterface:
		err = deleteInterface(os.Args[2:])
	case OpAddPeer:
		err = addPeer(os.Args[2:])
	case OpRemovePeer:
		err = removePeer(os.Args[2:])
	case OpSetKey:
		err = setKey(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown operation: %s\n", op)
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `cilo-wg-helper - WireGuard helper for Cilo

Usage:
  cilo-wg-helper create-interface <name> <address> [listen-port]
  cilo-wg-helper delete-interface <name>
  cilo-wg-helper add-peer <interface> <public-key> <allowed-ips>
  cilo-wg-helper remove-peer <interface> <public-key>
  cilo-wg-helper set-key <interface> <private-key-file>

This binary is designed to be installed setuid root and provides
minimal WireGuard operations for the Cilo CLI.
`)
}

// createInterface creates a new WireGuard interface
// Args: name, address, [listen-port]
func createInterface(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: create-interface <name> <address> [listen-port]")
	}

	name := args[0]
	address := args[1]
	listenPort := "51820"
	if len(args) > 2 {
		listenPort = args[2]
	}

	// Validate inputs
	if !isValidInterfaceName(name) {
		return fmt.Errorf("invalid interface name: %s", name)
	}
	if !isValidAddress(address) {
		return fmt.Errorf("invalid address: %s", address)
	}
	if !isValidPort(listenPort) {
		return fmt.Errorf("invalid port: %s", listenPort)
	}

	// Create interface
	if err := run("ip", "link", "add", "dev", name, "type", "wireguard"); err != nil {
		return fmt.Errorf("failed to create interface: %w", err)
	}

	// Set address
	if err := run("ip", "address", "add", address, "dev", name); err != nil {
		_ = run("ip", "link", "delete", name) // Cleanup on failure
		return fmt.Errorf("failed to set address: %w", err)
	}

	// Set listen port
	if err := run("wg", "set", name, "listen-port", listenPort); err != nil {
		_ = run("ip", "link", "delete", name)
		return fmt.Errorf("failed to set listen port: %w", err)
	}

	// Bring up interface
	if err := run("ip", "link", "set", "up", "dev", name); err != nil {
		_ = run("ip", "link", "delete", name)
		return fmt.Errorf("failed to bring up interface: %w", err)
	}

	fmt.Printf("Created interface %s with address %s\n", name, address)
	return nil
}

func deleteInterface(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: delete-interface <name>")
	}

	name := args[0]
	if !isValidInterfaceName(name) {
		return fmt.Errorf("invalid interface name: %s", name)
	}

	if err := run("ip", "link", "delete", name); err != nil {
		return fmt.Errorf("failed to delete interface: %w", err)
	}

	fmt.Printf("Deleted interface %s\n", name)
	return nil
}

func addPeer(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: add-peer <interface> <public-key> <allowed-ips>")
	}

	iface := args[0]
	pubKey := args[1]
	allowedIPs := args[2]

	if !isValidInterfaceName(iface) {
		return fmt.Errorf("invalid interface name: %s", iface)
	}
	if !isValidBase64Key(pubKey) {
		return fmt.Errorf("invalid public key format")
	}
	if !isValidAllowedIPs(allowedIPs) {
		return fmt.Errorf("invalid allowed IPs: %s", allowedIPs)
	}

	if err := run("wg", "set", iface, "peer", pubKey, "allowed-ips", allowedIPs); err != nil {
		return fmt.Errorf("failed to add peer: %w", err)
	}

	fmt.Printf("Added peer to %s\n", iface)
	return nil
}

func removePeer(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: remove-peer <interface> <public-key>")
	}

	iface := args[0]
	pubKey := args[1]

	if !isValidInterfaceName(iface) {
		return fmt.Errorf("invalid interface name: %s", iface)
	}
	if !isValidBase64Key(pubKey) {
		return fmt.Errorf("invalid public key format")
	}

	if err := run("wg", "set", iface, "peer", pubKey, "remove"); err != nil {
		return fmt.Errorf("failed to remove peer: %w", err)
	}

	fmt.Printf("Removed peer from %s\n", iface)
	return nil
}

func setKey(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: set-key <interface> <private-key-file>")
	}

	iface := args[0]
	keyFile := args[1]

	if !isValidInterfaceName(iface) {
		return fmt.Errorf("invalid interface name: %s", iface)
	}
	if !isValidKeyFilePath(keyFile) {
		return fmt.Errorf("invalid key file path")
	}

	if err := run("wg", "set", iface, "private-key", keyFile); err != nil {
		return fmt.Errorf("failed to set private key: %w", err)
	}

	fmt.Printf("Set private key for %s\n", iface)
	return nil
}

// Validation functions

func isValidInterfaceName(name string) bool {
	// Only allow cilo-prefixed interface names
	matched, _ := regexp.MatchString(`^cilo[0-9]*$`, name)
	return matched
}

func isValidAddress(addr string) bool {
	// Basic CIDR validation (e.g., 10.225.0.1/32)
	matched, _ := regexp.MatchString(`^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+/[0-9]+$`, addr)
	return matched
}

func isValidPort(port string) bool {
	matched, _ := regexp.MatchString(`^[0-9]+$`, port)
	return matched
}

func isValidBase64Key(key string) bool {
	// WireGuard keys are 44 chars base64 (32 bytes + padding)
	matched, _ := regexp.MatchString(`^[A-Za-z0-9+/]{43}=$`, key)
	return matched
}

func isValidAllowedIPs(ips string) bool {
	// Allow comma-separated CIDR notations
	parts := strings.Split(ips, ",")
	for _, part := range parts {
		matched, _ := regexp.MatchString(`^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+/[0-9]+$`, strings.TrimSpace(part))
		if !matched {
			return false
		}
	}
	return true
}

func isValidKeyFilePath(path string) bool {
	// Only allow paths in user's home dir or /tmp for cilo
	return strings.HasPrefix(path, "/home/") ||
		strings.HasPrefix(path, "/Users/") ||
		strings.HasPrefix(path, "/tmp/cilo-")
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
