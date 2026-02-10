// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cli

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// mockCilodServer creates a mock cilod server for testing
func mockCilodServer(t *testing.T) (*httptest.Server, string) {
	token := "test-session-token-12345"
	serverPublicKey := "server-public-key-wg-test"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/challenge":
			json.NewEncoder(w).Encode(map[string]string{
				"challenge": "test-challenge-nonce",
			})
		case "/auth/connect":
			json.NewEncoder(w).Encode(map[string]string{
				"token": token,
			})
		case "/wireguard/exchange":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"server_public_key":  serverPublicKey,
				"server_endpoint":    "10.225.0.1:51820",
				"assigned_ip":        "10.225.0.2/32",
				"allowed_ips":        []string{"10.224.0.0/16", "10.225.0.0/16"},
				"environment_subnet": "10.224.1.0/24",
			})
		case "/environments":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"environments": []map[string]interface{}{
					{"name": "test-env", "status": "running"},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	return server, token
}

// generateTestSSHKey generates a real RSA key pair for testing
func generateTestSSHKey(t *testing.T, sshDir string) string {
	privKeyPath := filepath.Join(sshDir, "id_rsa")
	pubKeyPath := privKeyPath + ".pub"

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}

	privFile, err := os.Create(privKeyPath)
	if err != nil {
		t.Fatalf("failed to create private key file: %v", err)
	}
	defer privFile.Close()

	if err := pem.Encode(privFile, privateKeyPEM); err != nil {
		t.Fatalf("failed to encode private key: %v", err)
	}
	privFile.Chmod(0600)

	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("failed to generate SSH public key: %v", err)
	}

	pubKeyData := ssh.MarshalAuthorizedKey(publicKey)
	if err := os.WriteFile(pubKeyPath, pubKeyData, 0644); err != nil {
		t.Fatalf("failed to write public key: %v", err)
	}

	return privKeyPath
}

// TestConnectNewMachine tests connecting to a new host and registering in machines state
func TestConnectNewMachine(t *testing.T) {
	server, expectedToken := mockCilodServer(t)
	defer server.Close()

	machinesDir := t.TempDir()

	sshDir := filepath.Join(t.TempDir(), ".ssh")
	os.MkdirAll(sshDir, 0700)
	privateKeyPath := generateTestSSHKey(t, sshDir)

	originalGetMachinesDir := getMachinesDir
	getMachinesDir = func() string { return machinesDir }
	defer func() { getMachinesDir = originalGetMachinesDir }()

	host := server.Listener.Addr().String()
	machine, err := connectMachine(host, privateKeyPath)
	if err != nil {
		t.Fatalf("connectMachine failed: %v", err)
	}

	if machine.Host != host {
		t.Errorf("expected host %s, got %s", host, machine.Host)
	}
	if machine.Token != expectedToken {
		t.Errorf("expected token %s, got %s", expectedToken, machine.Token)
	}
	if machine.WGPrivateKey == "" {
		t.Error("expected WireGuard private key to be set")
	}
	if machine.WGPublicKey == "" {
		t.Error("expected WireGuard public key to be set")
	}
	if machine.WGServerPublicKey == "" {
		t.Error("expected WireGuard server public key to be set")
	}
	if machine.ConnectedAt.IsZero() {
		t.Error("expected ConnectedAt to be set")
	}

	statePath := filepath.Join(machinesDir, sanitizeHost(host), "state.json")
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Errorf("state.json was not created at %s", statePath)
	}

	wgKeyPath := filepath.Join(machinesDir, sanitizeHost(host), "wg-key")
	if _, err := os.Stat(wgKeyPath); os.IsNotExist(err) {
		t.Errorf("wg-key was not created at %s", wgKeyPath)
	}

	machines, err := ListConnectedMachines()
	if err != nil {
		t.Fatalf("ListConnectedMachines failed: %v", err)
	}
	found := false
	for _, m := range machines {
		if m.Host == host {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("machine %s not found in connected machines list", host)
	}
}

