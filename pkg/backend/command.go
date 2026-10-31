package backend

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/MohamedElashri/txm/pkg/core"
)

type external struct {
	name, path string
	ctx        context.Context
}

func newExternal(name string, paths []string, contexts ...context.Context) external {
	ctx := context.Background()
	if len(contexts) > 0 && contexts[0] != nil {
		ctx = contexts[0]
	}
	path, lookupErr := exec.LookPath(name)
	if lookupErr != nil {
		path = ""
	}
	if path == "" {
		for _, p := range paths {
			info, e := os.Stat(p)
			if e == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
				path = p
				break
			}
		}
	}
	return external{name, path, ctx}
}

type cappedWriter struct {
	buf       bytes.Buffer
	max       int
	truncated bool
	cancel    context.CancelFunc
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	n := len(p)
	available := w.max - w.buf.Len()
	if len(p) > available {
		p = p[:available]
		w.truncated = true
		w.cancel()
	}
	_, _ = w.buf.Write(p)
	return n, nil
}
func (b external) output(ctx context.Context, limit int, allowTruncate bool, args ...string) (string, string, bool, error) {
	if b.path == "" {
		return "", "", false, core.Scope(core.Failure("backend_unavailable", "backend executable unavailable"), b.name)
	}
	ctx, cancel := context.WithTimeout(ctx, core.RequestTimeout)
	defer cancel()
	childCtx, stop := context.WithCancel(ctx)
	defer stop()
	out := &cappedWriter{max: limit, cancel: stop}
	diagnostic := &cappedWriter{max: 16384, cancel: stop}
	cmd := exec.CommandContext(childCtx, b.path, args...)
	preserveEnvironment(cmd)
	cmd.WaitDelay = 250 * time.Millisecond
	cmd.Stdout = out
	cmd.Stderr = diagnostic
	err := cmd.Run()
	if ctx.Err() != nil {
		err = ctx.Err()
	} else if diagnostic.truncated {
		err = core.Failure("operation_failed", "backend diagnostic limit exceeded")
	} else if out.truncated {
		if allowTruncate {
			err = nil
		} else {
			err = core.Failure("protocol_error", "backend response limit exceeded")
		}
	}
	if err != nil {
		var known *core.OperationError
		if !errors.As(err, &known) {
			if errors.Is(err, os.ErrPermission) || strings.Contains(strings.ToLower(diagnostic.buf.String()), "permission denied") {
				err = core.Failure("denied", "backend access denied")
			} else if errors.Is(err, os.ErrNotExist) || errors.Is(err, exec.ErrNotFound) {
				err = core.Failure("backend_unavailable", "backend executable unavailable")
			} else if ctx.Err() == nil {
				wrapped := core.Failure("operation_failed", "backend command failed")
				wrapped.Cause = err
				err = wrapped
			}
		}
		err = core.Scope(err, b.name)
	}
	return out.buf.String(), diagnostic.buf.String(), out.truncated, err
}
func (b external) run(args ...string) error {
	_, _, _, err := b.output(b.ctx, core.MaxBytes, false, args...)
	return err
}
func (b external) interactive(args ...string) error {
	if b.path == "" {
		return core.Scope(core.Failure("backend_unavailable", "backend executable unavailable"), b.name)
	}
	cmd := exec.CommandContext(b.ctx, b.path, args...)
	preserveEnvironment(cmd)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if b.ctx.Err() != nil {
			return b.ctx.Err()
		}
		code := "operation_failed"
		if errors.Is(err, os.ErrPermission) {
			code = "denied"
		}
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, exec.ErrNotFound) {
			code = "backend_unavailable"
		}
		e := core.Scope(core.Failure(code, "interactive backend command failed"), b.name)
		e.Cause = err
		return e
	}
	return nil
}
func request(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, core.RequestTimeout)
}
