package backend

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/MohamedElashri/txm/pkg/core"
	p "github.com/MohamedElashri/txm/pkg/nativeprotocol"
	"github.com/MohamedElashri/txm/pkg/supervisor"
)

var capabilityKeys = []string{"session_lifecycle", "attach", "read_only_attach", "output_snapshot", "output_history", "input_injection", "exact_pane_targeting", "window_layout", "working_directory", "managed_launch"}

func capabilities(name string) core.Capabilities {
	caps := core.Capabilities{}
	now := time.Now().UTC()
	for _, key := range capabilityKeys {
		level, reason := "unsupported", "not implemented or not verified for this adapter"
		constraints := []string{}
		if name == "zellij" {
			constraints = append(constraints, "listing may include resurrectable sessions; process state is unknown")
		}
		switch name {
		case "native":
			switch key {
			case "session_lifecycle":
				level, reason = "degraded", "session rename is unsupported"
			case "attach", "read_only_attach", "output_snapshot", "input_injection", "working_directory":
				level, reason = "supported", ""
			case "managed_launch":
				if supervisor.Supported() {
					level, reason = "supported", ""
					constraints = []string{"opt-in structured supervisor; Unix and CGO", "terminal loss may leave exit unknown without receipt"}
				}
			}
			if key == "input_injection" {
				constraints = []string{"one terminal: window 0, pane 0", "accepted input is not command completion", "maximum 16 KiB including newline"}
			}
			if key == "output_snapshot" {
				constraints = []string{"visible screen only", "maximum 1 MiB including native wire overhead"}
			}
		case "tmux":
			if key != "managed_launch" {
				level, reason = "supported", ""
			}
			constraints = []string{"requires compatible tmux CLI; local runtime validated with 3.4"}
			if key == "input_injection" {
				constraints = append(constraints, "synchronize-panes must be off", "accepted bytes do not prove command completion")
			}
			if key == "managed_launch" && supervisor.Supported() {
				level, reason = "degraded", "requires verified tmux 3.4+ and structured supervisor"
			}
		case "screen", "zellij":
			if key == "session_lifecycle" || key == "attach" {
				level, reason = "degraded", "CLI adapter; limited portable semantics; session rename unsupported"
			}
		}
		var why *string
		if reason != "" {
			why = core.String(reason)
		}
		caps[key] = core.Capability{Level: level, Reason: why, Constraints: constraints, Source: name + " adapter", ObservedAt: now}
	}
	return caps
}
func observed(name, session string, id *string) core.Session {
	return core.Session{Locator: core.SessionLocator{Backend: name, Name: session}, BackendID: id, ProcessState: "unknown", ClientState: "unknown", ObservedAt: time.Now().UTC(), Source: name + " session listing", Capabilities: capabilities(name)}
}
func exact(sessions []core.Session, target string) (core.Session, error) {
	var found []core.Session
	for _, s := range sessions {
		if s.Locator.Name == target || (s.BackendID != nil && *s.BackendID == target) {
			found = append(found, s)
		}
	}
	if len(found) == 0 {
		return core.Session{}, core.Failure("not_found", "exact session not found")
	}
	if len(found) > 1 {
		return core.Session{}, core.Failure("ambiguous_target", "session name is ambiguous; use its backend ID")
	}
	return found[0], nil
}
func names(sessions []core.Session) []string {
	out := make([]string, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, s.Locator.Name)
	}
	return out
}
func sortSessions(s []core.Session) {
	sort.SliceStable(s, func(i, j int) bool {
		a, b := s[i], s[j]
		if a.Locator.Backend != b.Locator.Backend {
			return a.Locator.Backend < b.Locator.Backend
		}
		if a.Locator.Name != b.Locator.Name {
			return a.Locator.Name < b.Locator.Name
		}
		ai, bi := "", ""
		if a.BackendID != nil {
			ai = *a.BackendID
		}
		if b.BackendID != nil {
			bi = *b.BackendID
		}
		return ai < bi
	})
}
func unsupported(name, operation string) error {
	return core.Scope(core.Failure("unsupported", operation+" is unsupported by "+name), name)
}
func nativeError(err error) error {
	if err == nil {
		return nil
	}
	wire, ok := err.(*p.Error)
	if !ok {
		return core.Scope(err, "native")
	}
	code := wire.Code
	if code == "limit_exceeded" {
		code = "invalid_usage"
	}
	return core.Scope(core.Failure(code, wire.Message), "native")
}
func (b *NativeBackend) Sessions(ctx context.Context) ([]core.Session, error) {
	ctx, cancel := request(ctx)
	defer cancel()
	found, err := b.sessions(ctx)
	if err != nil {
		return nil, nativeError(err)
	}
	out := make([]core.Session, 0, len(found))
	for _, entry := range found {
		c, e := p.Dial(ctx, entry.path, "observer")
		if e != nil {
			return nil, nativeError(e)
		}
		if c.Identity.InstanceID != entry.identity.InstanceID {
			_ = c.Close()
			return nil, core.Failure("conflict", "native instance changed")
		}
		var status p.Status
		e = c.CallContext(ctx, p.Operation{Op: "status"}, &status)
		_ = c.Close()
		if e != nil {
			return nil, nativeError(e)
		}
		s := observed("native", entry.identity.Name, core.String(entry.identity.InstanceID))
		s.InstanceID = core.String(entry.identity.InstanceID)
		s.ProcessState = status.ProcessState
		s.AttachedClients = &status.AttachedClients
		s.ClientState = "detached"
		if status.AttachedClients > 0 {
			s.ClientState = "attached"
		}
		s.Source = "native v1 authenticated status"
		out = append(out, s)
	}
	return out, nil
}
func (b *NativeBackend) Capture(ctx context.Context, name string, o core.CaptureOptions) (core.Snapshot, error) {
	if e := o.Validate(); e != nil {
		return core.Snapshot{}, e
	}
	if o.History {
		return core.Snapshot{}, unsupported("native", "output history")
	}
	ctx, cancel := request(ctx)
	defer cancel()
	bound := NewNativeBackend(ctx)
	c, e := bound.client(name, "observer")
	if e != nil {
		return core.Snapshot{}, nativeError(e)
	}
	defer func() { _ = c.Close() }()
	return captureNativeClient(ctx, c, name, o)
}
func (b *NativeBackend) BackendCapabilities(context.Context) (core.BackendCapabilities, error) {
	c := core.BackendCapabilities{Backend: "native", Available: b.IsAvailable(), Version: core.String(fmt.Sprintf("protocol %d", p.Version)), Capabilities: capabilities("native")}
	if !c.Available {
		disableCapabilities(&c, "native runtime unavailable")
		return c, core.Scope(core.Failure("backend_unavailable", "native runtime unavailable"), "native")
	}
	return c, nil
}
func (b *NativeBackend) Windows(context.Context, string) ([]core.Window, error) {
	return nil, unsupported("native", "window management")
}
func (b *NativeBackend) Panes(context.Context, string, string) ([]core.Pane, error) {
	return nil, unsupported("native", "pane management")
}

func (b *TmuxBackend) Lookup(ctx context.Context, name string) (core.Session, error) {
	return b.session(ctx, name)
}
func (b *ScreenBackend) Lookup(ctx context.Context, name string) (core.Session, error) {
	s, e := b.Sessions(ctx)
	if e != nil {
		return core.Session{}, e
	}
	return exact(s, name)
}
func (b *ZellijBackend) Lookup(ctx context.Context, name string) (core.Session, error) {
	s, e := b.Sessions(ctx)
	if e != nil {
		return core.Session{}, e
	}
	return exact(s, name)
}
func (b *NativeBackend) Lookup(ctx context.Context, name string) (core.Session, error) {
	ctx, cancel := request(ctx)
	defer cancel()
	bound := NewNativeBackend(ctx)
	if _, e := bound.lookup(ctx, name); e != nil {
		return core.Session{}, nativeError(e)
	}
	ss, e := bound.Sessions(ctx)
	if e != nil {
		return core.Session{}, e
	}
	return exact(ss, name)
}

func disableCapabilities(c *core.BackendCapabilities, reason string) {
	for key, value := range c.Capabilities {
		value.Level = "unsupported"
		value.Reason = core.String(reason)
		c.Capabilities[key] = value
	}
}
