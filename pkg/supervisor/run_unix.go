//go:build (linux || darwin) && cgo

package supervisor

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/localipc"
	"github.com/MohamedElashri/txm/pkg/nativeprocess"
	"github.com/MohamedElashri/txm/pkg/nativeprotocol"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

func Supported() bool { return true }

func Run(ctx context.Context, id string) error {
	if !runtimepath.ValidID(id) {
		return core.Failure("invalid_usage", "invalid managed instance ID")
	}
	state, e := State()
	if e != nil {
		return e
	}
	defer func() { _ = state.Close() }()
	lease, e := state.Lock("supervisor-" + id)
	if e != nil {
		return core.Failure("conflict", "managed instance already running or unsafe")
	}
	defer func() { _ = lease.Close() }()
	data, e := state.ReadPrivate(ManifestName(id), MaxManifest)
	if e != nil {
		return core.Failure("invalid_config", "managed manifest unavailable or unsafe")
	}
	if e = state.RemovePrivate(ManifestName(id)); e != nil {
		return e
	}
	var m Manifest
	if nativeprotocol.Decode(data, &m) != nil || m.InstanceID != id || m.Validate() != nil {
		return core.Failure("invalid_config", "invalid managed manifest")
	}
	// Immutable receipts also prevent deliberate reuse after the lease is gone.
	if _, e = state.ReadPrivate(m.Receipt, 4096); !os.IsNotExist(e) {
		return core.Failure("conflict", "managed receipt already exists or is unsafe")
	}
	runtime, e := Runtime()
	if e != nil {
		return e
	}
	defer func() { _ = runtime.Close() }()
	path, e := runtime.SocketPath(id)
	if e != nil {
		return e
	}
	listener, e := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if e != nil {
		return e
	}
	listener.SetUnlinkOnClose(false)
	defer func() { _ = listener.Close() }()
	if e = os.Chmod(path, 0600); e != nil {
		return e
	}
	socket, e := runtime.SocketInfo(id)
	if e != nil {
		return e
	}
	defer func() { _ = runtime.RemoveSocket(id, socket) }()
	receipt := Receipt{SchemaVersion: 1, ManagedID: m.ManagedID, InstanceID: id, Backend: m.Backend, Outcome: "start_failed"}
	child := exec.Command(m.Argv[0], m.Argv[1:]...)
	child.Dir = m.Cwd
	child.Env = ChildEnvironment(m.EnvNames)
	if m.Hooks != nil {
		// Explicit provider opt-in adds only namespace paths and opaque IDs.
		// Credential token values remain exclusively in private files.
		keys := map[string]bool{"TXM_AGENT_ID": true, "TXM_AGENT_INSTANCE_ID": true, "TXM_HOOK_GENERATION": true, "XDG_STATE_HOME": true, "XDG_RUNTIME_DIR": true, "TMPDIR": true}
		filtered := []string{}
		for _, entry := range child.Env {
			key, _, _ := strings.Cut(entry, "=")
			if !keys[key] {
				filtered = append(filtered, entry)
			}
		}
		child.Env = append(filtered, m.Hooks.Environment()...)
	}
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if term.IsTerminal(0) {
		child.SysProcAttr.Foreground = true
		child.SysProcAttr.Ctty = 0
	}
	e = child.Start()
	if e == nil {
		started := Now()
		receipt.StartedAt = &started
		var mu sync.Mutex
		live := true
		serveDone := make(chan struct{})
		go func() {
			defer close(serveDone)
			for {
				c, err := listener.Accept()
				if err != nil {
					return
				}
				_ = c.SetDeadline(time.Now().Add(time.Second))
				if runtimepath.VerifyPeer(c) == nil {
					mu.Lock()
					running := false
					if live {
						exited, err := nativeprocess.Exited(child.Process.Pid)
						running = err == nil && !exited
					}
					mu.Unlock()
					_ = localipc.Write(c, Status{1, m.ManagedID, id, running, Now()})
				}
				_ = c.Close()
			}
		}()
		done := make(chan error, 1)
		go func() { done <- nativeprocess.WaitExited(child.Process.Pid) }()
		select {
		case e = <-done:
		case <-ctx.Done():
			if _, e = nativeprocess.Exited(child.Process.Pid); e != nil {
				break
			}
			_ = unix.Kill(-child.Process.Pid, unix.SIGTERM)
			select {
			case e = <-done:
			case <-time.After(time.Second):
				if _, e = nativeprocess.Exited(child.Process.Pid); e == nil {
					_ = unix.Kill(-child.Process.Pid, unix.SIGKILL)
					e = <-done
				}
			}
		}
		// Reserve the PID until the last descendant signal, as in the native server.
		mu.Lock()
		live = false
		if e != nil {
			mu.Unlock()
			_ = listener.Close()
			<-serveDone
			return core.Failure("operation_failed", "managed child identity/observation lost; no group signal or receipt claimed")
		}
		_ = unix.Kill(-child.Process.Pid, unix.SIGKILL)
		waitErr := child.Wait()
		mu.Unlock()
		_ = listener.Close()
		<-serveDone
		if child.ProcessState == nil {
			return core.Failure("operation_failed", "managed child was not reaped; no receipt claimed")
		}
		status, ok := child.ProcessState.Sys().(syscall.WaitStatus)
		if !ok {
			return core.Failure("operation_failed", "managed child status unavailable")
		}
		if status.Signaled() {
			signal := status.Signal().String()
			receipt.Signal = &signal
			receipt.Outcome = "signaled"
		} else if status.Exited() {
			code := status.ExitStatus()
			receipt.ExitCode = &code
			receipt.Outcome = "exited"
		} else {
			return core.Failure("operation_failed", "managed child termination unobserved")
		}
		_ = waitErr // Nonzero child status is recorded, not a wrapper failure.
	}
	receipt.FinishedAt = Now()
	encoded, _ := json.Marshal(receipt)
	if e = state.WritePrivate(m.Receipt, encoded, false); e != nil {
		return core.Failure("operation_failed", "managed receipt publication failed")
	}
	// No coordinator dependency: durable receipt is authoritative on next query.
	return nil
}
