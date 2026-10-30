package runtimepath

import "golang.org/x/sys/unix"

// One atomic operation publishes the complete single-link file without replacing
// an existing receipt/manifest, including across concurrent writers or crashes.
func publishExclusive(fd int, from, to string) error {
	return unix.RenameatxNp(fd, from, fd, to, unix.RENAME_EXCL)
}
