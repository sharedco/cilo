// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: BUSL-1.1
// See LICENSES/BUSL-1.1.txt and LICENSE.enterprise for full license text

package vm

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHClient wraps an SSH connection for remote operations
type SSHClient struct {
	client *ssh.Client
	host   string
	user   string
}

// NewSSHClient creates a new SSH client connection
func NewSSHClient(host, user, keyPath string) (*SSHClient, error) {
	if keyPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home directory: %w", err)
		}
		keyPath = filepath.Join(homeDir, ".ssh", "id_rsa")
	}

	if envKeyPath := os.Getenv("CILO_SSH_KEY_PATH"); envKeyPath != "" {
		keyPath = envKeyPath
	}

	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read SSH key from %s: %w", keyPath, err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("parse SSH key: %w", err)
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: production should verify known hosts
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:22", host)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("SSH dial to %s: %w", addr, err)
	}

	return &SSHClient{
		client: client,
		host:   host,
		user:   user,
	}, nil
}

// Close closes the SSH connection
func (s *SSHClient) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

// Run executes a command on the remote host with context support
func (s *SSHClient) Run(ctx context.Context, cmd string) (string, error) {
	session, err := s.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("create SSH session: %w", err)
	}
	defer session.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	done := make(chan error, 1)
	go func() {
		done <- session.Run(cmd)
	}()

	select {
	case <-ctx.Done():
		session.Signal(ssh.SIGTERM)
		return "", ctx.Err()
	case err := <-done:
		if err != nil {
			return stdout.String(), fmt.Errorf("command failed: %w (stderr: %s)", err, stderr.String())
		}
		return stdout.String(), nil
	}
}

// UploadContent uploads file content to a remote path with specified permissions
func (s *SSHClient) UploadContent(ctx context.Context, content []byte, remotePath, perm string) error {
	session, err := s.client.NewSession()
	if err != nil {
		return fmt.Errorf("create SSH session: %w", err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("get stdin pipe: %w", err)
	}

	cmd := fmt.Sprintf("cat > %s && chmod %s %s", remotePath, perm, remotePath)
	if err := session.Start(cmd); err != nil {
		return fmt.Errorf("start upload command: %w", err)
	}

	if _, err := stdin.Write(content); err != nil {
		return fmt.Errorf("write content: %w", err)
	}
	stdin.Close()

	done := make(chan error, 1)
	go func() {
		done <- session.Wait()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return fmt.Errorf("upload failed: %w", err)
		}
		return nil
	}
}

// WireGuardKeys holds generated WireGuard key pair
type WireGuardKeys struct {
	PrivateKey string
	PublicKey  string
}

