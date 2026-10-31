//go:build linux && cgo

package nativeprocess

import "golang.org/x/sys/unix"

// SignalGroup requires an unreaped direct child reserving the PGID.
func SignalGroup(pid int, signal unix.Signal) error { return unix.Kill(-pid, signal) }
