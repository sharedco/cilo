package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cilo/cilo/pkg/config"
	"github.com/cilo/cilo/pkg/dns"
	"github.com/spf13/cobra"
)

var dnsCmd = &cobra.Command{
	Use:   "dns",
	Short: "DNS management commands",
	Long:  `Manage DNS configuration for cilo environments.`,
}

var dnsSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Setup DNS configuration (requires sudo)",
	Long: `Configure system DNS resolver to use cilo's dnsmasq for *.test domains.

This command requires elevated permissions to modify system DNS settings.
If you prefer to configure manually, see the commands below.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		printManual := cmd.Flags().Changed("print-manual")

		if printManual {
			printDNSManual()
			return nil
		}

		fmt.Println("Setting up DNS for cilo...")
		fmt.Println()

		if err := checkDNSMasqRunning(); err != nil {
			fmt.Println("⚠ dnsmasq is not running. Starting it...")
			if err := dns.SetupDNS(); err != nil {
				fmt.Printf("✗ Failed to start dnsmasq: %v\n", err)
				fmt.Println()
				printDNSManual()
				return fmt.Errorf("dns setup failed")
			}
		}

		fmt.Println("✓ dnsmasq is running")
		fmt.Println()

		if runtime.GOOS == "darwin" {
			fmt.Println("Detected macOS. Configuring DNS resolver...")
			if err := setupMacOSResolver(); err != nil {
				fmt.Printf("⚠ Could not auto-configure: %v\n", err)
				fmt.Println()
				printDNSManual()
				return nil
			}
		} else {
			fmt.Println("Detected Linux. Configuring systemd-resolved...")
			if err := setupLinuxResolver(); err != nil {
				fmt.Printf("⚠ Could not auto-configure: %v\n", err)
				fmt.Println()
				printDNSManual()
				return nil
			}
		}

		fmt.Println()
		fmt.Println("✓ DNS configured successfully!")
		fmt.Println()
		fmt.Println("Test it:")
		fmt.Println("  ping nginx.demo.test")
		fmt.Println()
		fmt.Println("Note: You may need to restart your browser or run 'sudo resolvectl flush-caches'")

		return nil
	},
}

var dnsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check DNS status",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("DNS Status:")
		fmt.Println()

		if err := checkDNSMasqRunning(); err != nil {
			fmt.Println("✗ dnsmasq: not running")
			fmt.Println("  Run: cilo dns setup")
		} else {
			fmt.Println("✓ dnsmasq: running")
		}

		if runtime.GOOS == "darwin" {
			if _, err := os.Stat("/etc/resolver/test"); err == nil {
				fmt.Println("✓ macOS resolver: configured")
			} else {
				fmt.Println("✗ macOS resolver: not configured")
				fmt.Println("  Run: cilo dns setup")
			}
		} else {
			if _, err := os.Stat("/etc/systemd/resolved.conf.d/cilo.conf"); err == nil {
				fmt.Println("✓ systemd-resolved: configured")
			} else {
				fmt.Println("✗ systemd-resolved: not configured")
				fmt.Println("  Run: cilo dns setup")
			}
		}

		fmt.Println()
		fmt.Println("Testing DNS resolution...")
		if out, err := exec.Command("dig", "@127.0.0.1", "-p", "5354", "nginx.demo.test").CombinedOutput(); err == nil && strings.Contains(string(out), "10.224") {
			fmt.Println("✓ Direct dnsmasq query: working")
		} else {
			fmt.Println("✗ Direct dnsmasq query: failed")
		}

		if out, err := exec.Command("resolvectl", "query", "nginx.demo.test").CombinedOutput(); err == nil && strings.Contains(string(out), "10.224") {
			fmt.Println("✓ System DNS resolution: working")
		} else {
			fmt.Println("✗ System DNS resolution: not working")
			fmt.Println("  Make sure systemd-resolved is configured and restarted")
		}

		return nil
	},
}

var dnsLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show dnsmasq logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Checking dnsmasq status...")

		if err := checkDNSMasqRunning(); err != nil {
			fmt.Println("dnsmasq is not running")
			fmt.Println("Run: cilo dns setup")
			return nil
		}

		dnsDir := config.GetDNSDir()
		configPath := filepath.Join(dnsDir, "dnsmasq.conf")

		data, err := os.ReadFile(configPath)
		if err != nil {
			fmt.Printf("Could not read config: %v\n", err)
			return nil
		}

		fmt.Println("dnsmasq configuration:")
		fmt.Println(string(data))

		return nil
	},
}

func checkDNSMasqRunning() error {
	cmd := exec.Command("pgrep", "-x", "dnsmasq")
	return cmd.Run()
}

func setupLinuxResolver() error {
	confDir := "/etc/systemd/resolved.conf.d"
	confFile := filepath.Join(confDir, "cilo.conf")

	if _, err := os.Stat(confFile); err == nil {
		fmt.Println("✓ DNS resolver already configured")
		return nil
	}

	config := `[Resolve]
DNS=127.0.0.1:5354
Domains=~test
DNSStubListener=yes
`

	if err := os.MkdirAll(confDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(confFile, []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Println("✓ Created /etc/systemd/resolved.conf.d/cilo.conf")

	if err := exec.Command("systemctl", "restart", "systemd-resolved").Run(); err != nil {
		return fmt.Errorf("failed to restart systemd-resolved: %w", err)
	}

	fmt.Println("✓ Restarted systemd-resolved")
	return nil
}

func setupMacOSResolver() error {
	resolverDir := "/etc/resolver"
	resolverFile := filepath.Join(resolverDir, "test")

	if _, err := os.Stat(resolverFile); err == nil {
		fmt.Println("✓ DNS resolver already configured")
		return nil
	}

	config := "nameserver 127.0.0.1\nport 5354\n"

	if err := os.MkdirAll(resolverDir, 0755); err != nil {
		return fmt.Errorf("requires sudo privileges: %w", err)
	}

	if err := os.WriteFile(resolverFile, []byte(config), 0644); err != nil {
		return fmt.Errorf("requires sudo privileges: %w", err)
	}

	fmt.Println("✓ Created /etc/resolver/test")
	return nil
}

func printDNSManual() {
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("MANUAL DNS SETUP REQUIRED")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	if runtime.GOOS == "darwin" {
		fmt.Println("Run these commands with sudo:")
		fmt.Println()
		fmt.Println("  sudo mkdir -p /etc/resolver")
		fmt.Println("  echo 'nameserver 127.0.0.1' | sudo tee /etc/resolver/test")
		fmt.Println("  echo 'port 5354' | sudo tee -a /etc/resolver/test")
	} else {
		fmt.Println("Run these commands with sudo:")
		fmt.Println()
		fmt.Println("  sudo mkdir -p /etc/systemd/resolved.conf.d")
		fmt.Println("  sudo tee /etc/systemd/resolved.conf.d/cilo.conf << 'EOF'")
		fmt.Println("  [Resolve]")
		fmt.Println("  DNS=127.0.0.1:5354")
		fmt.Println("  Domains=~test")
		fmt.Println("  DNSStubListener=yes")
		fmt.Println("  EOF")
		fmt.Println("  sudo systemctl restart systemd-resolved")
	}

	fmt.Println()
	fmt.Println("After running these commands, test with:")
	fmt.Println("  ping nginx.demo.test")
	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
}

func init() {
	dnsCmd.AddCommand(dnsSetupCmd)
	dnsCmd.AddCommand(dnsStatusCmd)
	dnsCmd.AddCommand(dnsLogsCmd)

	dnsSetupCmd.Flags().Bool("print-manual", false, "Print manual setup instructions")
}
