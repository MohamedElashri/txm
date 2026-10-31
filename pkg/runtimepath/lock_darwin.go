package runtimepath

import "golang.org/x/sys/unix"

func openNameLock(fd int, name string) (int, error) {
	// Darwin can return ENOENT from contending O_CREAT opens of the same new
	// file. Separate lookup and exclusive creation, then reopen the winner.
	// Every lookup stays anchored to the private directory and refuses symlinks.
	flags := unix.O_RDWR | unix.O_NOFOLLOW | unix.O_CLOEXEC | unix.O_NONBLOCK | unix.O_NOCTTY
	for range 8 {
		file, err := unix.Openat(fd, name, flags, 0)
		if err != unix.ENOENT {
			return file, err
		}
		file, err = unix.Openat(fd, name, flags|unix.O_CREAT|unix.O_EXCL, 0600)
		if err != unix.EEXIST {
			return file, err
		}
	}
	return -1, unix.EAGAIN
}
