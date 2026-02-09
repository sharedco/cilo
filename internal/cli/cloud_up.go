// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sharedco/cilo/internal/cloud"
	"github.com/sharedco/cilo/internal/cloud/tunnel"
	"github.com/sharedco/cilo/internal/dns"
	"github.com/sharedco/cilo/internal/engine"
	"github.com/sharedco/cilo/internal/models"
	_ "github.com/sharedco/cilo/internal/parsers"
	"github.com/spf13/cobra"
)

var cloudUpCmd = &cobra.Command{
	Use:   "up <name>",
	Short: "Start a remote environment",
	Long: `Create and start a remote environment on Cilo Cloud.

This command:
1. Detects your project format (Compose, Devcontainer, Procfile)
2. Syncs your workspace to a remote machine
3. Starts the environment
4. Establishes a WireGuard tunnel for connectivity
5. Configures local DNS to route to remote services

Examples:
  cilo cloud up agent-1 --from .
  cilo cloud up pr-42 --from /path/to/project
  cilo cloud up ci-test --ci  # CI mode: no tunnel, direct IP`,
	Args: cobra.ExactArgs(1),
	RunE: runCloudUp,
}

func init() {
	cloudUpCmd.Flags().String("from", ".", "Source project directory")
	cloudUpCmd.Flags().Bool("build", false, "Build images before starting")
	cloudUpCmd.Flags().Bool("ci", false, "CI mode: skip WireGuard, use direct IP")
	cloudUpCmd.Flags().Int("ci-timeout", 60, "CI mode: auto-destroy after N minutes")
	cloudCmd.AddCommand(cloudUpCmd)
}

func runCloudUp(cmd *cobra.Command, args []string) error {
	envName := args[0]
	fromPath, _ := cmd.Flags().GetString("from")
	build, _ := cmd.Flags().GetBool("build")
	ciMode, _ := cmd.Flags().GetBool("ci")
	ciTimeout, _ := cmd.Flags().GetInt("ci-timeout")

	absPath, err := filepath.Abs(fromPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("source path does not exist: %s", absPath)
	}

	spec, err := engine.DetectAndParse(absPath)
	if err != nil {
		return fmt.Errorf("no supported project format found: %w", err)
	}

	if spec.Project == "" {
		spec.Project = filepath.Base(absPath)
	}

	fmt.Printf("Found %s project (%d services): %s\n",
		spec.Source, len(spec.Services), spec.SourcePath)

	if ciMode {
		return runCloudUpCI(envName, spec, ciTimeout)
	}

	fmt.Printf("\nCreating cloud environment: %s\n", envName)
	fmt.Printf("  Source: %s\n", absPath)
	fmt.Printf("  Build: %v\n", build)

	return runCloudUpMain(envName, absPath, spec, build)
}

func runCloudUpCI(envName string, spec *engine.EnvironmentSpec, timeout int) error {
	fmt.Printf("\n=== CI Mode: Direct IP Access (No WireGuard) ===\n")
	fmt.Printf("Environment: %s\n", envName)
	fmt.Printf("Auto-destroy: %d minutes\n\n", timeout)

	client, err := cloud.NewClientFromAuth()
	if err != nil {
		return fmt.Errorf("cloud auth: %w", err)
	}

	ctx := context.Background()
	resp, err := client.CreateEnvironment(ctx, cloud.CreateEnvironmentRequest{
		Name:      envName,
		Project:   spec.Project,
		Format:    string(spec.Source),
		Source:    "ci",
		CIMode:    true,
		CITimeout: timeout,
	})
	if err != nil {
		return fmt.Errorf("create environment: %w", err)
	}

	outputCIResult(resp, envName)
	return nil
}

