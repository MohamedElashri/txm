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
func (s *coordinator) enableHook(r *Record, provider, version string) (Credential, error) {
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
	switch q.Op {
	case "hooks-enable":
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
		}
	case "hooks-info":
		if r.Hook == nil {
			return nil, core.Failure("unsupported", "no hook association configured")
		}
	case "signal":
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
		if input.SessionID != "" && !providerSession.MatchString(input.SessionID) {
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
		r.Hook = &next
		if e = saveRegistry(s.state, s.registry); e != nil {
			r.Hook = h
			return nil, e
		}
		v, e := s.observe(ctx, r)
		if e != nil {
			return nil, e
		}
		kind := "attention_changed"
		if state == "turn_complete" {
			kind = "turn_complete"
		}
		s.hub.emit(kind, "hook:"+h.Provider, &v)
		return []View{v}, nil
	}
	v, e := s.observe(ctx, r)
	return []View{v}, e
}
