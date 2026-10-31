// Package extension contains bounded local operations shared by optional extensions.
package extension

import (
	"bytes"
	"context"
	"errors"
	"github.com/MohamedElashri/txm/pkg/core"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type bounded struct {
	buffer   bytes.Buffer
	max      int
	overflow bool
}

func (b *bounded) Write(p []byte) (int, error) {
	n := len(p)
	if n > b.max-b.buffer.Len() {
		b.overflow = true
		p = p[:max(0, b.max-b.buffer.Len())]
	}
	_, _ = b.buffer.Write(p)
	return n, nil
}

// Output drains bounded streams and never exposes subprocess diagnostics.
func Output(ctx context.Context, exe, dir string, env []string, args ...string) ([]byte, int, error) {
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.WaitDelay = 250 * time.Millisecond
	out := &bounded{max: core.MaxBytes}
	diag := &bounded{max: 16384}
	cmd.Stdout = out
	cmd.Stderr = diag
	e := cmd.Run()
	if ctx.Err() != nil {
		return nil, -1, ctx.Err()
	}
	if out.overflow || diag.overflow {
		return nil, -1, core.Failure("protocol_error", "extension subprocess output exceeded limit")
	}
	if e != nil {
		var exit *exec.ExitError
		if errors.As(e, &exit) {
			return out.buffer.Bytes(), exit.ExitCode(), core.Failure("operation_failed", "extension subprocess failed")
		}
		return nil, -1, core.Failure("backend_unavailable", "extension executable unavailable")
	}
	return out.buffer.Bytes(), 0, nil
}
func Resolve(name string) (string, error) {
	p, e := exec.LookPath(name)
	if e != nil {
		return "", core.Failure("backend_unavailable", "required extension executable unavailable")
	}
	return filepath.Abs(p)
}

// NoSymlinks rejects symlinks in every existing component, including ancestors.
func NoSymlinks(p string) error {
	if !filepath.IsAbs(p) || len(p) > 4096 || strings.ContainsAny(p, "\x00\r\n") {
		return core.Failure("invalid_target", "invalid extension path")
	}
	current := filepath.VolumeName(p) + string(filepath.Separator)
	for _, part := range strings.Split(strings.TrimPrefix(filepath.Clean(p), current), string(filepath.Separator)) {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		s, e := os.Lstat(current)
		if os.IsNotExist(e) {
			return nil
		}
		if e != nil {
			return e
		}
		if s.Mode()&os.ModeSymlink != 0 {
			return core.Failure("conflict", "symlink in extension path")
		}
	}
	return nil
}
