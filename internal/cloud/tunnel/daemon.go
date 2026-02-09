// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package tunnel

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// DaemonConfig contains the configuration for a tunnel daemon
type DaemonConfig struct {
	Interface      string   `json:"interface"`
	PrivateKey     string   `json:"private_key"`
	Address        string   `json:"address"`
	ListenPort     int      `json:"listen_port"`
	ServerPubKey   string   `json:"server_pub_key"`
	ServerEndpoint string   `json:"server_endpoint"`
	AllowedIPs     []string `json:"allowed_ips"`
	EnvironmentID  string   `json:"environment_id"`
}

// DaemonState represents the current state of the tunnel daemon
type DaemonState struct {
	Running       bool      `json:"running"`
	PID           int       `json:"pid"`
	Interface     string    `json:"interface"`
	Address       string    `json:"address"`
	EnvironmentID string    `json:"environment_id"`
	StartedAt     time.Time `json:"started_at"`
}

func DaemonDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cilo", "tunnel"), nil
}

// configPath returns the path to the daemon config file
func configPath() (string, error) {
	dir, err := DaemonDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// pidPath returns the path to the daemon PID file
func pidPath() (string, error) {
	dir, err := DaemonDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "daemon.pid"), nil
}

// statePath returns the path to the daemon state file
func statePath() (string, error) {
	dir, err := DaemonDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "state.json"), nil
}

// socketPath returns the path to the daemon control socket
func socketPath() (string, error) {
	dir, err := DaemonDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "daemon.sock"), nil
}

// SaveDaemonConfig saves the daemon configuration
func SaveDaemonConfig(cfg *DaemonConfig) error {
	dir, err := DaemonDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	os.Chmod(dir, 0755)

	path, err := configPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}
	return os.Chmod(path, 0644)
}

// LoadDaemonConfig loads the daemon configuration
func LoadDaemonConfig() (*DaemonConfig, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg DaemonConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func SaveDaemonState(state *DaemonState) error {
	dir, err := DaemonDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0755); err != nil {
		return err
	}

	path, err := statePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}
	return os.Chmod(path, 0644)
}

// LoadDaemonState loads the daemon state
func LoadDaemonState() (*DaemonState, error) {
	path, err := statePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &DaemonState{Running: false}, nil
		}
		return nil, err
	}

	var state DaemonState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	// Verify the process is actually running
	if state.Running && state.PID > 0 {
		process, err := os.FindProcess(state.PID)
		if err != nil || process.Signal(syscall.Signal(0)) != nil {
			state.Running = false
		}
	}

	return &state, nil
}

// ClearDaemonState removes all daemon state files
func ClearDaemonState() error {
	dir, err := DaemonDir()
	if err != nil {
		return err
	}

	files := []string{"config.json", "state.json", "daemon.pid", "daemon.sock"}
	for _, f := range files {
		os.Remove(filepath.Join(dir, f))
	}

	return nil
}

// Daemon represents a running tunnel daemon
type Daemon struct {
	config      *DaemonConfig
	tunnel      *Tunnel
	manager     *Manager
	listener    net.Listener
	stopCh      chan struct{}
	actualIface string
}

// NewDaemon creates a new tunnel daemon
func NewDaemon(cfg *DaemonConfig) (*Daemon, error) {
	return &Daemon{
		config: cfg,
		stopCh: make(chan struct{}),
	}, nil
}

