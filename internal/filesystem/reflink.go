// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package filesystem

import (
	"io"
	"os"
	"runtime"
	"syscall"
)

// FICLONE is the ioctl number for Linux reflink (FICLONE)
const FICLONE = 0x40049409

// CopyFile attempts to use reflink (CoW) to copy a file.
// It falls back to standard io.Copy if reflink is not supported.
func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	// Try reflink (CoW) first
	if tryReflink(sourceFile, destFile) {
		return nil
	}

	// Fallback to standard copy
	_, err = io.Copy(destFile, sourceFile)
	return err
}

func tryReflink(src, dst *os.File) bool {
	switch runtime.GOOS {
	case "linux":
		// Linux FICLONE ioctl
		_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, dst.Fd(), FICLONE, src.Fd())
		return errno == 0
	case "darwin":
		// macOS clonefile is not exposed in syscall package easily for FDs.
		// In a real world production tool we would use CGO or fclonefileat.
		return false
	default:
		return false
	}
}
