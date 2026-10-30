package runtimepath

import "golang.org/x/sys/unix"

func openNameLock(fd int, name string) (int, error) {
	return unix.Openat(fd, name, unix.O_RDWR|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK|unix.O_NOCTTY, 0600)
}
