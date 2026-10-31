package agent

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/nativeprotocol"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"github.com/MohamedElashri/txm/pkg/supervisor"
)

const HookFreshness = 2 * time.Minute

type HookState struct {
	Provider       string     `json:"provider"`
	Version        string     `json:"version"`
	InstanceID     string     `json:"instance_id"`
	Generation     string     `json:"generation"`
	SessionID      string     `json:"session_id"`
	Sequence       uint64     `json:"sequence"`
	Attention      string     `json:"attention"`
	ObservedAt     *time.Time `json:"observed_at"`
	TurnCompleteAt *time.Time `json:"turn_complete_at"`
}
type HookInfo struct {
	Provider     string `json:"provider"`
	Version      string `json:"version"`
	InstanceID   string `json:"instance_id"`
	Generation   string `json:"generation"`
	Sequence     uint64 `json:"sequence"`
	SessionBound bool   `json:"session_bound"`
}
type Namespace struct {
	StateHome  string `json:"state_home"`
	RuntimeDir string `json:"runtime_dir"`
	TempDir    string `json:"temp_dir"`
}
type Credential struct {
	SchemaVersion int       `json:"schema_version"`
	AgentID       string    `json:"agent_id"`
	InstanceID    string    `json:"instance_id"`
	Generation    string    `json:"generation"`
	Token         string    `json:"token"`
	Provider      string    `json:"provider"`
	Version       string    `json:"version"`
	Namespace     Namespace `json:"namespace"`
}
type Signal struct {
	InstanceID string    `json:"instance_id"`
	Generation string    `json:"generation"`
	Token      string    `json:"token"`
	SessionID  string    `json:"session_id"`
	Sequence   uint64    `json:"sequence"`
	Time       time.Time `json:"time"`
	State      string    `json:"state"`
}
type Provider struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Supported bool   `json:"supported"`
	Reason    string `json:"reason"`
}