func outputCIResult(resp *cloud.CreateEnvironmentResponse, envName string) {
	envID := resp.Environment.ID
	publicIP := resp.PublicIP
	expiresAt := resp.ExpiresAt

	fmt.Printf("Environment created: %s\n", envID)
	if publicIP != "" {
		fmt.Printf("Public IP: %s\n", publicIP)
	}
	if expiresAt != "" {
		fmt.Printf("Expires at: %s\n\n", expiresAt)
	}

	fmt.Println("=== GitHub Actions Outputs ===")

	// Write to GITHUB_OUTPUT environment file (modern approach)
	if ghOutput := os.Getenv("GITHUB_OUTPUT"); ghOutput != "" {
		if f, err := os.OpenFile(ghOutput, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644); err == nil {
			fmt.Fprintf(f, "environment-id=%s\n", envID)
			fmt.Fprintf(f, "environment-name=%s\n", envName)
			if publicIP != "" {
				fmt.Fprintf(f, "public-ip=%s\n", publicIP)
			}
			f.Close()
			fmt.Printf("environment-id=%s\n", envID)
			fmt.Printf("environment-name=%s\n", envName)
			if publicIP != "" {
				fmt.Printf("public-ip=%s\n", publicIP)
			}
		}
	} else {
		// Fallback: print for manual copy-paste
		fmt.Printf("environment-id=%s\n", envID)
		fmt.Printf("environment-name=%s\n", envName)
		if publicIP != "" {
			fmt.Printf("public-ip=%s\n", publicIP)
		}
	}

	fmt.Println("\n=== Environment Variables ===")
	fmt.Printf("CILO_ENV_ID=%s\n", envID)
	fmt.Printf("CILO_ENV_NAME=%s\n", envName)
	if publicIP != "" {
		fmt.Printf("CILO_PUBLIC_IP=%s\n", publicIP)
	}
}

