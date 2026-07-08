//go:build windows

package backend

import (
	"net"
	"os/exec"

	"golang.org/x/term"
)

func setSysProcAttr(cmd *exec.Cmd) {
	// Not implemented for windows
}

func watchWindowSize(fd int, conn net.Conn) {
	w, h, err := term.GetSize(fd)
	if err == nil {
		resizePacket := []byte{0x02, byte(w >> 8), byte(w), byte(h >> 8), byte(h)}
		_, _ = conn.Write(resizePacket)
	}
}
