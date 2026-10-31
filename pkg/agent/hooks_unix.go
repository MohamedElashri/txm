//go:build linux || darwin

package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"github.com/MohamedElashri/txm/pkg/supervisor"
)

func (s *coordinator) find(id string) *Record {
	for i := range s.registry.Agents {
		if s.registry.Agents[i].ID == id {
			return &s.registry.Agents[i]
		}
	}
	return nil
}
func (s *coordinator) allocateHook(r *Record, provider, version string) (Credential, error) {
	var c Credential
	if !validProvider(provider, version) {
		return c, core.Failure("unsupported_version", "provider adapter/version is unsupported")
	}
	generation, e := runtimepath.NewID()
	if e != nil {
		return c, e
	}
	instance, e := runtimepath.NewID()
	if e != nil {
		return c, e
	}
	if r.Supervised {
		instance = *r.InstanceID
	}
	a, e := runtimepath.NewID()
	if e != nil {
		return c, e
	}
	b, e := runtimepath.NewID()
	if e != nil {
		return c, e
	}
	c = Credential{1, r.ID, instance, generation, a + b, provider, version, Namespace{filepath.Dir(filepath.Dir(filepath.Dir(s.state.Path))), os.Getenv("XDG_RUNTIME_DIR"), os.TempDir()}}
	return c, nil
}
func (s *coordinator) enableHook(r *Record, provider, version string) (Credential, error) {
	if !legacyProvider(provider, version) {
		return Credential{}, core.Failure("unsupported_version", "provider version requires a separate association")
	}
	c, e := s.allocateHook(r, provider, version)
	if e != nil {
		return c, e
	}
	generation, instance := c.Generation, c.InstanceID
	data, _ := json.Marshal(c)
	if e = s.state.WritePrivate(credentialName(generation), data, false); e != nil {
		return c, e
	}
	r.Hook = &HookState{Provider: provider, Version: version, InstanceID: instance, Generation: generation, Attention: "unknown"}
	return c, nil
}
func (s *coordinator) hookRequest(ctx context.Context, q Request) ([]View, error) {
	r := s.find(q.ID)
	if r == nil {
		return nil, core.Failure("not_found", "agent ID not registered")
	}
	external := false
	if q.Op != "hooks-enable" && r.Hook == nil {
		projected, err := projectExternalHook(s.state, *r)
		if err != nil {
			return nil, err
		}
		if projected.Hook != nil {
			r, external = &projected, true
		}
	}
	switch q.Op {
	case "hooks-enable":
		if validProvider(q.Provider, q.ProviderVersion) && !legacyProvider(q.Provider, q.ProviderVersion) {
			if _, e := s.enableExternalHook(r, q.Provider, q.ProviderVersion); e != nil {
				return nil, e
			}
			v, e := s.observe(ctx, r)
			return []View{v}, e
		}
		old := r.Hook
		c, e := s.enableHook(r, q.Provider, q.ProviderVersion)
		if e != nil {
			return nil, e
		}
		if e = saveRegistry(s.state, s.registry); e != nil {
			r.Hook = old
			_ = s.state.RemovePrivate(credentialName(c.Generation))
			return nil, e
		}
		if old != nil {
			_ = s.state.RemovePrivate(credentialName(old.Generation))
			_ = s.state.RemovePrivate("p-" + old.Generation + ".json")
			_ = s.state.RemovePrivate(observationName(old.Generation))
		}
		discardExternalHook(s.state, *r)
	case "hooks-info":
		if r.Hook == nil {
			return nil, core.Failure("unsupported", "no hook association configured")
		}
	case "signal", "provider-event":
		if r.Hook == nil {
			return nil, core.Failure("unsupported", "no hook association configured")
		}
		input := q.Signal
		h := r.Hook
		c, e := ReadCredential(s.state, r.ID, h.Generation)
		if e != nil {
			return nil, e
		}
		if input.InstanceID != h.InstanceID || input.Generation != h.Generation || !validToken(input.Token, c.Token) {
			return nil, core.Failure("denied", "hook association authentication failed")
		}
		if input.Sequence == 0 || input.Sequence <= h.Sequence || (h.ObservedAt != nil && input.Time.Before(*h.ObservedAt)) {
			return nil, core.Failure("conflict", "stale or replayed hook sequence")
		}
		now := supervisor.Now()
		if input.Time.IsZero() || input.Time.Before(now.Add(-HookFreshness)) || input.Time.After(now.Add(5*time.Second)) {
			return nil, core.Failure("invalid_usage", "hook timestamp outside freshness window")
		}
		state := strings.ReplaceAll(input.State, "-", "_")
		if !validAttention(state) && state != "turn_complete" {
			return nil, core.Failure("invalid_usage", "invalid hook state")
		}
		if input.SessionID != "" && !providerSession().MatchString(input.SessionID) {
			return nil, core.Failure("invalid_usage", "invalid provider session association")
		}
		if h.Provider != "generic" && input.SessionID == "" {
			return nil, core.Failure("invalid_usage", "provider session association required")
		}
		if h.SessionID != "" && input.SessionID != h.SessionID {
			return nil, core.Failure("denied", "hook belongs to another provider session")
		}
		if r.Supervised {
			if _, e = supervisor.ReadReceipt(s.state, *r.ManagedID, *r.InstanceID, r.Locator.Backend); e == nil {
				return nil, core.Failure("conflict", "managed process already exited")
			} else if !os.IsNotExist(e) {
				return nil, e
			}
		}
		previous, err := loadObservation(s.state, *r)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		event := ProviderEvent{Kind: "signal", SessionID: input.SessionID, State: state}
		if q.ProviderEvent != nil {
			event = *q.ProviderEvent
			if event.SessionID != input.SessionID {
				return nil, core.Failure("denied", "provider event session mismatch")
			}
		}
		canonical, err := ReduceObservation(previous, ObservationIdentity{ManagedID: r.ID, InstanceID: h.InstanceID, Generation: h.Generation, Provider: h.Provider, Version: h.Version}, event, input.Sequence, input.Time, now)
		if err != nil {
			return nil, err
		}
		next := *h
		next.SessionID = input.SessionID
		next.Sequence = input.Sequence
		next.ObservedAt = &input.Time
		if state == "turn_complete" {
			next.Attention = "idle"
			next.TurnCompleteAt = &input.Time
		} else {
			next.Attention = state
			if state == "working" {
				next.TurnCompleteAt = nil
			}
		}
		if q.ProviderEvent != nil {
			next.Attention = canonical.Attention
			if next.Attention == "permission_requested" {
				next.Attention = "unknown"
			}
			next.TurnCompleteAt = canonical.TurnCompleteAt
		}
		encoded, err := json.Marshal(canonical)
		if err != nil {
			return nil, err
		}
		if err = s.state.WritePrivate(observationName(h.Generation), encoded, true); err != nil {
			return nil, err
		}
		r.Hook = &next
		if external {
			e = saveExternalHook(s.state, *r)
		} else {
			e = saveRegistry(s.state, s.registry)
		}
		if e != nil {
			r.Hook = h
			return nil, e
		}
		v, e := s.observe(ctx, r)
		if e != nil {
			return nil, e
		}
		kind := "attention_changed"
		if state == "turn_complete" || (event.Kind == "Stop" && canonical.TurnCompleteAt != nil) {
			kind = "turn_complete"
		}
		s.hub.emit(kind, "hook:"+h.Provider, &v)
		return []View{v}, nil
	}
	v, e := s.observe(ctx, r)
	return []View{v}, e
}

