// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/sharedco/cilo/internal/cilod"
	"github.com/spf13/cobra"
)

type MachineInfo struct {
	Host        string    `json:"host"`
	WGIP        string    `json:"wg_ip"`
	Status      string    `json:"status"`
	EnvCount    int       `json:"env_count"`
	ConnectedAt time.Time `json:"connected_at"`
}

var machinesCmd = &cobra.Command{
	Use:   "machines",
	Short: "List connected machines",
	Long: `List all connected machines with their status and environment counts.

Shows a table of machines with:
  - Machine hostname
  - Connection status (connected or unreachable)
  - Number of environments on each machine
  - Connection time (e.g., "2h ago", "5d ago")

Use --json for machine-readable output.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonFlag, _ := cmd.Flags().GetBool("json")
		out := cmd.OutOrStdout()

		machines, err := ListConnectedMachines()
		if err != nil {
			return fmt.Errorf("failed to list machines: %w", err)
		}

		if len(machines) == 0 {
			fmt.Fprintln(out, "No connected machines")
			return nil
		}

		machineInfos := make([]MachineInfo, 0, len(machines))
		for _, m := range machines {
			info := MachineInfo{
				Host:        m.Host,
				WGIP:        m.WGAssignedIP,
				ConnectedAt: m.ConnectedAt,
			}

			client := cilod.NewClient(m.WGAssignedIP, m.Token)
			client.SetTimeout(5 * time.Second)
			envs, err := client.ListEnvironments()
			if err != nil {
				info.Status = "unreachable"
				info.EnvCount = 0
			} else {
				info.Status = "connected"
				info.EnvCount = len(envs)
			}

			machineInfos = append(machineInfos, info)
		}

		if jsonFlag {
			return machinesJSON(out, machineInfos)
		}
		return machinesTable(out, machineInfos)
	},
}

func machinesTable(out io.Writer, infos []MachineInfo) error {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "MACHINE\tSTATUS\tENVS\tCONNECTED SINCE\t\n")
	fmt.Fprintf(w, "-------\t------\t----\t---------------\t\n")

	for _, info := range infos {
		connectedSince := formatDuration(time.Since(info.ConnectedAt))
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t\n",
			info.Host,
			info.Status,
			info.EnvCount,
			connectedSince,
		)
	}

	return w.Flush()
}

func machinesJSON(out io.Writer, infos []MachineInfo) error {
	output := make([]map[string]interface{}, 0, len(infos))

	for _, info := range infos {
		output = append(output, map[string]interface{}{
			"host":         info.Host,
			"wg_ip":        info.WGIP,
			"status":       info.Status,
			"env_count":    info.EnvCount,
			"connected_at": info.ConnectedAt,
		})
	}

	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	if d < 7*24*time.Hour {
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
	if d < 30*24*time.Hour {
		return fmt.Sprintf("%dw ago", int(d.Hours()/24/7))
	}
	return fmt.Sprintf("%dmo ago", int(d.Hours()/24/30))
}

func init() {
	machinesCmd.Flags().Bool("json", false, "Output in JSON format")
}
