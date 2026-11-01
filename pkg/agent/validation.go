package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"runtime"
)

// This identifies the CLI used by the recorded four-turn lifecycle fixture.
// It is reference evidence, never attestation of an arbitrary running instance.
const liveCodexVersion = "codex-cli 0.166.0"
const liveCodexSHA256 = "b8a11fedf24b2dfb59a70c9e52b030f9a508dcd49cd2dd0d41956f4c785156b8"
const liveCodexScope = "Linux/amd64; interactive --no-daemon; trusted neutral hooks; SessionStart startup/resume only; fork rebinding unsupported"
const liveClaudeSHA256 = "4d704a8aecbe038affa3000f498ae17c1b4f753add8facb8764120affc99b658"
const liveClaudeScope = "Linux/amd64; interactive --restricted; default permission mode; neutral hooks; startup/resume and prompt_id correlation; model_not_found failure; user cancellation has no callback; fork rebinding unsupported"

type ProviderValidationReference struct {
	Version      string `json:"version"`
	BinarySHA256 string `json:"binary_sha256"`
	Scope        string `json:"scope"`
}

func liveReferencePlatform() bool { return runtime.GOOS == "linux" && runtime.GOARCH == "amd64" }

func liveCodexEvent(kind, source string) bool {
	switch kind {
	case "SessionStart":
		return source == "startup" || source == "resume"
	case "UserPromptSubmit", "PermissionRequest", "PostToolUse", "Stop", "SessionEnd", "Interrupt":
		return true
	}
	return false
}

func annotateLiveCodex(ctx context.Context, p *Provider, path string) {
	if p.Name != "codex" || p.Version != liveCodexVersion {
		return
	}
	p.ValidationReference = &ProviderValidationReference{liveCodexVersion, liveCodexSHA256, liveCodexScope}
	p.Reason = "schema-compatible; reference lifecycle validated only in the stated scope; installed build or platform is unverified; review hook trust manually"
	if !liveReferencePlatform() || !matchesReference(ctx, path, liveCodexSHA256) {
		return
	}
	p.Verified, p.Validation = true, "authorized_live_reference"
	p.Reason = "installed build matches the validated interactive Linux/amd64 reference; current hook trust/enablement remain unknown; automatic configuration is disabled"
	for _, name := range providerEventNames("codex") {
		reason := "Validated in the reference scope; no permission decisions or process-exit inference."
		if name == "SessionStart" {
			reason = "startup/resume validated; compact/clear/fork start sources remain fixture-only; fork rebinding is refused."
		}
		if name == "PermissionRequest" {
			reason = "Actual prompt/cancellation validated; permission_requested remains unconfirmed metadata and never grants permission."
		}
		p.Events[name] = ProviderCapability{"authorized_live_reference", reason}
	}
}

func liveClaudeEvent(kind, source string) bool {
	if kind == "SessionStart" {
		return source == "startup" || source == "resume"
	}
	for _, name := range providerEventNames("claude") {
		if name == kind {
			return true
		}
	}
	return false
}

func annotateLiveClaude(ctx context.Context, p *Provider, path string) {
	if p.Name != "claude" || p.Version != currentClaudeVersion {
		return
	}
	p.ValidationReference = &ProviderValidationReference{currentClaudeVersion, liveClaudeSHA256, liveClaudeScope}
	p.Reason = "schema-compatible; reference lifecycle validated only in the stated scope; installed build or platform is unverified; preserve inherited settings and permissions"
	if !liveReferencePlatform() || !matchesReference(ctx, path, liveClaudeSHA256) {
		return
	}
	p.Verified, p.Validation = true, "authorized_live_reference"
	p.Reason = "installed build matches the validated Linux/amd64 Claude reference; current hook enablement/trust remain unknown; explicit --settings requires manual reviewed composition"
	for _, name := range providerEventNames("claude") {
		reason := "Validated in the interactive restricted/default-permission reference; no decisions or process-exit inference."
		switch name {
		case "SessionStart":
			reason = "startup/resume validated; other start sources remain fixture-only; fork rebinding is refused."
		case "PermissionRequest":
			reason = "Actual prompt/cancellation validated; permission_requested is unconfirmed metadata; user cancellation emits no terminal hook."
		case "StopFailure":
			reason = "Actual model_not_found callback validated; other API error types remain unverified."
		}
		p.Events[name] = ProviderCapability{"authorized_live_reference", reason}
	}
}

// Version probing is already explicitly opted in. Hash only regular bounded
// executable files, under the same deadline; default PATH discovery does no IO.
func matchesReference(ctx context.Context, path, expected string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	const limit = 512 << 20
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > limit {
		return false
	}
	hash := sha256.New()
	var buffer [64 << 10]byte
	var total int64
	for {
		if ctx.Err() != nil {
			return false
		}
		n, err := file.Read(buffer[:])
		total += int64(n)
		if total > limit {
			return false
		}
		_, _ = hash.Write(buffer[:n])
		if err == io.EOF {
			break
		}
		if err != nil {
			return false
		}
	}
	return total == info.Size() && hex.EncodeToString(hash.Sum(nil)) == expected
}
