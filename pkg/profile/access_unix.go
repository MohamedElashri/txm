//go:build linux || darwin

package profile

import "golang.org/x/sys/unix"

func searchable(path string) error { return unix.Access(path, unix.X_OK) }