func (s *coordinator) enableExternalHook(r *Record, provider, version string) (Credential, error) {
	if !r.Supervised || r.InstanceID == nil || r.ManagedID == nil {
		return Credential{}, core.Failure("unsupported", "new provider versions require an existing supervised instance")
	}
	oldExternal, e := readExternalHook(s.state, *r)
	if e != nil {
		return Credential{}, e
	}
	c, e := s.allocateHook(r, provider, version)
	if e != nil {
		return c, e
	}
	c.SchemaVersion = 2
	data, _ := json.Marshal(c)
	if e = s.state.WritePrivate(externalCredentialName(c.Generation), data, false); e != nil {
		return c, e
	}
	virtual := *r
	virtual.Hook = &HookState{Provider: provider, Version: version, InstanceID: c.InstanceID, Generation: c.Generation, Attention: "unknown"}
	old := r.Hook
	// Removing a legacy association requires an ordinary v1-compatible save.
	// Publish the new sidecar first; legacy readers ignore it entirely.
	if e = saveExternalHook(s.state, virtual); e != nil {
		_ = s.state.RemovePrivate(externalCredentialName(c.Generation))
		return c, e
	}
	if old != nil {
		r.Hook = nil
		if e = saveRegistry(s.state, s.registry); e != nil {
			r.Hook = old
			_ = s.state.RemovePrivate(externalCredentialName(c.Generation))
			return c, e
		}
	}
	for _, h := range []*HookState{old, oldExternal} {
		if h == nil {
			continue
		}
		_ = s.state.RemovePrivate(credentialName(h.Generation))
		_ = s.state.RemovePrivate(externalCredentialName(h.Generation))
		_ = s.state.RemovePrivate(observationName(h.Generation))
		_ = s.state.RemovePrivate("p-" + h.Generation + ".json")
	}
	return c, nil
}
