//go:build linux || darwin

package dashboard

import (
	"golang.org/x/sys/unix"
	"golang.org/x/term"
	"os"
)

// An external attach process may be killed before restoring its own raw mode.
// Preserve the caller's cooked TTY and descriptor flags independently of it.
func terminalGuard() func() {
	fd := int(os.Stdin.Fd())
	state, e := term.GetState(fd)
	if e != nil {
		return func() {}
	}
	inputs := []uintptr{os.Stdin.Fd(), os.Stdout.Fd()}
	flags := []int{-1, -1}
	for i, descriptor := range inputs {
		if value, e := unix.FcntlInt(descriptor, unix.F_GETFL, 0); e == nil {
			flags[i] = value
		}
	}
	return func() {
		_ = term.Restore(fd, state)
		for i, descriptor := range inputs {
			if flags[i] >= 0 {
				_, _ = unix.FcntlInt(descriptor, unix.F_SETFL, flags[i])
			}
		}
	}
}