// TestConnectAlreadyConnected tests that connecting to an already connected machine returns an error
func TestConnectAlreadyConnected(t *testing.T) {
	server, _ := mockCilodServer(t)
	defer server.Close()

	machinesDir := t.TempDir()

	sshDir := filepath.Join(t.TempDir(), ".ssh")
	os.MkdirAll(sshDir, 0700)
	privateKeyPath := generateTestSSHKey(t, sshDir)

	originalGetMachinesDir := getMachinesDir
	getMachinesDir = func() string { return machinesDir }
	defer func() { getMachinesDir = originalGetMachinesDir }()

	host := server.Listener.Addr().String()

	_, err := connectMachine(host, privateKeyPath)
	if err != nil {
		t.Fatalf("first connect failed: %v", err)
	}

	_, err = connectMachine(host, privateKeyPath)
	if err == nil {
		t.Fatal("expected error for already connected machine, got nil")
	}
	if err.Error() != "machine already connected: "+host {
		t.Errorf("expected 'already connected' error, got: %v", err)
	}
}

// TestConnectUnreachable tests that connecting to an unreachable host fails gracefully
func TestConnectUnreachable(t *testing.T) {
	machinesDir := t.TempDir()

	originalGetMachinesDir := getMachinesDir
	getMachinesDir = func() string { return machinesDir }
	defer func() { getMachinesDir = originalGetMachinesDir }()

	sshDir := filepath.Join(t.TempDir(), ".ssh")
	os.MkdirAll(sshDir, 0700)
	privateKeyPath := generateTestSSHKey(t, sshDir)

	_, err := connectMachine("nonexistent.invalid:99999", privateKeyPath)
	if err == nil {
		t.Fatal("expected error for unreachable host, got nil")
	}
}

// TestDisconnectConnected tests disconnecting from a connected machine
func TestDisconnectConnected(t *testing.T) {
	server, _ := mockCilodServer(t)
	defer server.Close()

	machinesDir := t.TempDir()

	sshDir := filepath.Join(t.TempDir(), ".ssh")
	os.MkdirAll(sshDir, 0700)
	privateKeyPath := generateTestSSHKey(t, sshDir)

	originalGetMachinesDir := getMachinesDir
	getMachinesDir = func() string { return machinesDir }
	defer func() { getMachinesDir = originalGetMachinesDir }()

	host := server.Listener.Addr().String()

	_, err := connectMachine(host, privateKeyPath)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	machineDir := filepath.Join(machinesDir, sanitizeHost(host))
	if _, err := os.Stat(machineDir); os.IsNotExist(err) {
		t.Fatal("machine directory should exist after connect")
	}

	err = disconnectMachine(host)
	if err != nil {
		t.Fatalf("disconnect failed: %v", err)
	}

	if _, err := os.Stat(machineDir); !os.IsNotExist(err) {
		t.Error("machine directory should be removed after disconnect")
	}

	machines, err := ListConnectedMachines()
	if err != nil {
		t.Fatalf("ListConnectedMachines failed: %v", err)
	}
	for _, m := range machines {
		if m.Host == host {
			t.Error("machine should not be in connected machines list after disconnect")
		}
	}
}

// TestDisconnectNotConnected tests that disconnecting from a non-connected machine returns an error
func TestDisconnectNotConnected(t *testing.T) {
	machinesDir := t.TempDir()

	originalGetMachinesDir := getMachinesDir
	getMachinesDir = func() string { return machinesDir }
	defer func() { getMachinesDir = originalGetMachinesDir }()

	err := disconnectMachine("never-connected.example.com")
	if err == nil {
		t.Fatal("expected error for non-connected machine, got nil")
	}
	expectedErr := "machine not connected: never-connected.example.com"
	if err.Error() != expectedErr {
		t.Errorf("expected error '%s', got '%v'", expectedErr, err)
	}
}

