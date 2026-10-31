//go:build linux || darwin

package backend

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MohamedElashri/txm/pkg/core"
)

func executable(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if e := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0700); e != nil {
		t.Fatal(e)
	}
	return p
}
func TestAvailabilityAndExecutionUseSameResolvedPath(t *testing.T) {
	dir := t.TempDir()
	empty := t.TempDir()
	fallback := executable(t, dir, "fixturemux", "printf 'resolved'\n")
	t.Setenv("PATH", empty)
	b := newExternal("fixturemux", []string{fallback})
	if b.path != fallback {
		t.Fatal("fallback not resolved")
	}
	replacement := t.TempDir()
	executable(t, replacement, "fixturemux", "exit 99\n")
	t.Setenv("PATH", replacement)
	out, _, _, e := b.output(context.Background(), 100, false, "--version")
	if e != nil || out != "resolved" {
		t.Fatal("execution disagrees with availability", out, e)
	}
}
func TestExternalBoundsCancellationAndPrivateDiagnostics(t *testing.T) {
	dir := t.TempDir()
	path := executable(t, dir, "fixturemux", "case \"$1\" in\n flood) while :; do printf '0123456789'; done;;\n stderr) while :; do printf 'private-synthetic' >&2; done;;\n slow) exec /bin/sleep 120;;\n denied) printf 'permission denied: private-synthetic' >&2;exit 1;;\n esac\n")
	b := newExternal("fixturemux", []string{path})
	out, _, limited, e := b.output(context.Background(), 101, true, "flood")
	if e != nil || !limited || len(out) != 101 {
		t.Fatal("unbounded capture", len(out), limited, e)
	}
	_, diag, _, e := b.output(context.Background(), 100, false, "stderr")
	if e == nil || len(diag) > 16384 || strings.Contains(e.Error(), "private-synthetic") {
		t.Fatal("diagnostic leaked or unbounded", e)
	}
	_, _, _, e = b.output(context.Background(), 100, false, "denied")
	if e == nil || core.Normalize(e).Code != "denied" || strings.Contains(e.Error(), "private-synthetic") {
		t.Fatal("permission/diagnostic contract", e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, _, _, e = b.output(ctx, 100, false, "slow")
	if e == nil || core.Normalize(e).Code != "timeout" || time.Since(start) > time.Second {
		t.Fatal("query did not terminate and reap promptly", e)
	}
}
func TestNumericWindowNameAmbiguityAndContextLookup(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "mutations")
	executable(t, dir, "tmux", "case \"$1\" in\n -V) printf 'tmux 3.4\\n';;\n list-sessions) printf '$1|sample|0\\n';;\n list-windows) printf '@1|1|2\\n@2|2|other\\n';;\n *) printf mutation > '"+log+"';;\n esac\n")
	t.Setenv("PATH", dir)
	b := NewTmuxBackend()
	if e := b.KillWindow("sample", "2"); e == nil || core.Normalize(e).Code != "ambiguous_target" {
		t.Fatal("numeric name/index ambiguity accepted", e)
	}
	if _, e := os.Stat(log); !os.IsNotExist(e) {
		t.Fatal("ambiguous target caused a mutation")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := b.Lookup(ctx, "sample"); e == nil || core.Normalize(e).Code != "cancelled" {
		t.Fatal("lookup ignored cancellation", e)
	}
}

func TestImplicitCurrentDirectoryExecutableIsRefused(t *testing.T) {
	dir := t.TempDir()
	executable(t, dir, "fixturemux", "exit 0\n")
	t.Chdir(dir)
	t.Setenv("PATH", ".")
	if b := newExternal("fixturemux", nil); b.path != "" {
		t.Fatal("ignored exec.ErrDot and trusted a cwd executable")
	}
}
