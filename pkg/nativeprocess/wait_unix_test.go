//go:build (linux || darwin) && cgo

package nativeprocess

import (
	"os/exec"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

func TestExitObservationDoesNotReap(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "exit 0")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Wait() }()
	if err := WaitExited(cmd.Process.Pid); err != nil {
		t.Fatal(err)
	}
	if exited, err := Exited(cmd.Process.Pid); err != nil || !exited {
		t.Fatal("non-reaping kernel exit query", exited, err)
	}
	if err := syscall.Kill(cmd.Process.Pid, 0); err != nil {
		t.Fatal("PID released before group teardown", err)
	}
	if err := SignalGroup(cmd.Process.Pid, unix.SIGKILL); err != nil {
		t.Fatal("owned zombie-only group reported permission failure", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
}
