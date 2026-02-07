package cmd

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/sharedco/cilo/pkg/dns"
	"github.com/sharedco/cilo/pkg/reconcile"
	"github.com/sharedco/cilo/pkg/runtime/docker"
	"github.com/sharedco/cilo/pkg/share"
	"github.com/sharedco/cilo/pkg/state"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check and repair cilo configuration",
	Long: `Doctor checks the health of your cilo installation and can fix common issues.

Checks performed:
- Docker daemon availability
- dnsmasq process status
- State/runtime synchronization
- Orphaned Docker resources

Use --fix to automatically repair issues.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fix, _ := cmd.Flags().GetBool("fix")

		fmt.Println("🔍 Checking cilo configuration...")
		fmt.Println()

		// Check Docker
		fmt.Print("Checking Docker... ")
		if err := checkDocker(); err != nil {
			fmt.Printf("❌ %v\n", err)
		} else {
			fmt.Println("✅")
		}

		// Check dnsmasq
		fmt.Print("Checking dnsmasq... ")
		if err := checkDNSMasq(); err != nil {
			fmt.Printf("❌ %v\n", err)
		} else {
			fmt.Println("✅")
		}

		// Load state
		fmt.Print("\nLoading state... ")
		st, err := state.LoadState()
		if err != nil {
			fmt.Printf("❌ %v\n", err)
			return nil
		}

		// Count environments from all hosts
		envCount := 0
		for _, host := range st.Hosts {
			envCount += len(host.Environments)
		}
		fmt.Printf("✅ (%d environments)\n", envCount)

		// Reconcile environments
		fmt.Println("\n📊 Reconciling environments...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result := reconcile.All(ctx, st)

		fmt.Printf("  Reconciled: %d environments\n", result.EnvsReconciled)
		if len(result.EnvsNotRunning) > 0 {
			fmt.Printf("  Not running: %v\n", result.EnvsNotRunning)
		}
		if len(result.Errors) > 0 {
			fmt.Printf("  Errors: %d\n", len(result.Errors))
			for _, err := range result.Errors {
				fmt.Printf("    - %v\n", err)
			}
		}

		// Check for orphans
		fmt.Println("\n🔎 Checking for orphaned resources...")
		orphans, err := reconcile.FindOrphans(ctx, st)
		if err != nil {
			fmt.Printf("  ⚠️  Could not check orphans: %v\n", err)
		} else if len(orphans) > 0 {
			fmt.Printf("  Found %d orphaned resources:\n", len(orphans))
			for _, o := range orphans {
				fmt.Printf("    - %s: %s\n", o.Type, o.Name)
			}
		} else {
			fmt.Println("  ✅ No orphaned resources")
		}

		// Check shared services
		fmt.Println("\n🔎 Checking shared services...")
		provider := docker.NewProvider()
		sharedIssues, err := share.CheckSharedServices(st, provider, ctx)
		if err != nil {
			fmt.Printf("  ⚠️  Could not check shared services: %v\n", err)
		} else if len(sharedIssues) > 0 {
			fmt.Printf("  Found %d issues:\n", len(sharedIssues))
			for _, issue := range sharedIssues {
				var emoji string
				switch issue.Type {
				case "orphaned":
					emoji = "🗑️ "
				case "missing":
					emoji = "❓"
				case "stale_grace":
					emoji = "⏰"
				case "stopped":
					emoji = "⏸️ "
				default:
					emoji = "⚠️ "
				}
				fmt.Printf("    %s %s: %s\n", emoji, issue.Type, issue.Detail)
			}
		} else {
			fmt.Println("  ✅ No shared service issues")
		}

		// Fix if requested
		if fix {
			fmt.Println("\n🔧 Applying fixes...")

			// Fix shared services
			if len(sharedIssues) > 0 {
				fmt.Print("  Fixing orphaned shared services... ")
				fixed, err := share.FixOrphanedServices(st, provider, ctx)
				if err != nil {
					fmt.Printf("❌ %v\n", err)
				} else {
					fmt.Printf("✅ (fixed %d)\n", fixed)
				}

				fmt.Print("  Fixing stale grace periods... ")
				fixed, err = share.FixStaleGracePeriods(st, provider, ctx)
				if err != nil {
					fmt.Printf("❌ %v\n", err)
				} else {
					fmt.Printf("✅ (fixed %d)\n", fixed)
				}

				fmt.Print("  Cleaning up missing service entries... ")
				fixed, err = share.FixMissingServices(st, provider, ctx)
				if err != nil {
					fmt.Printf("❌ %v\n", err)
				} else {
					fmt.Printf("✅ (fixed %d)\n", fixed)
				}
			}

			// Save reconciled state
			fmt.Print("  Saving state... ")
			if err := state.SaveState(st); err != nil {
				fmt.Printf("❌ %v\n", err)
			} else {
				fmt.Println("✅")
			}

			// Regenerate DNS
			fmt.Print("  Regenerating DNS... ")
			if err := dns.UpdateDNSFromState(st); err != nil {
				fmt.Printf("❌ %v\n", err)
			} else {
				fmt.Println("✅")
			}
		} else if len(result.Errors) > 0 || len(orphans) > 0 || len(sharedIssues) > 0 {
			fmt.Println("\n💡 Run 'cilo doctor --fix' to repair issues")
		}

		fmt.Println("\n✨ Doctor check complete")
		return nil
	},
}

func init() {
	doctorCmd.Flags().Bool("fix", false, "Automatically fix issues")
	rootCmd.AddCommand(doctorCmd)
}

func checkDocker() error {
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker not available: %w", err)
	}
	return nil
}

func checkDNSMasq() error {
	cmd := exec.Command("pgrep", "-x", "dnsmasq")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("dnsmasq not running")
	}
	return nil
}
