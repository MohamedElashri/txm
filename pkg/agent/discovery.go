package agent

import (
	"context"
	"io"
	"os/exec"
	"strings"
	"time"
)

type ProviderCapability struct {
	Level  string `json:"level"`
	Reason string `json:"reason"`
}

// InspectProvider retains v2.0's explicit version-probing behavior. Supported is
// the legacy schema-allowlist flag, not a claim of live lifecycle validation.
func InspectProvider(ctx context.Context, name string) Provider {
	return DiscoverProvider(ctx, name, true)
}

// DiscoverProvider never launches a provider unless the caller requests probe.
// Enablement/trust cannot be inferred from a version and remain nullable.
func DiscoverProvider(ctx context.Context, name string, probe bool) Provider {
	p := Provider{Name: name, Validation: "unverified", Reason: "provider executable unavailable", Events: map[string]ProviderCapability{}}
	if name == "generic" {
		compatible := true
		p.Version, p.Available, p.Compatible, p.Supported = "1", true, &compatible, true
		p.Verified, p.Validation, p.Reason = true, "isolated_runtime", "provider-neutral authenticated signals; hooks are opt-in per managed instance"
		p.Events["signal"] = ProviderCapability{"isolated_runtime", "authenticated metadata; process exit still requires a supervisor receipt"}
		return p
	}
	if name != "claude" && name != "codex" && name != "opencode" {
		p.Reason = "unknown provider"
		return p
	}
	p.TrustRequired = true
	path, err := exec.LookPath(name)
	if err != nil {
		return p
	}
	p.Available = true
	p.Reason = "version not probed; use agent doctor --probe for bounded executable discovery"
	if !probe {
		return p
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--version")
	cmd.WaitDelay = 250 * time.Millisecond
	var out versionWriter
	cmd.Stdout, cmd.Stderr = &out, io.Discard
	if cmd.Run() != nil {
		p.Reason = "provider version could not be verified"
		return p
	}
	p.Version = strings.TrimSpace(string(out.data))
	compatible := validProvider(name, p.Version)
	p.Compatible, p.Supported = &compatible, compatible
	p.Reason = "installed version is not in the adapter schema allowlist; lifecycle unverified"
	if compatible {
		p.Validation = "fixture"
		p.Reason = "schema fixtures only; authorized live lifecycle unverified; review instance hook setup and trust"
		for _, event := range providerEventNames(name) {
			p.Events[event] = ProviderCapability{"fixture", "payload parsing tested; installed lifecycle and event availability unverified"}
		}
	}
	if name == "opencode" {
		p.Reason = "v2 metadata projection has reference fixtures; plugin/session runtime unverified; automatic activation disabled"
	}
	annotateLiveCodex(ctx, &p, path)
	annotateLiveClaude(ctx, &p, path)
	return p
}

type DoctorReport struct {
	Scope       string     `json:"scope"`
	Coordinator string     `json:"coordinator"`
	Probed      bool       `json:"probed"`
	Providers   []Provider `json:"providers"`
	Guidance    []string   `json:"guidance"`
}

func Doctor(ctx context.Context, probe bool) DoctorReport {
	r := DoctorReport{Scope: "local_provider_discovery", Coordinator: "not_contacted", Probed: probe, Providers: []Provider{}, Guidance: []string{
		"Generic supervised jobs require no provider account; hooks are opt-in.",
		"Compatibility and fixture validation do not establish live provider support.",
		"Review instance-local hook settings and provider trust manually; doctor never changes them.",
		"Version probing executes installed binaries and may create their own caches; omit --probe for PATH-only discovery.",
	}}
	for _, name := range []string{"generic", "claude", "codex", "opencode"} {
		r.Providers = append(r.Providers, DiscoverProvider(ctx, name, probe))
	}
	return r
}
