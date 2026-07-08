//go:build !windows

package backend

import (
	"net"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"golang.org/x/term"
)

func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

func watchWindowSize(fd int, conn net.Conn) {
	sigwinch := make(chan os.Signal, 1)
	signal.Notify(sigwinch, syscall.SIGWINCH)
	go func() {
		for range sigwinch {
			w, h, err := term.GetSize(fd)
			if err == nil {
				// Send resize packet: [0x02, w(hi), w(lo), h(hi), h(lo)]
				resizePacket := []byte{0x02, byte(w >> 8), byte(w), byte(h >> 8), byte(h)}
				_, _ = conn.Write(resizePacket)
			}
		}
	}()
	// Trigger initial resize
	sigwinch <- syscall.SIGWINCH
}
