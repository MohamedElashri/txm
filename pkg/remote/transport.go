package remote

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/extension"
	"github.com/MohamedElashri/txm/pkg/nativeprotocol"
	"golang.org/x/term"
	"os"
	"os/exec"
	"sync"
	"time"
)

func sshArgs(h Host, q Request, tty bool) ([]string, error) {
	command, e := Command(h, q)
	if e != nil {
		return nil, e
	}
	mode := "-T"
	if tty {
		mode = "-tt"
	}
	return []string{mode, "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=yes", "-o", "ConnectTimeout=5", "-o", "ConnectionAttempts=1", "-o", "ServerAliveInterval=5", "-o", "ServerAliveCountMax=1", "--", h.Alias, command}, nil
}
func remoteFailure(code string) error {
	switch code {
	case "invalid_usage", "invalid_target", "not_found", "unsupported", "unsupported_version", "conflict", "timeout", "backend_unavailable", "denied", "cancelled", "protocol_error", "invalid_config", "operation_failed":
		return core.Failure(code, "remote operation failed ("+code+")")
	default:
		return core.Failure("protocol_error", "invalid remote error code")
	}
}
func Query(ctx context.Context, h Host, q Request, timeout time.Duration) (Data, error) {
	d := Data{}
	if timeout < time.Second || timeout > 30*time.Second {
		return d, core.Failure("invalid_usage", "remote timeout must be 1..30 seconds")
	}
	if h.HostID != q.HostID {
		return d, core.Failure("conflict", "request host identity differs from configured pin")
	}
	if q.Op != "probe" && q.Op != "list" {
		return d, core.Failure("invalid_usage", "SSH query requires probe or list")
	}
	args, e := sshArgs(h, q, false)
	if e != nil {
		return d, e
	}
	exe, e := extension.Resolve("ssh")
	if e != nil {
		return d, e
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	raw, exit, runErr := extension.Output(ctx, exe, "", nil, args...)
	if ctx.Err() != nil {
		return d, ctx.Err()
	}
	if exit == 255 {
		return d, core.Failure("backend_unavailable", "SSH connection/authentication failed")
	}
	if len(raw) == 0 {
		if runErr != nil {
			return d, runErr
		}
		return d, core.Failure("protocol_error", "missing remote JSON response")
	}
	var env struct {
		Schema  int                    `json:"schema_version"`
		Data    json.RawMessage        `json:"data"`
		Errors  []*core.OperationError `json:"errors"`
		Partial *bool                  `json:"partial"`
	}
	if nativeprotocol.Decode(raw, &env) != nil || len(env.Data) == 0 || env.Errors == nil || env.Partial == nil || *env.Partial {
		return d, core.Failure("protocol_error", "invalid remote JSON envelope")
	}
	if env.Schema != 1 {
		return d, core.Failure("unsupported_version", "remote envelope schema mismatch")
	}
	if len(env.Errors) > 0 {
		if len(env.Errors) != 1 || env.Errors[0] == nil {
			return d, core.Failure("protocol_error", "invalid remote error envelope")
		}
		return d, remoteFailure(env.Errors[0].Code)
	}
	if runErr != nil || exit != 0 {
		return d, core.Failure("protocol_error", "remote returned success envelope with failing status")
	}
	if nativeprotocol.Decode(env.Data, &d) != nil {
		return d, core.Failure("protocol_error", "invalid remote response data")
	}
	return d, validateData(d, q)
}

type Result struct {
	Alias    string               `json:"alias"`
	HostID   string               `json:"host_id"`
	Sessions []core.Session       `json:"sessions"`
	Error    *core.OperationError `json:"error"`
}

func List(ctx context.Context, hosts []Host, aliases []string, backend string, timeout time.Duration) ([]Result, error) {
	if backend != "native" && backend != "tmux" {
		return nil, core.Failure("unsupported", "remote list supports native or tmux")
	}
	if timeout < time.Second || timeout > 30*time.Second {
		return nil, core.Failure("invalid_usage", "remote timeout must be 1..30 seconds")
	}
	selected := []Host{}
	if len(aliases) == 0 {
		selected = append(selected, hosts...)
	} else {
		seen := map[string]bool{}
		for _, alias := range aliases {
			if seen[alias] {
				return nil, core.Failure("invalid_usage", "duplicate SSH alias")
			}
			h, e := Select(hosts, alias)
			if e != nil {
				return nil, e
			}
			seen[alias] = true
			selected = append(selected, h)
		}
	}
	out := make([]Result, len(selected))
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for i, h := range selected {
		wg.Add(1)
		go func(i int, h Host) {
			defer wg.Done()
			out[i] = Result{Alias: h.Alias, HostID: h.HostID}
			if h.HostID == "" {
				out[i].Error = core.Failure("denied", "SSH host must be explicitly pinned before listing")
				return
			}
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				out[i].Error = core.Normalize(ctx.Err())
				return
			}
			d, e := Query(ctx, h, Request{Schema: 1, Protocol: 1, Op: "list", HostID: h.HostID, Backend: backend}, timeout)
			if e != nil {
				out[i].Error = core.Normalize(e)
				return
			}
			out[i].Sessions = d.Sessions
		}(i, h)
	}
	wg.Wait()
	return out, nil
}
func Attach(ctx context.Context, h Host, q Request) error {
	if q.Op != "attach" || h.HostID == "" || q.HostID != h.HostID {
		return core.Failure("conflict", "attach requires the explicitly configured host pin")
	}
	args, e := sshArgs(h, q, true)
	if e != nil {
		return e
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return core.Failure("invalid_usage", "remote attach requires a terminal")
	}
	exe, e := extension.Resolve("ssh")
	if e != nil {
		return e
	}
	restore := extension.TerminalGuard()
	defer restore()
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.WaitDelay = 250 * time.Millisecond
	e = cmd.Run()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if e == nil {
		return nil
	}
	var exit *exec.ExitError
	if errors.As(e, &exit) {
		switch exit.ExitCode() {
		case 1:
			return remoteFailure("operation_failed")
		case 2:
			return remoteFailure("invalid_usage")
		case 3:
			return remoteFailure("not_found")
		case 4:
			return remoteFailure("unsupported")
		case 5:
			return remoteFailure("conflict")
		case 6:
			return remoteFailure("timeout")
		case 8:
			return remoteFailure("denied")
		case 130:
			return remoteFailure("cancelled")
		}
	}
	return core.Failure("backend_unavailable", "SSH attachment failed")
}
