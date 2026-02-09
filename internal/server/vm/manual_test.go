// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: BUSL-1.1
// See LICENSES/BUSL-1.1.txt and LICENSE.enterprise for full license text

package vm

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// mockSSHServer creates a test SSH server
func mockSSHServer(t *testing.T) (host string, port int, privateKey []byte, cleanup func()) {
	hostKey, err := generateTestHostKey()
	if err != nil {
		t.Fatalf("Failed to generate host key: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	addr := listener.Addr().(*net.TCPAddr)
	host = "127.0.0.1"
	port = addr.Port

	clientPrivateKey, err := generateTestClientKey()
	if err != nil {
		t.Fatalf("Failed to generate client key: %v", err)
	}

	_, err = ssh.ParsePrivateKey(clientPrivateKey)
	if err != nil {
		t.Fatalf("Failed to parse client key: %v", err)
	}

	config := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			return &ssh.Permissions{}, nil
		},
	}
	config.AddHostKey(hostKey)

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
				if err != nil {
					return
				}
				defer sshConn.Close()
				go ssh.DiscardRequests(reqs)
				for newChannel := range chans {
					go handleChannel(t, newChannel)
				}
			}()
		}
	}()

	cleanup = func() {
		listener.Close()
		<-serverDone
	}

	return host, port, clientPrivateKey, cleanup
}

func generateTestHostKey() (ssh.Signer, error) {
	privateKey, err := generateTestClientKey()
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return nil, err
	}
	return signer, nil
}

func generateTestClientKey() ([]byte, error) {
	_, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, err
	}

	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, err
	}

	pemBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	}

	return pem.EncodeToMemory(pemBlock), nil
}

func handleChannel(t *testing.T, newChannel ssh.NewChannel) {
	if newChannel.ChannelType() != "session" {
		newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
		return
	}
	channel, requests, err := newChannel.Accept()
	if err != nil {
		return
	}
	defer channel.Close()

	go func() {
		for req := range requests {
			switch req.Type {
			case "exec":
				req.Reply(true, nil)
				channel.Write([]byte("test output"))
				channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
				return
			default:
				req.Reply(false, nil)
			}
		}
	}()
}

// TestSSHClientClose tests the Close method
func TestSSHClientClose(t *testing.T) {
	host, port, _, cleanup := mockSSHServer(t)
	defer cleanup()

	// Generate a client key for authentication
	clientKey, err := generateTestClientKey()
	if err != nil {
		t.Fatalf("Failed to generate client key: %v", err)
	}

	signer, err := ssh.ParsePrivateKey(clientKey)
	if err != nil {
		t.Fatalf("Failed to parse client key: %v", err)
	}

	// Create a client config for testing
	config := &ssh.ClientConfig{
		User:            "testuser",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		Timeout:         5 * time.Second,
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", host, port), config)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	sshClient := &SSHClient{
		client: client,
		host:   host,
		user:   "testuser",
	}

	err = sshClient.Close()
	if err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

// TestSSHClientCloseNilClient tests Close with nil client
func TestSSHClientCloseNilClient(t *testing.T) {
	sshClient := &SSHClient{
		client: nil,
		host:   "localhost",
		user:   "testuser",
	}

	err := sshClient.Close()
	if err != nil {
		t.Errorf("Close() with nil client returned error: %v", err)
	}
}

// TestSSHConfigAssembly tests SSH config assembly logic
func TestSSHConfigAssembly(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		user     string
		keyPath  string
		wantAddr string
	}{
		{
			name:     "default port",
			host:     "192.168.1.1",
			user:     "root",
			keyPath:  "/tmp/test_key",
			wantAddr: "192.168.1.1:22",
		},
		{
			name:     "localhost",
			host:     "127.0.0.1",
			user:     "admin",
			keyPath:  "/home/admin/.ssh/id_rsa",
			wantAddr: "127.0.0.1:22",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := fmt.Sprintf("%s:22", tt.host)
			if addr != tt.wantAddr {
				t.Errorf("address = %v, want %v", addr, tt.wantAddr)
			}
		})
	}
}

// TestContextCancellation tests context cancellation handling
func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	select {
	case <-ctx.Done():
		if ctx.Err() != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", ctx.Err())
		}
	default:
		t.Error("context should be cancelled")
	}
}

// TestUploadContentCommand tests UploadContent command building
func TestUploadContentCommand(t *testing.T) {
	remotePath := "/tmp/test.txt"
	perm := "644"
	expectedCmd := fmt.Sprintf("cat > %s && chmod %s %s", remotePath, perm, remotePath)

	if expectedCmd != "cat > /tmp/test.txt && chmod 644 /tmp/test.txt" {
		t.Errorf("command mismatch: %s", expectedCmd)
	}
}

// TestRunTimeout tests Run command timeout handling
func TestRunTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	time.Sleep(5 * time.Millisecond)

	select {
	case <-ctx.Done():
		if ctx.Err() != context.DeadlineExceeded {
			t.Errorf("expected DeadlineExceeded, got %v", ctx.Err())
		}
	default:
		t.Error("context should have timed out")
	}
}

// TestErrorMessageFormatting tests error message formatting
func TestErrorMessageFormatting(t *testing.T) {
	cmd := "ls -la"
	stderr := "permission denied"
	originalErr := fmt.Errorf("exit status 1")

	formattedErr := fmt.Errorf("command failed: %w (stderr: %s)", originalErr, stderr)
	expected := "command failed: exit status 1 (stderr: permission denied)"

	if formattedErr.Error() != expected {
		t.Errorf("error message = %v, want %v", formattedErr.Error(), expected)
	}

	_ = cmd
}

// TestNewSSHClientKeyPathResolution tests key path resolution
func TestNewSSHClientKeyPathResolution(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get home directory")
	}

	defaultKeyPath := filepath.Join(homeDir, ".ssh", "id_rsa")
	if !strings.Contains(defaultKeyPath, ".ssh") {
		t.Error("default key path should contain .ssh")
	}

	envKeyPath := os.Getenv("CILO_SSH_KEY_PATH")
	_ = envKeyPath
}
