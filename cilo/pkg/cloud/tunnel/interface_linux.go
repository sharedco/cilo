//go:build linux

package tunnel

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

// CreateInterface creates a new WireGuard network interface (Linux only)
// Returns the actual interface name (same as requested on Linux)
func CreateInterface(name string) (string, error) {
	// Check if interface already exists
	_, err := netlink.LinkByName(name)
	if err == nil {
		// Interface exists, remove it first
		if err := RemoveInterface(name); err != nil {
			return "", fmt.Errorf("remove existing interface: %w", err)
		}
	}

	// Create WireGuard interface
	wgLink := &netlink.Wireguard{
		LinkAttrs: netlink.LinkAttrs{
			Name: name,
		},
	}

	if err := netlink.LinkAdd(wgLink); err != nil {
		return "", fmt.Errorf("create interface: %w", err)
	}

	return name, nil
}

// RemoveInterface removes a WireGuard network interface
func RemoveInterface(name string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("find interface: %w", err)
	}

	if err := netlink.LinkDel(link); err != nil {
		return fmt.Errorf("delete interface: %w", err)
	}

	return nil
}

// SetInterfaceUp brings the interface up
func SetInterfaceUp(name string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("find interface: %w", err)
	}

	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("set interface up: %w", err)
	}

	return nil
}

// AddAddress adds an IP address to the interface
func AddAddress(name string, cidr string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("find interface: %w", err)
	}

	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		return fmt.Errorf("parse address: %w", err)
	}

	if err := netlink.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("add address: %w", err)
	}

	return nil
}

// AddRoute adds a route through the interface
func AddRoute(name string, destination string, gateway net.IP) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("find interface: %w", err)
	}

	_, dst, err := net.ParseCIDR(destination)
	if err != nil {
		return fmt.Errorf("parse destination: %w", err)
	}

	route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       dst,
		Gw:        gateway,
	}

	if err := netlink.RouteAdd(route); err != nil {
		return fmt.Errorf("add route: %w", err)
	}

	return nil
}
