package runtimepath

import (
	"fmt"
	"golang.org/x/sys/unix"
	"os"
)

func checkPeer(fd int) error {
	c, err := unix.GetsockoptUcred(fd, unix.SOL_SOCKET, unix.SO_PEERCRED)
	if err != nil {
		return err
	}
	if c.Uid != uint32(os.Geteuid()) {
		return fmt.Errorf("native peer UID denied")
	}
	return nil
}
