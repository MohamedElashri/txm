//go:build linux || darwin

package metrics

import (
	"syscall"
)

func errWouldBlock() error { return syscall.EWOULDBLOCK }