// TestMachineStateStorage tests the machine state storage functions
func TestMachineStateStorage(t *testing.T) {
	machinesDir := t.TempDir()

	originalGetMachinesDir := getMachinesDir
	getMachinesDir = func() string { return machinesDir }
	defer func() { getMachinesDir = originalGetMachinesDir }()

	machine := &Machine{
		Host:              "test-host.example.com",
		Token:             "test-token",
		WGPrivateKey:      "wg-private-key",
		WGPublicKey:       "wg-public-key",
		WGServerPublicKey: "wg-server-public-key",
		WGAssignedIP:      "10.225.0.2/32",
		WGEndpoint:        "10.225.0.1:51820",
		ConnectedAt:       time.Now(),
		Version:           1,
	}

	err := SaveMachine(machine)
	if err != nil {
		t.Fatalf("SaveMachine failed: %v", err)
	}

	retrieved, err := GetMachine("test-host.example.com")
	if err != nil {
		t.Fatalf("GetMachine failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetMachine returned nil")
	}
	if retrieved.Host != machine.Host {
		t.Errorf("expected host %s, got %s", machine.Host, retrieved.Host)
	}
	if retrieved.Token != machine.Token {
		t.Errorf("expected token %s, got %s", machine.Token, retrieved.Token)
	}

	machines, err := ListConnectedMachines()
	if err != nil {
		t.Fatalf("ListConnectedMachines failed: %v", err)
	}
	if len(machines) != 1 {
		t.Errorf("expected 1 machine, got %d", len(machines))
	}

	err = RemoveMachine("test-host.example.com")
	if err != nil {
		t.Fatalf("RemoveMachine failed: %v", err)
	}

	retrieved, err = GetMachine("test-host.example.com")
	if err != nil {
		t.Fatalf("GetMachine after removal failed: %v", err)
	}
	if retrieved != nil {
		t.Error("GetMachine should return nil after removal")
	}
}