func runCloudUpMain(envName, absPath string, spec *engine.EnvironmentSpec, build bool) error {
	ctx := context.Background()

	// Phase 1: Require elevated privileges for WireGuard (PRD Section 3.7)
	// Future Phase 3 will use setuid helper to avoid this requirement
	if os.Geteuid() != 0 {
		return fmt.Errorf(
			"WireGuard tunnel requires elevated privileges.\n\n"+
				"Run with sudo:\n"+
				"  sudo cilo cloud up %s --from %s\n\n"+
				"Why: Creates a WireGuard interface to route traffic to your\n"+
				"remote environment. Requires CAP_NET_ADMIN.",
			envName, absPath,
		)
	}

	fmt.Println("\n→ Loading cloud authentication...")
	client, err := cloud.NewClientFromAuth()
	if err != nil {
		return fmt.Errorf("not logged in, run 'cilo cloud login' first: %w", err)
	}

	fmt.Println("→ Creating environment on Cilo Cloud...")

	format := string(spec.Source)
	if format == "compose" {
		format = "docker-compose"
	}

	createReq := cloud.CreateEnvironmentRequest{
		Name:    envName,
		Project: spec.Project,
		Format:  format,
		Source:  "cli",
		CIMode:  false,
	}

	createResp, err := client.CreateEnvironment(ctx, createReq)
	if err != nil {
		return fmt.Errorf("create environment: %w", err)
	}

	envID := createResp.Environment.ID
	fmt.Printf("  ✓ Environment created: %s\n", envID)

	cleanup := func(reason string) {
		fmt.Printf("\n⚠ Cleanup due to: %s\n", reason)
		if envID != "" {
			fmt.Println("  → Destroying environment...")
			if err := client.DestroyEnvironment(ctx, envID); err != nil {
				fmt.Printf("    ✗ Failed to destroy environment: %v\n", err)
			} else {
				fmt.Println("    ✓ Environment destroyed")
			}
		}
		_ = tunnel.RemoveInterface("cilo0")
	}

	fmt.Println("→ Syncing workspace to remote machine...")
	if createResp.Machine == nil || createResp.Machine.PublicIP == "" {
		cleanup("no machine assigned")
		return fmt.Errorf("no machine assigned to environment")
	}

	machineIP := createResp.Machine.PublicIP
	syncCfg := cloud.SyncConfig{
		LocalPath:       absPath,
		RemoteHost:      machineIP,
		RemoteUser:      "root",
		RemotePath:      fmt.Sprintf("/var/cilo/envs/%s", envID),
		ExcludePatterns: cloud.DefaultExcludePatterns(),
	}

	if err := cloud.SyncWorkspace(ctx, syncCfg); err != nil {
		if strings.Contains(err.Error(), "rsync") {
			fmt.Println("  ⚠ rsync not available or failed, attempting alternative sync...")
			if err := fallbackSync(ctx, absPath, machineIP, envID); err != nil {
				cleanup("workspace sync failed")
				return fmt.Errorf("sync workspace (fallback): %w", err)
			}
		} else {
			cleanup("workspace sync failed")
			return fmt.Errorf("sync workspace: %w", err)
		}
	}
	fmt.Println("  ✓ Workspace synced")

	if err := client.SyncComplete(ctx, envID); err != nil {
		fmt.Printf("  ⚠ Failed to signal sync complete: %v\n", err)
	}

	fmt.Println("→ Setting up WireGuard tunnel...")
	keyPair, err := tunnel.GenerateKeyPair()
	if err != nil {
		cleanup("key generation failed")
		return fmt.Errorf("generate WireGuard keys: %w", err)
	}

	wgReq := cloud.WireGuardExchangeRequest{
		EnvironmentID: envID,
		ClientPubKey:  keyPair.PublicKey,
	}

	wgResp, err := client.WireGuardExchange(ctx, wgReq)
	if err != nil {
		cleanup("WireGuard key exchange failed")
		return fmt.Errorf("WireGuard key exchange: %w", err)
	}

	fmt.Printf("  ✓ Keys exchanged (assigned IP: %s)\n", wgResp.AssignedIP)

	fmt.Println("→ Configuring local WireGuard interface...")
	tunCfg := tunnel.Config{
		Interface:  "cilo0",
		ListenPort: 51820,
		Address:    wgResp.AssignedIP + "/32",
	}

	tun, err := tunnel.New(tunCfg)
	if err != nil {
		cleanup("tunnel creation failed")
		return fmt.Errorf("create tunnel: %w", err)
	}

	if err := setupWireGuardInterface(tun, keyPair.PrivateKey, wgResp); err != nil {
		cleanup("WireGuard setup failed")
		return fmt.Errorf("setup WireGuard: %w", err)
	}
	fmt.Println("  ✓ WireGuard interface configured (cilo0)")

	fmt.Println("→ Waiting for environment to be ready...")
	if err := waitForEnvironmentReady(ctx, client, envID); err != nil {
		cleanup("environment failed to become ready")
		return fmt.Errorf("wait for ready: %w", err)
	}

	env, err := client.GetEnvironment(ctx, envID)
	if err != nil {
		fmt.Printf("  ⚠ Failed to get environment details: %v\n", err)
	}

	fmt.Println("→ Configuring DNS...")
	if err := configureCloudDNS(envName, spec.Project, env); err != nil {
		fmt.Printf("  ⚠ DNS configuration warning: %v\n", err)
		fmt.Println("    You may need to manually configure /etc/hosts")
	} else {
		fmt.Println("  ✓ DNS configured")
	}

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("✓ Environment ready!")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("Environment ID: %s\n", envID)
	fmt.Printf("Name: %s\n", envName)
	if env != nil && env.Subnet != "" {
		fmt.Printf("Subnet: %s\n", env.Subnet)
	}
	fmt.Printf("WireGuard: cilo0 (%s)\n", wgResp.AssignedIP)

	fmt.Println("\nService URLs:")
	dnsSuffix := ".test"
	if env != nil && len(env.Services) > 0 {
		for _, svc := range env.Services {
			url := fmt.Sprintf("http://%s.%s%s", svc.Name, envName, dnsSuffix)
			fmt.Printf("  %s\n", url)
		}
	} else {
		for _, svc := range spec.Services {
			url := fmt.Sprintf("http://%s.%s%s", svc.Name, envName, dnsSuffix)
			fmt.Printf("  %s (predicted)\n", url)
		}
	}

	fmt.Printf("\nApex URL: http://%s.%s%s\n", spec.Project, envName, dnsSuffix)
	fmt.Println("\nTo connect:")
	fmt.Printf("  curl http://api.%s.test\n", envName)
	fmt.Println("\nTo destroy:")
	fmt.Printf("  cilo cloud destroy %s\n", envName)

	return nil
}

func setupWireGuardInterface(tun *tunnel.Tunnel, privateKey string, wgResp *cloud.WireGuardExchangeResponse) error {
	actualName, err := tunnel.CreateInterface(tun.Interface)
	if err != nil {
		return fmt.Errorf("create interface: %w", err)
	}
	tun.Interface = actualName

	manager, err := tunnel.NewManager(tun.Interface)
	if err != nil {
		tunnel.RemoveInterface(tun.Interface)
		return fmt.Errorf("create manager: %w", err)
	}
	defer manager.Close()

	if err := manager.Configure(privateKey, tun.ListenPort); err != nil {
		tunnel.RemoveInterface(tun.Interface)
		return fmt.Errorf("configure: %w", err)
	}

	if err := tunnel.AddAddress(tun.Interface, tun.Address); err != nil {
		tunnel.RemoveInterface(tun.Interface)
		return fmt.Errorf("add address: %w", err)
	}

	if err := tunnel.SetInterfaceUp(tun.Interface); err != nil {
		tunnel.RemoveInterface(tun.Interface)
		return fmt.Errorf("set interface up: %w", err)
	}

	allowedIPs := strings.Split(wgResp.AllowedIPs, ",")
	for i, ip := range allowedIPs {
		allowedIPs[i] = strings.TrimSpace(ip)
	}

	if err := manager.AddPeer(wgResp.ServerPubKey, wgResp.ServerEndpoint, allowedIPs, 25*time.Second); err != nil {
		tunnel.RemoveInterface(tun.Interface)
		return fmt.Errorf("add peer: %w", err)
	}

	return nil
}

