//go:build linux || darwin

package extension

import (
	"errors"
	"github.com/MohamedElashri/txm/pkg/core"
	"golang.org/x/sys/unix"
	"os"
)

type Identity struct {
	Device uint64 `json:"device"`
	Inode  uint64 `json:"inode"`
}

func Identify(p string) (Identity, error) {
	var s unix.Stat_t
	if e := NoSymlinks(p); e != nil {
		return Identity{}, e
	}
	if e := unix.Lstat(p, &s); e != nil {
		return Identity{}, e
	}
	if s.Uid != uint32(os.Geteuid()) || s.Mode&unix.S_IFMT == unix.S_IFLNK {
		return Identity{}, core.Failure("conflict", "extension path owner changed")
	}
	return Identity{uint64(s.Dev), s.Ino}, nil
}
func busy(e error) bool { return errors.Is(e, unix.EWOULDBLOCK) || errors.Is(e, unix.EAGAIN) }
