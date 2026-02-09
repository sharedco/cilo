// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

//go:build darwin

package tunnel

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// getInterfaceNameFromFD retrieves the interface name from a TUN file descriptor.
// On macOS, utun interfaces are created as utun[0-9]+, we detect the actual name
// by scanning network interfaces after creation.
func getInterfaceNameFromFD(fd int) (string, error) {
	// The utun interface name is assigned by the kernel when we create the TUN.
	// Since the interface just got created, it's typically the highest numbered utun.
	// We'll scan for utun interfaces and find the one that was just created.

	interfaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("list interfaces: %w", err)
	}

	// Find the highest utun interface number
	maxNum := -1
	for _, iface := range interfaces {
		name := iface.Name
		if strings.HasPrefix(name, "utun") {
			// Extract number from utun[0-9]+
			numStr := strings.TrimPrefix(name, "utun")
			if num, err := strconv.Atoi(numStr); err == nil && num > maxNum {
				maxNum = num
			}
		}
	}

	if maxNum >= 0 {
		return fmt.Sprintf("utun%d", maxNum), nil
	}

	return "", fmt.Errorf("could not determine utun interface name")
}