// Run starts the daemon and blocks until stopped
func (d *Daemon) Run() error {
	// Create the tunnel interface
	actualName, err := CreateInterface(d.config.Interface)
	if err != nil {
		return fmt.Errorf("create interface: %w", err)
	}
	d.actualIface = actualName

	// Create manager for this interface
	d.manager, err = NewManager(actualName)
	if err != nil {
		RemoveInterface(actualName)
		return fmt.Errorf("create manager: %w", err)
	}

	// Configure WireGuard
	if err := d.manager.Configure(d.config.PrivateKey, d.config.ListenPort); err != nil {
		RemoveInterface(actualName)
		return fmt.Errorf("configure wireguard: %w", err)
	}

	// Add IP address
	if err := AddAddress(actualName, d.config.Address); err != nil {
		RemoveInterface(actualName)
		return fmt.Errorf("add address: %w", err)
	}

	// Bring interface up
	if err := SetInterfaceUp(actualName); err != nil {
		RemoveInterface(actualName)
		return fmt.Errorf("set interface up: %w", err)
	}

	// Add peer (server)
	if err := d.manager.AddPeer(d.config.ServerPubKey, d.config.ServerEndpoint, d.config.AllowedIPs, 25*time.Second); err != nil {
		RemoveInterface(actualName)
		return fmt.Errorf("add peer: %w", err)
	}

	// Add routes for AllowedIPs CIDRs
	for _, cidr := range d.config.AllowedIPs {
		// Skip the host's own /32 address - it's not a routeable destination
		if strings.HasSuffix(cidr, "/32") {
			continue
		}

		// Parse the local address to use as gateway
		ip, _, err := net.ParseCIDR(d.config.Address)
		if err != nil {
			fmt.Printf("Warning: could not parse local address %s: %v\n", d.config.Address, err)
			continue
		}

		if err := AddRoute(actualName, cidr, ip); err != nil {
			fmt.Printf("Warning: failed to add route for %s: %v\n", cidr, err)
			// Continue - WireGuard might still work for some traffic
		} else {
			fmt.Printf("  ✓ Added route: %s via %s\n", cidr, ip.String())
		}
	}

	// Save state
	state := &DaemonState{
		Running:       true,
		PID:           os.Getpid(),
		Interface:     actualName,
		Address:       d.config.Address,
		EnvironmentID: d.config.EnvironmentID,
		StartedAt:     time.Now(),
	}
	if err := SaveDaemonState(state); err != nil {
		fmt.Printf("Warning: failed to save state: %v\n", err)
	}

	sockPath, _ := socketPath()
	os.Remove(sockPath)
	d.listener, err = net.Listen("unix", sockPath)
	if err != nil {
		fmt.Printf("Warning: failed to create control socket: %v\n", err)
	} else {
		go d.handleControlSocket()
	}

	fmt.Printf("Tunnel daemon started (interface: %s, address: %s)\n", actualName, d.config.Address)

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		fmt.Println("\nReceived shutdown signal")
	case <-d.stopCh:
		fmt.Println("\nReceived stop command")
	}

	d.cleanup()

	return nil
}

func (d *Daemon) cleanup() {
	if d.listener != nil {
		d.listener.Close()
	}

	if d.manager != nil {
		d.manager.Close()
	}

	if d.actualIface != "" {
		RemoveInterface(d.actualIface)
	}

	SaveDaemonState(&DaemonState{Running: false})

	sockPath, _ := socketPath()
	os.Remove(sockPath)

	fmt.Println("Tunnel daemon stopped")
}

// handleControlSocket handles commands from the control socket
func (d *Daemon) handleControlSocket() {
	for {
		conn, err := d.listener.Accept()
		if err != nil {
			return // Listener closed
		}

		buf := make([]byte, 256)
		n, err := conn.Read(buf)
		if err != nil {
			conn.Close()
			continue
		}

		cmd := string(buf[:n])
		switch cmd {
		case "stop":
			conn.Write([]byte("stopping"))
			conn.Close()
			close(d.stopCh)
			return
		case "status":
			stats, _ := d.manager.GetPeerStats()
			resp, _ := json.Marshal(map[string]interface{}{
				"interface": d.actualIface,
				"address":   d.config.Address,
				"peers":     stats,
			})
			conn.Write(resp)
		default:
			conn.Write([]byte("unknown command"))
		}
		conn.Close()
	}
}

// StopDaemon stops a running daemon by sending a command to its control socket
func StopDaemon() error {
	state, err := LoadDaemonState()
	if err != nil {
		return err
	}

	if !state.Running {
		return fmt.Errorf("daemon is not running")
	}

	sockPath, err := socketPath()
	if err != nil {
		return err
	}

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		// Socket not available, try to kill the process directly
		if state.PID > 0 {
			process, err := os.FindProcess(state.PID)
			if err == nil {
				process.Signal(syscall.SIGTERM)
			}
		}
		ClearDaemonState()
		return nil
	}
	defer conn.Close()

	conn.Write([]byte("stop"))

	buf := make([]byte, 256)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	conn.Read(buf)

	return nil
}

// GetDaemonStatus returns the current daemon status
func GetDaemonStatus() (*DaemonState, error) {
	state, err := LoadDaemonState()
	if err != nil {
		return nil, err
	}

	if !state.Running {
		return state, nil
	}

	// Try to get live status from socket
	sockPath, err := socketPath()
	if err != nil {
		return state, nil
	}

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		state.Running = false
		return state, nil
	}
	defer conn.Close()

	conn.Write([]byte("status"))

	buf := make([]byte, 4096)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		return state, nil
	}

	var liveStatus map[string]interface{}
	if err := json.Unmarshal(buf[:n], &liveStatus); err == nil {
		if iface, ok := liveStatus["interface"].(string); ok {
			state.Interface = iface
		}
	}

	return state, nil
}