var providerVersions = map[string]string{"generic": "1", "claude": "2.1.308 (Claude Code)", "codex": "codex-cli 0.165.0"}
var providerSession = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.:-]{0,127}$`)
var tokenPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func validAttention(state string) bool {
	return state == "unknown" || state == "working" || state == "needs_input" || state == "idle"
}
func validProvider(name, version string) bool {
	expected, ok := providerVersions[name]
	return ok && version == expected
}
func (h HookState) validate(r Record) bool {
	if !validProvider(h.Provider, h.Version) || !runtimepath.ValidID(h.InstanceID) || !runtimepath.ValidID(h.Generation) || !validAttention(h.Attention) || (h.SessionID != "" && !providerSession.MatchString(h.SessionID)) {
		return false
	}
	if r.Supervised && (r.InstanceID == nil || h.InstanceID != *r.InstanceID) {
		return false
	}
	if h.ObservedAt != nil && h.ObservedAt.IsZero() {
		return false
	}
	if h.TurnCompleteAt != nil && (h.TurnCompleteAt.IsZero() || h.ObservedAt == nil || h.TurnCompleteAt.After(*h.ObservedAt)) {
		return false
	}
	if h.Sequence == 0 && (h.SessionID != "" || h.Attention != "unknown") {
		return false
	}
	return (h.ObservedAt != nil) == (h.Sequence > 0)
}
func credentialName(generation string) string { return "h-" + generation + ".json" }
func ReadCredential(state *runtimepath.Root, id, generation string) (Credential, error) {
	var c Credential
	if !runtimepath.ValidID(id) || !runtimepath.ValidID(generation) {
		return c, core.Failure("invalid_usage", "invalid hook association")
	}
	data, e := state.ReadPrivate(credentialName(generation), 4096)
	if e != nil {
		return c, core.Failure("invalid_config", "hook credential unavailable or unsafe")
	}
	if nativeprotocol.Decode(data, &c) != nil || c.SchemaVersion != 1 || c.AgentID != id || c.Generation != generation || !runtimepath.ValidID(c.InstanceID) || !tokenPattern.MatchString(c.Token) || !validProvider(c.Provider, c.Version) || !filepath.IsAbs(c.Namespace.StateHome) || !filepath.IsAbs(c.Namespace.TempDir) || (c.Namespace.RuntimeDir != "" && !filepath.IsAbs(c.Namespace.RuntimeDir)) {
		return c, core.Failure("invalid_config", "invalid hook credential")
	}
	return c, nil
}

type versionWriter struct{ data []byte }

func (b *versionWriter) Write(p []byte) (int, error) {
	if len(b.data)+len(p) > 4096 {
		return 0, io.ErrShortBuffer
	}
	b.data = append(b.data, p...)
	return len(p), nil
}
func InspectProvider(ctx context.Context, name string) Provider {
	p := Provider{Name: name, Reason: "provider executable unavailable"}
	if name == "generic" {
		return Provider{name, "1", true, "provider-neutral authenticated signals"}
	}
	if name != "claude" && name != "codex" && name != "opencode" {
		p.Reason = "unknown provider"
		return p
	}
	path, e := exec.LookPath(name)
	if e != nil {
		return p
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--version")
	cmd.WaitDelay = 250 * time.Millisecond
	var out versionWriter
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if cmd.Run() != nil {
		p.Reason = "provider version could not be verified"
		return p
	}
	p.Version = strings.TrimSpace(string(out.data))
	p.Supported = validProvider(name, p.Version)
	p.Reason = "installed version is not in the verified adapter allowlist"
	if p.Supported {
		p.Reason = "verified schema; real initialization coverage only; full conversations unverified"
		if name == "codex" {
			p.Reason = "verified schema only; manual trust review required; provider runtime unverified"
		}
	}
	if name == "opencode" {
		p.Reason = "installed v2 plugin/session API is not runtime-verified; v1 examples are not used"
	}
	return p
}
func hookView(v View, r Record) View {
	if r.Hook == nil {
		return v
	}
	h := r.Hook
	v.Hooks = &HookInfo{h.Provider, h.Version, h.InstanceID, h.Generation, h.Sequence, h.SessionID != ""}
	v.TurnCompleteAt = h.TurnCompleteAt
	if h.ObservedAt != nil {
		v.AttentionObservedAt = h.ObservedAt
		v.AttentionSource = "hook:" + h.Provider
		age := time.Since(*h.ObservedAt)
		if age <= HookFreshness && age >= -5*time.Second && v.Lifecycle != "exited" {
			v.Attention = h.Attention
			v.TurnCompleteAt = h.TurnCompleteAt
		} else {
			v.Attention = "unknown"
			v.AttentionSource = "hook expired or process exited"
		}
	}
	return v
}

// ProviderInput deliberately ignores messages, tool inputs and transcript paths.
func ProviderInput(provider string, data []byte) (string, string, error) {
	if len(data) > 64<<10 {
		return "", "", core.Failure("invalid_usage", "provider input exceeds limit")
	}
	var fields map[string]json.RawMessage
	if nativeprotocol.Decode(data, &fields) != nil {
		return "", "", core.Failure("invalid_usage", "invalid provider input")
	}
	text := func(key string) string { var v string; _ = json.Unmarshal(fields[key], &v); return v }
	session := text("session_id")
	if !providerSession.MatchString(session) {
		return "", "", core.Failure("invalid_usage", "invalid provider session association")
	}
	if provider != "claude" && provider != "codex" {
		return "", "", core.Failure("unsupported", "provider bridge unsupported")
	}
	switch text("hook_event_name") {
	case "SessionStart":
		return "unknown", session, nil
	case "UserPromptSubmit":
		return "working", session, nil
	case "PermissionRequest":
		return "needs_input", session, nil
	case "Stop":
		var active bool
		if json.Unmarshal(fields["stop_hook_active"], &active) != nil || active {
			return "", session, nil
		}
		return "idle", session, nil
	}
	return "", session, nil
}
func NextSignal(ctx context.Context, c Credential, state string, session string) (Signal, error) {
	r, e := supervisor.State()
	if e != nil {
		return Signal{}, e
	}
	defer func() { _ = r.Close() }()
	var lock *os.File
	for {
		lock, e = r.Lock("hook-" + c.Generation)
		if e == nil {
			break
		}
		if !lockBusy(e) {
			return Signal{}, e
		}
		select {
		case <-ctx.Done():
			return Signal{}, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
	defer func() { _ = lock.Close() }()
	name := "c-" + c.Generation + ".json"
	cursor := struct {
		Schema     int    `json:"schema_version"`
		Generation string `json:"generation"`
		Sequence   uint64 `json:"sequence"`
	}{1, c.Generation, 0}
	data, e := r.ReadPrivate(name, 256)
	if e == nil {
		if nativeprotocol.Decode(data, &cursor) != nil || cursor.Schema != 1 || cursor.Generation != c.Generation {
			return Signal{}, core.Failure("invalid_config", "invalid hook cursor")
		}
	} else if !os.IsNotExist(e) {
		return Signal{}, e
	}
	if cursor.Sequence == ^uint64(0) {
		return Signal{}, core.Failure("conflict", "hook sequence exhausted")
	}
	cursor.Sequence++
	data, _ = json.Marshal(cursor)
	if e = r.WritePrivate(name, data, true); e != nil {
		return Signal{}, e
	}
	return Signal{c.InstanceID, c.Generation, c.Token, session, cursor.Sequence, supervisor.Now(), state}, nil
}
func validToken(a, b string) bool {
	return tokenPattern.MatchString(a) && tokenPattern.MatchString(b) && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
