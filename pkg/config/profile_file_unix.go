//go:build linux || darwin

package config

import (
	"fmt"
	"golang.org/x/sys/unix"
	"os"
)

func openProfile(path string) (*os.File, error) {
	fd, e := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(fd), path)
	st, e := f.Stat()
	if e == nil && !st.Mode().IsRegular() {
		e = fmt.Errorf("profile must be a regular file")
	}
	if e != nil {
		_ = f.Close()
		return nil, e
	}
	return f, nil
}
