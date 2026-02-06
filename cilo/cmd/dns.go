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
	"github.com/cilo/cilo/pkg/state"
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
		dnsSuffix, _ := cmd.Flags().GetString("dns-suffix")
		if dnsSuffix == "" {
			dnsSuffix = ".test"
		}
		// Remove leading dot for system config if present, we'll add it where needed
		cleanSuffix := strings.TrimPrefix(dnsSuffix, ".")

		if printManual {
			printDNSManual(cleanSuffix)
			return nil
		}

		fmt.Printf("Setting up DNS for cilo (suffix: .%s)...\n", cleanSuffix)
		fmt.Println()

		if err := checkDNSMasqRunning(); err != nil {
			fmt.Println("⚠ dnsmasq is not running. Starting it...")
			st, _ := state.LoadState()
			if err := dns.SetupDNS(st); err != nil {
				fmt.Printf("✗ Failed to start dnsmasq: %v\n", err)
				fmt.Println()
				printDNSManual(cleanSuffix)
				return fmt.Errorf("dns setup failed")
			}
		}

		fmt.Println("✓ dnsmasq is running")
		fmt.Println()

		if runtime.GOOS == "darwin" {
			fmt.Println("Detected macOS. Configuring DNS resolver...")
			if err := setupMacOSResolver(cleanSuffix); err != nil {
				fmt.Printf("⚠ Could not auto-configure: %v\n", err)
				fmt.Println()
				printDNSManual(cleanSuffix)
				return nil
			}
		} else {
			fmt.Println("Detected Linux. Configuring systemd-resolved...")
			if err := setupLinuxResolver(cleanSuffix); err != nil {
				fmt.Printf("⚠ Could not auto-configure: %v\n", err)
				fmt.Println()
				printDNSManual(cleanSuffix)
				return nil
			}
		}

		fmt.Println()
		fmt.Println("✓ DNS configured successfully!")
		fmt.Println()
		fmt.Println("Test it:")
		fmt.Printf("  ping nginx.demo.%s\n", cleanSuffix)
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

		// Detect all configured suffixes
		var suffixes []string
		if runtime.GOOS == "darwin" {
			if entries, err := os.ReadDir("/etc/resolver"); err == nil {
				for _, entry := range entries {
					if entry.Name() != "README" {
						suffixes = append(suffixes, entry.Name())
					}
				}
			}
		} else {
			if data, err := os.ReadFile("/etc/systemd/resolved.conf.d/cilo.conf"); err == nil {
				lines := strings.Split(string(data), "\n")
				for _, line := range lines {
					if strings.HasPrefix(line, "Domains=") {
						domains := strings.TrimPrefix(line, "Domains=")
						parts := strings.Fields(domains)
						for _, p := range parts {
							suffixes = append(suffixes, strings.TrimPrefix(p, "~"))
						}
					}
				}
			}
		}

		if len(suffixes) == 0 {
			fmt.Println("✗ No DNS suffixes configured in system resolver")
			fmt.Println("  Run: cilo dns setup")
		} else {
			fmt.Printf("✓ System resolver configured for: .%s\n", strings.Join(suffixes, ", ."))
		}

		fmt.Println()
		fmt.Println("Testing DNS resolution...")
		for _, s := range suffixes {
			testHost := fmt.Sprintf("nginx.demo.%s", s)
			if out, err := exec.Command("dig", "@127.0.0.1", "-p", "5354", testHost).CombinedOutput(); err == nil && strings.Contains(string(out), "10.224") {
				fmt.Printf("✓ Direct dnsmasq query (%s): working\n", testHost)
			} else {
				fmt.Printf("✗ Direct dnsmasq query (%s): failed (ensure an environment exists for this suffix)\n", testHost)
			}

			if out, err := exec.Command("resolvectl", "query", testHost).CombinedOutput(); err == nil && strings.Contains(string(out), "10.224") {
				fmt.Printf("✓ System DNS resolution (%s): working\n", testHost)
			} else {
				fmt.Printf("✗ System DNS resolution (%s): not working\n", testHost)
			}
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

func setupLinuxResolver(suffix string) error {
	confDir := "/etc/systemd/resolved.conf.d"
	confFile := filepath.Join(confDir, "cilo.conf")

	domains := []string{"~" + suffix}

	// Try to read existing config to be additive
	if data, err := os.ReadFile(confFile); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "Domains=") {
				existing := strings.TrimPrefix(line, "Domains=")
				parts := strings.Fields(existing)
				for _, p := range parts {
					if p != "~"+suffix {
						domains = append(domains, p)
					}
				}
			}
		}
	}

	config := fmt.Sprintf(`[Resolve]
DNS=127.0.0.1:5354
Domains=%s
DNSStubListener=yes
`, strings.Join(domains, " "))

	if err := os.MkdirAll(confDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(confFile, []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Printf("✓ Updated /etc/systemd/resolved.conf.d/cilo.conf (Domains=%s)\n", strings.Join(domains, " "))

	if err := exec.Command("systemctl", "restart", "systemd-resolved").Run(); err != nil {
		return fmt.Errorf("failed to restart systemd-resolved: %w", err)
	}

	fmt.Println("✓ Restarted systemd-resolved")
	return nil
}

func setupMacOSResolver(suffix string) error {
	resolverDir := "/etc/resolver"
	resolverFile := filepath.Join(resolverDir, suffix)

	config := "nameserver 127.0.0.1\nport 5354\n"

	if err := os.MkdirAll(resolverDir, 0755); err != nil {
		return fmt.Errorf("requires sudo privileges: %w", err)
	}

	if err := os.WriteFile(resolverFile, []byte(config), 0644); err != nil {
		return fmt.Errorf("requires sudo privileges: %w", err)
	}

	fmt.Printf("✓ Created /etc/resolver/%s\n", suffix)
	return nil
}

func printDNSManual(suffix string) {
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("MANUAL DNS SETUP REQUIRED")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	if runtime.GOOS == "darwin" {
		fmt.Println("Run these commands with sudo:")
		fmt.Println()
		fmt.Println("  sudo mkdir -p /etc/resolver")
		fmt.Printf("  echo 'nameserver 127.0.0.1' | sudo tee /etc/resolver/%s\n", suffix)
		fmt.Printf("  echo 'port 5354' | sudo tee -a /etc/resolver/%s\n", suffix)
	} else {
		fmt.Println("Run these commands with sudo:")
		fmt.Println()
		fmt.Println("  sudo mkdir -p /etc/systemd/resolved.conf.d")
		fmt.Println("  sudo tee /etc/systemd/resolved.conf.d/cilo.conf << 'EOF'")
		fmt.Println("  [Resolve]")
		fmt.Println("  DNS=127.0.0.1:5354")
		fmt.Printf("  Domains=~%s\n", suffix)
		fmt.Println("  DNSStubListener=yes")
		fmt.Println("  EOF")
		fmt.Println("  sudo systemctl restart systemd-resolved")
	}

	fmt.Println()
	fmt.Println("After running these commands, test with:")
	fmt.Printf("  ping nginx.demo.%s\n", suffix)
	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
}

func init() {
	dnsCmd.AddCommand(dnsSetupCmd)
	dnsCmd.AddCommand(dnsStatusCmd)
	dnsCmd.AddCommand(dnsLogsCmd)

	dnsSetupCmd.Flags().Bool("print-manual", false, "Print manual setup instructions")
	dnsSetupCmd.Flags().String("dns-suffix", ".test", "DNS suffix to configure (default: .test)")
}
