//go:build darwin

package tunnel

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
)

const (
	defaultMTU = 1420
)

// deviceRegistry stores userspace WireGuard devices by interface name
var (
	deviceRegistry = make(map[string]*device.Device)
	tunRegistry    = make(map[string]tun.Device)
	registryMu     sync.RWMutex
)

// CreateInterface creates a WireGuard interface on macOS using wireguard-go userspace implementation.
// On macOS, the interface name must be "utun" for auto-assignment or "utun[0-9]+" for explicit assignment.
// The actual assigned name (e.g., "utun4") is returned.
func CreateInterface(name string) (string, error) {
	// On macOS, we use "utun" to let the kernel auto-assign the interface number
	// The actual name will be something like "utun0", "utun1", etc.
	tunDev, err := tun.CreateTUN("utun", defaultMTU)
	if err != nil {
		return "", fmt.Errorf("create TUN device: %w", err)
	}

	// Get the actual interface name assigned by the kernel
	actualName, err := getTUNInterfaceName(tunDev)
	if err != nil {
		tunDev.Close()
		return "", fmt.Errorf("get TUN interface name: %w", err)
	}

	// Create userspace WireGuard device
	logger := device.NewLogger(device.LogLevelError, "(cilo-wg)")
	bind := conn.NewDefaultBind()
	wgDev := device.NewDevice(tunDev, bind, logger)

	// Store in registry for later access by Manager
	registryMu.Lock()
	deviceRegistry[actualName] = wgDev
	tunRegistry[actualName] = tunDev
	registryMu.Unlock()

	return actualName, nil
}

// RemoveInterface removes a WireGuard interface on macOS
func RemoveInterface(name string) error {
	registryMu.Lock()
	defer registryMu.Unlock()

	// Stop and clean up the userspace device
	if wgDev, exists := deviceRegistry[name]; exists {
		wgDev.Close()
		delete(deviceRegistry, name)
	}

	// Close the TUN device
	if tunDev, exists := tunRegistry[name]; exists {
		tunDev.Close()
		delete(tunRegistry, name)
	}

	return nil
}

// SetInterfaceUp brings the interface up on macOS
func SetInterfaceUp(name string) error {
	// On macOS with userspace WireGuard, the interface is brought up
	// by sending the EventUp to the device. However, we also use
	// ifconfig to ensure the system knows it's up.
	cmd := exec.Command("ifconfig", name, "up")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("set interface up: %w", err)
	}

	// Signal the device to start processing
	registryMu.RLock()
	wgDev, exists := deviceRegistry[name]
	registryMu.RUnlock()

	if exists {
		wgDev.Up()
	}

	return nil
}

// AddAddress adds an IP address on macOS
func AddAddress(name string, cidr string) error {
	// Parse CIDR to get IP and mask
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("parse CIDR: %w", err)
	}

	// Get mask size
	maskSize, _ := ipNet.Mask.Size()

	// macOS ifconfig syntax: ifconfig utunX inet IP/prefix destination
	// For point-to-point WireGuard, we use the same IP for destination
	cmd := exec.Command("ifconfig", name, "inet", fmt.Sprintf("%s/%d", ip.String(), maskSize), ip.String())
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("add address: %w", err)
	}

	return nil
}

// AddRoute adds a route on macOS
func AddRoute(name string, destination string, gateway net.IP) error {
	// Parse destination CIDR
	_, dstNet, err := net.ParseCIDR(destination)
	if err != nil {
		return fmt.Errorf("parse destination: %w", err)
	}

	// Get mask size
	maskSize, _ := dstNet.Mask.Size()

	// macOS route command: route add -net destination/prefix gateway
	cmd := exec.Command("route", "add", "-net", fmt.Sprintf("%s/%d", dstNet.IP.String(), maskSize), gateway.String())
	if err := cmd.Run(); err != nil {
		// Route might already exist, which is okay
		if !strings.Contains(err.Error(), "file exists") {
			return fmt.Errorf("add route: %w", err)
		}
	}

	return nil
}

// getTUNInterfaceName extracts the actual interface name from a TUN device
func getTUNInterfaceName(tunDev tun.Device) (string, error) {
	// The TUN device file descriptor can be used to determine the interface name
	// On macOS, we can get this from the device file
	file := tunDev.File()
	if file == nil {
		return "", fmt.Errorf("TUN device has no file descriptor")
	}

	// Use SIOCGIFNAME ioctl to get the interface name
	// This requires syscall access
	return getInterfaceNameFromFD(int(file.Fd()))
}

// getDevice returns the userspace device for the given interface name
func getDevice(name string) (*device.Device, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	dev, exists := deviceRegistry[name]
	return dev, exists
}
