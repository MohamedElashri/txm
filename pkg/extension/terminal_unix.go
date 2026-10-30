//go:build linux || darwin

package extension

import (
	"golang.org/x/sys/unix"
	"golang.org/x/term"
	"os"
)

func TerminalGuard() func() {
	fd := int(os.Stdin.Fd())
	state, e := term.GetState(fd)
	if e != nil {
		return func() {}
	}
	fds := []uintptr{os.Stdin.Fd(), os.Stdout.Fd()}
	flags := []int{-1, -1}
	for i, d := range fds {
		if v, e := unix.FcntlInt(d, unix.F_GETFL, 0); e == nil {
			flags[i] = v
		}
	}
	return func() {
		_ = term.Restore(fd, state)
		for i, d := range fds {
			if flags[i] >= 0 {
				_, _ = unix.FcntlInt(d, unix.F_SETFL, flags[i])
			}
		}
	}
}
