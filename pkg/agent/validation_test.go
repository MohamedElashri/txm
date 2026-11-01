package agent

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestCurrentCodexCompatibilityDoesNotInventInstalledBuildValidation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "codex"), []byte("#!/bin/sh\necho 'codex-cli 0.166.0'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "missing-state"))
	p := DiscoverProvider(context.Background(), "codex", true)
	if !p.Supported || p.Compatible == nil || !*p.Compatible || p.Verified || p.Validation != "fixture" || p.ValidationReference == nil || p.Enabled != nil || !p.TrustRequired {
		t.Fatalf("version output fabricated runtime/build/trust proof: %+v", p)
	}
	if _, err := os.Stat(filepath.Join(dir, "missing-state")); !os.IsNotExist(err) {
		t.Fatal("probe initialized txm state")
	}
	if !validProvider("codex", "codex-cli 0.165.0") || !validProvider("codex", liveCodexVersion) || validProvider("codex", "codex-cli 0.167.0") {
		t.Fatal("exact allowlist or old compatibility changed")
	}
}

func TestCurrentClaudeCompatibilityDoesNotInventReferenceValidation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "claude"), []byte("#!/bin/sh\necho '2.1.309 (Claude Code)'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	p := DiscoverProvider(context.Background(), "claude", true)
	if !p.Supported || p.Compatible == nil || !*p.Compatible || p.Verified || p.Validation != "fixture" || p.ValidationReference == nil || p.Enabled != nil || !p.TrustRequired {
		t.Fatalf("version output fabricated runtime/build/trust proof: %+v", p)
	}
	if !validProvider("claude", "2.1.308 (Claude Code)") || !validProvider("claude", currentClaudeVersion) || validProvider("claude", "2.1.310 (Claude Code)") {
		t.Fatal("exact Claude allowlist or old compatibility changed")
	}
	if legacyProvider("claude", currentClaudeVersion) {
		t.Fatal("new Claude version admitted to strict v1")
	}
}

func TestClaudeReferenceScopeExcludesUnobservedSources(t *testing.T) {
	id := observationIdentity()
	id.Provider, id.Version = "claude", currentClaudeVersion
	at := time.Now().UTC()
	for _, source := range []string{"startup", "resume", "compact", "clear", "fork"} {
		o, err := ReduceObservation(nil, id, ProviderEvent{Kind: "SessionStart", SessionID: "thread", StartSource: source}, 1, at, at)
		if err != nil {
			t.Fatal(err)
		}
		want := "fixture"
		if liveReferencePlatform() && (source == "startup" || source == "resume") {
			want = "authorized_live_reference"
		}
		if o.Fresh(at).Validation != want {
			t.Fatalf("source %s certified beyond reference", source)
		}
	}
}

func TestReferenceHashAndCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "binary")
	data := []byte("reviewed executable bytes")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(data))
	if !matchesReference(context.Background(), path, hash) || matchesReference(context.Background(), path, liveCodexSHA256) {
		t.Fatal("hash mismatch accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if matchesReference(ctx, path, hash) {
		t.Fatal("cancelled reference verification continued")
	}
}

func TestLiveReferenceScopeDoesNotCertifyUntestedStartSources(t *testing.T) {
	id := observationIdentity()
	id.Version = liveCodexVersion
	at := time.Now().UTC()
	for _, source := range []string{"startup", "resume", "compact", "fork", "clear"} {
		observation, err := ReduceObservation(nil, id, ProviderEvent{Kind: "SessionStart", SessionID: "thread", StartSource: source}, 1, at, at)
		if err != nil {
			t.Fatal(err)
		}
		view := observation.Fresh(at)
		want := "fixture"
		if liveReferencePlatform() && (source == "startup" || source == "resume") {
			want = "authorized_live_reference"
		}
		if view.Validation != want {
			t.Fatalf("source %s: %s, want %s", source, view.Validation, want)
		}
	}
}
