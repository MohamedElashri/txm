package runtimepath

import "golang.org/x/sys/unix"

// macOS path awaits its runtime gate. A crash between link/unlink is rejected
// by the single-link reader rather than accepted as a completed publication.
func publishExclusive(fd int, from, to string) error {
	if e := unix.Linkat(fd, from, fd, to, 0); e != nil {
		return e
	}
	return unix.Unlinkat(fd, from, 0)
}