func waitForEnvironmentReady(ctx context.Context, client *cloud.Client, envID string) error {
	const pollInterval = 5 * time.Second
	const timeout = 5 * time.Minute

	deadline := time.Now().Add(timeout)
	lastStatus := ""

	for time.Now().Before(deadline) {
		env, err := client.GetEnvironment(ctx, envID)
		if err != nil {
			fmt.Printf("  ⚠ Poll error: %v\n", err)
			time.Sleep(pollInterval)
			continue
		}

		if env.Status != lastStatus {
			fmt.Printf("  → Status: %s\n", env.Status)
			lastStatus = env.Status
		}

		switch env.Status {
		case "ready":
			fmt.Println("  ✓ Environment is ready!")
			return nil
		case "failed":
			return fmt.Errorf("environment failed to provision")
		case "destroyed":
			return fmt.Errorf("environment was destroyed")
		}

		time.Sleep(pollInterval)
	}

	return fmt.Errorf("timeout waiting for environment (5 minutes)")
}

func configureCloudDNS(envName, project string, env *cloud.Environment) error {
	var hostsEntries []string
	dnsSuffix := ".test"

	if env != nil && len(env.Services) > 0 {
		for _, svc := range env.Services {
			if svc.IP != "" {
				hostname := fmt.Sprintf("%s.%s%s", svc.Name, envName, dnsSuffix)
				hostsEntries = append(hostsEntries, fmt.Sprintf("%s %s", svc.IP, hostname))
			}
		}
		if len(env.Services) > 0 {
			apexIP := env.Services[0].IP
			apexHostname := fmt.Sprintf("%s.%s%s", project, envName, dnsSuffix)
			hostsEntries = append(hostsEntries, fmt.Sprintf("%s %s", apexIP, apexHostname))
		}
	}

	if len(hostsEntries) > 0 {
		state := &models.State{
			Version: 2,
			Hosts:   make(map[string]*models.Host),
		}

		hostID := "cloud"
		if state.Hosts[hostID] == nil {
			state.Hosts[hostID] = &models.Host{
				Environments: make(map[string]*models.Environment),
			}
		}

		cloudEnv := &models.Environment{
			Name:      envName,
			Project:   project,
			DNSSuffix: dnsSuffix,
			Services:  make(map[string]*models.Service),
		}

		if env != nil {
			for _, svc := range env.Services {
				cloudEnv.Services[svc.Name] = &models.Service{
					Name:      svc.Name,
					IP:        svc.IP,
					IsIngress: false,
				}
			}
		}

		state.Hosts[hostID].Environments[envName] = cloudEnv

		if err := dns.UpdateDNSFromState(state); err != nil {
			fmt.Println("  → Manual DNS configuration required. Add these to /etc/hosts:")
			for _, entry := range hostsEntries {
				fmt.Printf("     %s\n", entry)
			}
			return err
		}
	}

	return nil
}

func fallbackSync(ctx context.Context, localPath, remoteIP, envID string) error {
	tarCmd := exec.CommandContext(ctx, "tar", "-czf", "-", "-C", localPath, ".")
	tarCmd.Dir = localPath

	remotePath := fmt.Sprintf("/var/cilo/envs/%s", envID)
	sshCmd := exec.CommandContext(ctx, "ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		fmt.Sprintf("root@%s", remoteIP),
		fmt.Sprintf("mkdir -p %s && tar -xzf - -C %s", remotePath, remotePath),
	)

	pipe, err := tarCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create pipe: %w", err)
	}
	sshCmd.Stdin = pipe
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr

	if err := sshCmd.Start(); err != nil {
		return fmt.Errorf("start ssh: %w", err)
	}

	if err := tarCmd.Run(); err != nil {
		return fmt.Errorf("tar: %w", err)
	}

	if err := sshCmd.Wait(); err != nil {
		return fmt.Errorf("ssh: %w", err)
	}

	return nil
}
