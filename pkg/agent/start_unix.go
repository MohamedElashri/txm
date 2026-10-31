//go:build linux || darwin

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"golang.org/x/sys/unix"
)

func lockBusy(e error) bool         { return errors.Is(e, unix.EWOULDBLOCK) || errors.Is(e, unix.EAGAIN) }
func staleCoordinator(e error) bool { return errors.Is(e, unix.ECONNREFUSED) }
func startCoordinator(ctx context.Context) error {
	epoch, e := runtimepath.NewID()
	if e != nil {
		return e
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	reader, writer, e := os.Pipe()
	if e != nil {
		return e
	}
	defer func() { _ = reader.Close() }()
	cmd := exec.Command(exe, "internal", "coordinator", "--epoch", epoch, "--ready-fd", "3")
	cmd.ExtraFiles = []*os.File{writer}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if e = cmd.Start(); e != nil {
		_ = writer.Close()
		return e
	}
	_ = writer.Close()
	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(exited) }()
	ready := make(chan error, 1)
	go func() {
		var v struct {
			Epoch string `json:"epoch"`
			Error string `json:"error"`
		}
		err := json.NewDecoder(io.LimitReader(reader, 1024)).Decode(&v)
		if err == nil && (v.Error != "" || v.Epoch != epoch) {
			err = core.Failure("operation_failed", "coordinator failed to become ready")
		}
		ready <- err
	}()
	select {
	case e = <-ready:
	case <-ctx.Done():
		e = ctx.Err()
	}
	if e != nil {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-exited:
		case <-time.After(2 * time.Second):
			_ = cmd.Process.Kill()
			<-exited
		}
	}
	return e
}