// generateWireGuardKeys generates a new WireGuard key pair on the remote machine
func (m *ManualProvider) generateWireGuardKeys(ctx context.Context, sshClient *SSHClient) (*WireGuardKeys, error) {
	_, err := sshClient.Run(ctx, "which wg")
	if err != nil {
		_, err = sshClient.Run(ctx, "apt-get update && apt-get install -y wireguard-tools")
		if err != nil {
			return nil, fmt.Errorf("install wireguard-tools: %w", err)
		}
	}

	privateKey, err := sshClient.Run(ctx, "wg genkey")
	if err != nil {
		return nil, fmt.Errorf("generate WireGuard private key: %w", err)
	}
	privateKey = strings.TrimSpace(privateKey)

	publicKey, err := sshClient.Run(ctx, fmt.Sprintf("echo '%s' | wg pubkey", privateKey))
	if err != nil {
		return nil, fmt.Errorf("generate WireGuard public key: %w", err)
	}
	publicKey = strings.TrimSpace(publicKey)

	return &WireGuardKeys{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

// setupWireGuard configures WireGuard on the remote machine
func (m *ManualProvider) setupWireGuard(ctx context.Context, sshClient *SSHClient, keys *WireGuardKeys, machine *Machine) error {
	_, err := sshClient.Run(ctx, "apt-get update && apt-get install -y wireguard-tools")
	if err != nil {
		return fmt.Errorf("install wireguard-tools: %w", err)
	}

	_, err = sshClient.Run(ctx, "sysctl -w net.ipv4.ip_forward=1")
	if err != nil {
		return fmt.Errorf("enable IP forwarding: %w", err)
	}

	_, err = sshClient.Run(ctx, "echo 'net.ipv4.ip_forward=1' > /etc/sysctl.d/99-wireguard.conf")
	if err != nil {
		return fmt.Errorf("persist IP forwarding: %w", err)
	}

	wgConfig := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = 10.224.0.2/32
ListenPort = 51820

PostUp = iptables -A FORWARD -i wg0 -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
PostDown = iptables -D FORWARD -i wg0 -j ACCEPT; iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE
`, keys.PrivateKey)

	err = sshClient.UploadContent(ctx, []byte(wgConfig), "/etc/wireguard/wg0.conf", "600")
	if err != nil {
		return fmt.Errorf("upload WireGuard config: %w", err)
	}

	_, err = sshClient.Run(ctx, "systemctl enable wg-quick@wg0 && systemctl start wg-quick@wg0")
	if err != nil {
		return fmt.Errorf("start WireGuard service: %w", err)
	}

	return nil
}

// installSystemdService creates and enables the cilo-agent systemd service
func (m *ManualProvider) installSystemdService(ctx context.Context, sshClient *SSHClient, machine *Machine) error {
	serviceContent := `[Unit]
Description=Cilo Agent
After=network.target wg-quick@wg0.service
Requires=wg-quick@wg0.service

[Service]
Type=simple
User=root
WorkingDirectory=/opt/cilo
EnvironmentFile=/opt/cilo/config/agent.env
ExecStart=/opt/cilo/bin/cilo-agent
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`

	err := sshClient.UploadContent(ctx, []byte(serviceContent), "/etc/systemd/system/cilo-agent.service", "644")
	if err != nil {
		return fmt.Errorf("upload systemd service file: %w", err)
	}

	_, err = sshClient.Run(ctx, "systemctl daemon-reload && systemctl enable cilo-agent")
	if err != nil {
		return fmt.Errorf("enable cilo-agent service: %w", err)
	}

	return nil
}

// verifyAgent checks if the agent is running and accessible on port 8080
func (m *ManualProvider) verifyAgent(ctx context.Context, sshClient *SSHClient) error {
	_, err := sshClient.Run(ctx, "systemctl is-active cilo-agent")
	if err != nil {
		return fmt.Errorf("cilo-agent service not active: %w", err)
	}

	_, err = sshClient.Run(ctx, "ss -tlnp | grep ':8080'")
	if err != nil {
		return fmt.Errorf("agent not listening on port 8080: %w", err)
	}

	_, err = sshClient.Run(ctx, "curl -s -o /dev/null -w '%{http_code}' http://localhost:8080/health")
	if err != nil {
		return fmt.Errorf("agent health check failed: %w", err)
	}

	return nil
}

// ManualProvider implements Provider for manually registered machines
type ManualProvider struct {
	store PoolStore
}

// NewManualProvider creates a new manual provider
func NewManualProvider(store PoolStore) *ManualProvider {
	return &ManualProvider{store: store}
}

// Name returns the provider name
func (m *ManualProvider) Name() string {
	return "manual"
}

// Provision is not supported for manual provider
func (m *ManualProvider) Provision(ctx context.Context, config ProvisionConfig) (*Machine, error) {
	return nil, fmt.Errorf("manual provider does not support auto-provisioning; use 'cilo-server machines add' to register machines")
}

// Destroy is a no-op for manual provider (machines are persistent)
func (m *ManualProvider) Destroy(ctx context.Context, providerID string) error {
	// Manual machines are not destroyed, just unregistered
	return nil
}

// List returns all registered machines
func (m *ManualProvider) List(ctx context.Context) ([]*Machine, error) {
	return m.store.ListMachines(ctx)
}

// HealthCheck checks if a machine is reachable via SSH
func (m *ManualProvider) HealthCheck(ctx context.Context, providerID string) (bool, error) {
	machine, err := m.store.GetMachine(ctx, providerID)
	if err != nil {
		return false, err
	}

	// Try to connect via SSH
	config := &ssh.ClientConfig{
		User:            machine.SSHUser,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Use known hosts
		Timeout:         5 * time.Second,
	}

	addr := fmt.Sprintf("%s:22", machine.SSHHost)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return false, nil // Machine is unreachable
	}
	defer client.Close()

	return true, nil
}

// GetMachine returns a machine by ID
func (m *ManualProvider) GetMachine(ctx context.Context, providerID string) (*Machine, error) {
	return m.store.GetMachine(ctx, providerID)
}

// RegisterMachine registers a new machine (manual provider specific)
func (m *ManualProvider) RegisterMachine(ctx context.Context, name, sshHost, sshUser string) (*Machine, error) {
	machine := &Machine{
		ID:           fmt.Sprintf("manual-%s", name),
		ProviderID:   name,
		ProviderType: "manual",
		PublicIP:     sshHost,
		SSHHost:      sshHost,
		SSHUser:      sshUser,
		Status:       MachineStatusProvisioning,
		CreatedAt:    time.Now(),
	}

	// Save to store
	if err := m.store.SaveMachine(ctx, machine); err != nil {
		return nil, fmt.Errorf("save machine: %w", err)
	}

	return machine, nil
}

// SetupAgent installs the cilo-agent on a machine via SSH
func (m *ManualProvider) SetupAgent(ctx context.Context, machine *Machine, agentURL string) error {
	keyPath := os.Getenv("CILO_SSH_KEY_PATH")

	sshClient, err := NewSSHClient(machine.SSHHost, machine.SSHUser, keyPath)
	if err != nil {
		return fmt.Errorf("connect to machine %s: %w", machine.SSHHost, err)
	}
	defer sshClient.Close()

	_, err = sshClient.Run(ctx, "mkdir -p /opt/cilo/bin /opt/cilo/config /var/lib/cilo/envs")
	if err != nil {
		return fmt.Errorf("create directories: %w", err)
	}

	_, err = sshClient.Run(ctx, fmt.Sprintf("curl -fsSL -o /opt/cilo/bin/cilo-agent '%s' && chmod +x /opt/cilo/bin/cilo-agent", agentURL))
	if err != nil {
		return fmt.Errorf("download agent binary: %w", err)
	}

	wgKeys, err := m.generateWireGuardKeys(ctx, sshClient)
	if err != nil {
		return fmt.Errorf("generate WireGuard keys: %w", err)
	}

	machine.WGPublicKey = wgKeys.PublicKey
	machine.WGEndpoint = fmt.Sprintf("%s:51820", machine.PublicIP)

	err = m.setupWireGuard(ctx, sshClient, wgKeys, machine)
	if err != nil {
		return fmt.Errorf("setup WireGuard: %w", err)
	}

	agentEnv := fmt.Sprintf(`CILO_AGENT_ID=%s
CILO_AGENT_PRIVATE_KEY=%s
CILO_AGENT_PUBLIC_KEY=%s
CILO_AGENT_ENDPOINT=%s
CILO_AGENT_LISTEN_ADDR=:8080
CILO_AGENT_DATA_DIR=/var/lib/cilo
`, machine.ID, wgKeys.PrivateKey, wgKeys.PublicKey, machine.WGEndpoint)

	err = sshClient.UploadContent(ctx, []byte(agentEnv), "/opt/cilo/config/agent.env", "600")
	if err != nil {
		return fmt.Errorf("upload agent.env: %w", err)
	}

	err = m.installSystemdService(ctx, sshClient, machine)
	if err != nil {
		return fmt.Errorf("install systemd service: %w", err)
	}

	_, err = sshClient.Run(ctx, "systemctl start cilo-agent")
	if err != nil {
		return fmt.Errorf("start cilo-agent service: %w", err)
	}

	err = m.verifyAgent(ctx, sshClient)
	if err != nil {
		return fmt.Errorf("verify agent: %w", err)
	}

	machine.Status = MachineStatusReady
	if err := m.store.SaveMachine(ctx, machine); err != nil {
		return fmt.Errorf("update machine status: %w", err)
	}

	return nil
}
