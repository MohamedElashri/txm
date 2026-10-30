package runtimepath

import "golang.org/x/sys/unix"

// An immutable file becomes visible with one link, without an overwrite window.
func publishExclusive(fd int, from, to string) error {
	return unix.Renameat2(fd, from, fd, to, unix.RENAME_NOREPLACE)
}
