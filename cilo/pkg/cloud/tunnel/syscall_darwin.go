//go:build darwin

package tunnel

import (
	"fmt"
	"syscall"
	"unsafe"
)

// getInterfaceNameFromFD retrieves the interface name from a TUN file descriptor
// using the SIOCGIFNAME ioctl on macOS
func getInterfaceNameFromFD(fd int) (string, error) {
	// Buffer for interface name (IFNAMSIZ is typically 16 on macOS)
	var ifName [16]byte

	// SIOCGIFNAME ioctl to get interface name from fd
	// On macOS, this is defined in sys/sockio.h
	const SIOCGIFNAME = 0xc0106922 // _IOWR('i', 34, struct ifreq)

	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(SIOCGIFNAME),
		uintptr(unsafe.Pointer(&ifName[0])),
	)

	if errno != 0 {
		return "", fmt.Errorf("ioctl SIOCGIFNAME failed: %v", errno)
	}

	// Convert to string (null-terminated)
	nameLen := 0
	for i, b := range ifName {
		if b == 0 {
			nameLen = i
			break
		}
	}

	return string(ifName[:nameLen]), nil
}
