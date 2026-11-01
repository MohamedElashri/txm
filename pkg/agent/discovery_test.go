package agent

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDoctorDoesNotRunProvidersOrInitializeState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable shell fixture")
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "called")
	for _, name := range []string{"claude", "codex", "opencode"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n: > '"+marker+"'\necho unsupported-version\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "absent-state"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(dir, "absent-runtime"))
	report := Doctor(context.Background(), false)
	if report.Coordinator != "not_contacted" || len(report.Providers) != 4 {
		t.Fatal(report)
	}
	for _, p := range report.Providers[1:] {
		if !p.Available || p.Compatible != nil || p.Verified || p.Supported {
			t.Fatal("discovery invented compatibility", p)
		}
	}
	for _, path := range []string{marker, filepath.Join(dir, "absent-state"), filepath.Join(dir, "absent-runtime")} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatal("doctor mutated state", path, err)
		}
	}
	p := DiscoverProvider(context.Background(), "codex", true)
	if p.Compatible == nil || *p.Compatible || p.Verified || p.Supported {
		t.Fatal("unknown version accepted", p)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("explicit probe did not execute", err)
	}
}