// TestSanitizeHost tests the host sanitization function
func TestSanitizeHost(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"localhost", "localhost"},
		{"localhost:8080", "localhost_8080"},
		{"192.168.1.1", "192.168.1.1"},
		{"192.168.1.1:8080", "192.168.1.1_8080"},
		{"host.example.com", "host.example.com"},
		{"host.example.com:443", "host.example.com_443"},
	}

	for _, test := range tests {
		result := sanitizeHost(test.input)
		if result != test.expected {
			t.Errorf("sanitizeHost(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

// TestIsConnected tests the IsConnected function
func TestIsConnected(t *testing.T) {
	machinesDir := t.TempDir()

	originalGetMachinesDir := getMachinesDir
	getMachinesDir = func() string { return machinesDir }
	defer func() { getMachinesDir = originalGetMachinesDir }()

	if IsConnected("test-host.example.com") {
		t.Error("expected IsConnected to return false for non-existent machine")
	}

	machine := &Machine{
		Host:        "test-host.example.com",
		Token:       "test-token",
		ConnectedAt: time.Now(),
		Version:     1,
	}
	SaveMachine(machine)

	if !IsConnected("test-host.example.com") {
		t.Error("expected IsConnected to return true for saved machine")
	}
}

// TestDisconnectAll tests disconnecting all machines
func TestDisconnectAll(t *testing.T) {
	server, _ := mockCilodServer(t)
	defer server.Close()

	machinesDir := t.TempDir()

	sshDir := filepath.Join(t.TempDir(), ".ssh")
	os.MkdirAll(sshDir, 0700)
	privateKeyPath := generateTestSSHKey(t, sshDir)

	originalGetMachinesDir := getMachinesDir
	getMachinesDir = func() string { return machinesDir }
	defer func() { getMachinesDir = originalGetMachinesDir }()

	host := server.Listener.Addr().String()

	_, err := connectMachine(host, privateKeyPath)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	err = disconnectAllMachines()
	if err != nil {
		t.Fatalf("disconnectAllMachines failed: %v", err)
	}

	machines, err := ListConnectedMachines()
	if err != nil {
		t.Fatalf("ListConnectedMachines failed: %v", err)
	}
	if len(machines) != 0 {
		t.Errorf("expected 0 machines after disconnect all, got %d", len(machines))
	}
}

// TestEmptyMachinesList tests listing machines when none exist
func TestEmptyMachinesList(t *testing.T) {
	machinesDir := t.TempDir()

	originalGetMachinesDir := getMachinesDir
	getMachinesDir = func() string { return machinesDir }
	defer func() { getMachinesDir = originalGetMachinesDir }()

	machines, err := ListConnectedMachines()
	if err != nil {
		t.Fatalf("ListConnectedMachines failed: %v", err)
	}
	if len(machines) != 0 {
		t.Errorf("expected 0 machines, got %d", len(machines))
	}
}

// TestMachineWithPort tests machine operations with port in hostname
func TestMachineWithPort(t *testing.T) {
	machinesDir := t.TempDir()

	originalGetMachinesDir := getMachinesDir
	getMachinesDir = func() string { return machinesDir }
	defer func() { getMachinesDir = originalGetMachinesDir }()

	machine := &Machine{
		Host:              "localhost:9090",
		Token:             "port-token",
		WGPrivateKey:      "wg-private",
		WGPublicKey:       "wg-public",
		WGServerPublicKey: "wg-server",
		WGAssignedIP:      "10.225.0.4/32",
		WGEndpoint:        "10.225.0.1:51820",
		ConnectedAt:       time.Now(),
		Version:           1,
	}

	err := SaveMachine(machine)
	if err != nil {
		t.Fatalf("SaveMachine failed: %v", err)
	}

	sanitizedDir := filepath.Join(machinesDir, "localhost_9090")
	if _, err := os.Stat(sanitizedDir); os.IsNotExist(err) {
		t.Error("directory with sanitized name should exist")
	}

	retrieved, err := GetMachine("localhost:9090")
	if err != nil {
		t.Fatalf("GetMachine failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetMachine returned nil")
	}
	if retrieved.Host != "localhost:9090" {
		t.Errorf("expected host localhost:9090, got %s", retrieved.Host)
	}
}

// TestMachineStateCorruption tests handling of corrupted state files
func TestMachineStateCorruption(t *testing.T) {
	machinesDir := t.TempDir()

	originalGetMachinesDir := getMachinesDir
	getMachinesDir = func() string { return machinesDir }
	defer func() { getMachinesDir = originalGetMachinesDir }()

	hostDir := filepath.Join(machinesDir, "corrupted-host")
	os.MkdirAll(hostDir, 0755)
	statePath := filepath.Join(hostDir, "state.json")
	os.WriteFile(statePath, []byte("not-valid-json"), 0644)

	machines, err := ListConnectedMachines()
	if err != nil {
		t.Fatalf("ListConnectedMachines should handle corrupted files gracefully: %v", err)
	}

	for _, m := range machines {
		if m.Host == "corrupted-host" {
			t.Error("corrupted machine should not appear in list")
		}
	}
}

// TestMachineStatePermissions tests file permissions on machine state
func TestMachineStatePermissions(t *testing.T) {
	machinesDir := t.TempDir()

	originalGetMachinesDir := getMachinesDir
	getMachinesDir = func() string { return machinesDir }
	defer func() { getMachinesDir = originalGetMachinesDir }()

	machine := &Machine{
		Host:              "perms-test.example.com",
		Token:             "secret-token",
		WGPrivateKey:      "secret-private-key",
		WGPublicKey:       "public-key",
		WGServerPublicKey: "server-key",
		WGAssignedIP:      "10.225.0.5/32",
		WGEndpoint:        "10.225.0.1:51820",
		ConnectedAt:       time.Now(),
		Version:           1,
	}

	err := SaveMachine(machine)
	if err != nil {
		t.Fatalf("SaveMachine failed: %v", err)
	}

	statePath := filepath.Join(machinesDir, "perms-test.example.com", "state.json")
	info, err := os.Stat(statePath)
	if err != nil {
		t.Fatalf("failed to stat state file: %v", err)
	}

	mode := info.Mode().Perm()
	if mode != 0600 {
		t.Errorf("expected permissions 0600, got %04o", mode)
	}
}

// TestMachineAllowedIPs tests that AllowedIPs are stored correctly
func TestMachineAllowedIPs(t *testing.T) {
	machinesDir := t.TempDir()

	originalGetMachinesDir := getMachinesDir
	getMachinesDir = func() string { return machinesDir }
	defer func() { getMachinesDir = originalGetMachinesDir }()

	machine := &Machine{
		Host:              "allowed-ips.example.com",
		Token:             "token",
		WGPrivateKey:      "private",
		WGPublicKey:       "public",
		WGServerPublicKey: "server",
		WGAssignedIP:      "10.225.0.9/32",
		WGEndpoint:        "10.225.0.1:51820",
		WGAllowedIPs:      []string{"10.224.0.0/16", "10.225.0.0/16", "10.226.0.0/16"},
		ConnectedAt:       time.Now(),
		Version:           1,
	}

	err := SaveMachine(machine)
	if err != nil {
		t.Fatalf("SaveMachine failed: %v", err)
	}

	retrieved, err := GetMachine("allowed-ips.example.com")
	if err != nil {
		t.Fatalf("GetMachine failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetMachine returned nil")
	}

	if len(retrieved.WGAllowedIPs) != 3 {
		t.Errorf("expected 3 allowed IPs, got %d", len(retrieved.WGAllowedIPs))
	}
}

// TestMachineEnvironmentSubnet tests that EnvironmentSubnet is stored correctly
func TestMachineEnvironmentSubnet(t *testing.T) {
	machinesDir := t.TempDir()

	originalGetMachinesDir := getMachinesDir
	getMachinesDir = func() string { return machinesDir }
	defer func() { getMachinesDir = originalGetMachinesDir }()

	machine := &Machine{
		Host:              "subnet-test.example.com",
		Token:             "token",
		WGPrivateKey:      "private",
		WGPublicKey:       "public",
		WGServerPublicKey: "server",
		WGAssignedIP:      "10.225.0.11/32",
		WGEndpoint:        "10.225.0.1:51820",
		WGAllowedIPs:      []string{"10.224.0.0/16"},
		EnvironmentSubnet: "10.224.1.0/24",
		ConnectedAt:       time.Now(),
		Version:           1,
	}

	err := SaveMachine(machine)
	if err != nil {
		t.Fatalf("SaveMachine failed: %v", err)
	}

	retrieved, err := GetMachine("subnet-test.example.com")
	if err != nil {
		t.Fatalf("GetMachine failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetMachine returned nil")
	}

	if retrieved.EnvironmentSubnet != "10.224.1.0/24" {
		t.Errorf("expected subnet '10.224.1.0/24', got '%s'", retrieved.EnvironmentSubnet)
	}
}

// TestMachineStateUpdate tests updating an existing machine
func TestMachineStateUpdate(t *testing.T) {
	machinesDir := t.TempDir()

	originalGetMachinesDir := getMachinesDir
	getMachinesDir = func() string { return machinesDir }
	defer func() { getMachinesDir = originalGetMachinesDir }()

	host := "update-test.example.com"

	machine1 := &Machine{
		Host:              host,
		Token:             "token1",
		WGPrivateKey:      "private1",
		WGPublicKey:       "public1",
		WGServerPublicKey: "server1",
		WGAssignedIP:      "10.225.0.15/32",
		WGEndpoint:        "10.225.0.1:51820",
		ConnectedAt:       time.Now(),
		Version:           1,
	}

	err := SaveMachine(machine1)
	if err != nil {
		t.Fatalf("first SaveMachine failed: %v", err)
	}

	machine2 := &Machine{
		Host:              host,
		Token:             "token2",
		WGPrivateKey:      "private2",
		WGPublicKey:       "public2",
		WGServerPublicKey: "server2",
		WGAssignedIP:      "10.225.0.16/32",
		WGEndpoint:        "10.225.0.1:51821",
		ConnectedAt:       time.Now(),
		Version:           1,
	}

	err = SaveMachine(machine2)
	if err != nil {
		t.Fatalf("second SaveMachine failed: %v", err)
	}

	retrieved, err := GetMachine(host)
	if err != nil {
		t.Fatalf("GetMachine failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetMachine returned nil")
	}
	if retrieved.Token != "token2" {
		t.Errorf("expected token 'token2', got '%s'", retrieved.Token)
	}
	if retrieved.WGPrivateKey != "private2" {
		t.Errorf("expected private key 'private2', got '%s'", retrieved.WGPrivateKey)
	}
}

// TestResolveHostWithPort tests the host resolution function
func TestResolveHostWithPort(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"localhost", "localhost:8080"},
		{"localhost:9090", "localhost:9090"},
		{"192.168.1.1", "192.168.1.1:8080"},
		{"192.168.1.1:9090", "192.168.1.1:9090"},
		{"host.example.com", "host.example.com:8080"},
		{"host.example.com:443", "host.example.com:443"},
	}

	for _, test := range tests {
		result := resolveHostWithPort(test.input)
		if result != test.expected {
			t.Errorf("resolveHostWithPort(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

// TestFindSSHKey tests finding SSH keys
func TestFindSSHKey(t *testing.T) {
	sshDir := filepath.Join(t.TempDir(), ".ssh")
	os.MkdirAll(sshDir, 0700)
	generateTestSSHKey(t, sshDir)

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", filepath.Dir(sshDir))
	defer os.Setenv("HOME", originalHome)

	pubKey, privKey, err := findSSHKey()
	if err != nil {
		t.Fatalf("findSSHKey failed: %v", err)
	}
	if pubKey == "" {
		t.Error("expected public key path, got empty")
	}
	if privKey == "" {
		t.Error("expected private key path, got empty")
	}
}

// TestFindSSHKeyNotFound tests finding SSH keys when none exist
func TestFindSSHKeyNotFound(t *testing.T) {
	sshDir := filepath.Join(t.TempDir(), ".ssh")
	os.MkdirAll(sshDir, 0700)

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", filepath.Dir(sshDir))
	defer os.Setenv("HOME", originalHome)

	_, _, err := findSSHKey()
	if err == nil {
		t.Error("expected error when no SSH key found")
	}
}

// TestMachineStateJSONMarshal tests JSON marshaling of machine state
func TestMachineStateJSONMarshal(t *testing.T) {
	machine := &Machine{
		Host:              "test.example.com",
		Token:             "token123",
		WGPrivateKey:      "private-key",
		WGPublicKey:       "public-key",
		WGServerPublicKey: "server-key",
		WGAssignedIP:      "10.225.0.2/32",
		WGEndpoint:        "10.225.0.1:51820",
		ConnectedAt:       time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
		Version:           1,
	}

	data, err := json.Marshal(machine)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var unmarshaled Machine
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if unmarshaled.Host != machine.Host {
		t.Errorf("host mismatch: %s vs %s", unmarshaled.Host, machine.Host)
	}
	if unmarshaled.Token != machine.Token {
		t.Errorf("token mismatch: %s vs %s", unmarshaled.Token, machine.Token)
	}
}

// TestConcurrentMachineAccess tests concurrent access to machine state
func TestConcurrentMachineAccess(t *testing.T) {
	machinesDir := t.TempDir()

	originalGetMachinesDir := getMachinesDir
	getMachinesDir = func() string { return machinesDir }
	defer func() { getMachinesDir = originalGetMachinesDir }()

	done := make(chan bool, 3)

	for i := 0; i < 3; i++ {
		go func(idx int) {
			machine := &Machine{
				Host:              fmt.Sprintf("concurrent-%d.example.com", idx),
				Token:             fmt.Sprintf("token-%d", idx),
				WGPrivateKey:      fmt.Sprintf("private-%d", idx),
				WGPublicKey:       fmt.Sprintf("public-%d", idx),
				WGServerPublicKey: fmt.Sprintf("server-%d", idx),
				WGAssignedIP:      fmt.Sprintf("10.225.0.%d/32", idx+10),
				WGEndpoint:        "10.225.0.1:51820",
				ConnectedAt:       time.Now(),
				Version:           1,
			}
			SaveMachine(machine)
			done <- true
		}(i)
	}

	for i := 0; i < 3; i++ {
		<-done
	}

	machines, err := ListConnectedMachines()
	if err != nil {
		t.Fatalf("ListConnectedMachines failed: %v", err)
	}
	if len(machines) != 3 {
		t.Errorf("expected 3 machines, got %d", len(machines))
	}
}
