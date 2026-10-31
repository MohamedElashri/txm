package agent

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/nativeprotocol"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
)

// ProviderEvent contains only observation metadata. Provider payloads are never
// persisted: prompts, tool inputs, transcript paths and responses stop here.
type ProviderEvent struct {
	Kind        string `json:"kind"`
	SessionID   string `json:"session_id"`
	TurnID      string `json:"turn_id,omitempty"`
	StartSource string `json:"start_source,omitempty"`
	State       string `json:"state,omitempty"`
}

type ObservationIdentity struct {
	ManagedID  string `json:"managed_id"`
	InstanceID string `json:"instance_id"`
	Generation string `json:"generation"`
	Provider   string `json:"provider"`
	Version    string `json:"version"`
}

// Observation is a separate schema, never embedded in the strict v1 registry
// or coordinator replies. A sidecar is usable only alongside its exact legacy
// projection; a crash between the two publications is an unknown observation.
type Observation struct {
	SchemaVersion  int                 `json:"schema_version"`
	Identity       ObservationIdentity `json:"identity"`
	SessionID      string              `json:"session_id"`
	TurnID         string              `json:"turn_id,omitempty"`
	LocalTurn      uint64              `json:"local_turn"`
	TurnActive     bool                `json:"turn_active"`
	Sequence       uint64              `json:"sequence"`
	Event          string              `json:"event"`
	StartSource    string              `json:"start_source,omitempty"`
	Source         string              `json:"source"`
	Ordering       string              `json:"ordering"`
	Attention      string              `json:"attention"`
	TurnOutcome    string              `json:"turn_outcome"`
	ObservedAt     time.Time           `json:"observed_at"`
	ReceivedAt     time.Time           `json:"received_at"`
	TurnCompleteAt *time.Time          `json:"turn_complete_at"`
}

type ObservationView struct {
	Observation
	Freshness       string `json:"freshness"`
	Validation      string `json:"validation"`
	ValidationScope string `json:"validation_scope,omitempty"`
}

func (o Observation) Fresh(now time.Time) ObservationView {
	v := ObservationView{Observation: o, Freshness: "fresh", Validation: "fixture"}
	if o.Identity.Provider == "generic" {
		v.Validation = "isolated_runtime"
	}
	if liveReferencePlatform() && o.Identity.Provider == "codex" && o.Identity.Version == liveCodexVersion && liveCodexEvent(o.Event, o.StartSource) {
		v.Validation = "authorized_live_reference"
		v.ValidationScope = liveCodexScope
	}
	if liveReferencePlatform() && o.Identity.Provider == "claude" && o.Identity.Version == currentClaudeVersion && liveClaudeEvent(o.Event, o.StartSource) {
		v.Validation = "authorized_live_reference"
		v.ValidationScope = liveClaudeScope
	}
	age := now.Sub(o.ObservedAt)
	if age < -5*time.Second || age > HookFreshness {
		v.Freshness = "expired"
		v.Attention = "unknown"
	}
	return v
}

func (id ObservationIdentity) valid() bool {
	return runtimepath.ValidID(id.ManagedID) && runtimepath.ValidID(id.InstanceID) &&
		runtimepath.ValidID(id.Generation) && id.ManagedID != id.InstanceID && validProvider(id.Provider, id.Version)
}

func (o Observation) valid() bool {
	if o.SchemaVersion != 1 || !o.Identity.valid() || o.Sequence == 0 || o.ObservedAt.IsZero() || o.ReceivedAt.IsZero() || o.ObservedAt.Before(o.ReceivedAt.Add(-HookFreshness)) || o.ObservedAt.After(o.ReceivedAt.Add(5*time.Second)) || o.Ordering != "bridge_arrival" {
		return false
	}
	if !validAttention(o.Attention) && o.Attention != "permission_requested" {
		return false
	}
	switch o.TurnOutcome {
	case "unknown", "completed", "interrupted", "ended", "failed":
	default:
		return false
	}
	if o.TurnActive && o.TurnOutcome != "unknown" {
		return false
	}
	if (o.TurnCompleteAt != nil) != (o.TurnOutcome == "completed") || (o.TurnCompleteAt != nil && (o.TurnCompleteAt.IsZero() || o.TurnCompleteAt.After(o.ObservedAt))) {
		return false
	}
	e := ProviderEvent{Kind: o.Event, SessionID: o.SessionID, TurnID: o.TurnID}
	if o.Event == "SessionStart" {
		e.TurnID, e.StartSource = "", o.StartSource
		if o.StartSource != "compact" && (o.TurnActive || o.TurnID != "") {
			return false
		}
	} else if o.StartSource != "" {
		return false
	}
	if o.Event == "signal" {
		e.State = o.Attention
		if o.Source != "authenticated_manual_signal" {
			return false
		}
	} else if o.Source != "hook:"+o.Identity.Provider {
		return false
	}
	return e.valid(o.Identity.Provider)
}

func (e ProviderEvent) valid(provider string) bool {
	if (e.SessionID != "" && !providerSession().MatchString(e.SessionID)) || (e.TurnID != "" && !providerSession().MatchString(e.TurnID)) {
		return false
	}
	if provider != "generic" && e.SessionID == "" {
		return false
	}
	if e.Kind == "signal" {
		return (validAttention(e.State) || e.State == "turn_complete") && e.TurnID == "" && e.StartSource == ""
	}
	if provider == "generic" || e.State != "" {
		return false
	}
	switch e.Kind {
	case "SessionStart":
		switch e.StartSource {
		case "", "startup", "resume", "fork", "compact", "clear":
			return e.TurnID == ""
		}
	case "StopFailure", "PostToolUseFailure":
		return provider == "claude" && e.StartSource == ""
	case "Interrupt":
		return provider == "codex" && e.StartSource == ""
	case "UserPromptSubmit", "PermissionRequest", "PostToolUse", "Stop", "SessionEnd":
		return e.StartSource == ""
	}
	return false
}

// ReduceObservation uses bridge-arrival order, not invented provider causality.
// Unknown session rebindings and cross-turn terminal events fail closed.
func ReduceObservation(previous *Observation, id ObservationIdentity, event ProviderEvent, sequence uint64, observed, received time.Time) (Observation, error) {
	bad := func() (Observation, error) {
		return Observation{}, core.Failure("conflict", "provider observation identity, order or timestamp mismatch")
	}
	if !id.valid() || !event.valid(id.Provider) || sequence == 0 || received.IsZero() || observed.IsZero() || observed.Before(received.Add(-HookFreshness)) || observed.After(received.Add(5*time.Second)) {
		return bad()
	}
	next := Observation{SchemaVersion: 1, Identity: id, Attention: "unknown", TurnOutcome: "unknown"}
	if previous != nil {
		if !previous.valid() || previous.Identity != id || sequence <= previous.Sequence || observed.Before(previous.ObservedAt) || previous.SessionID != event.SessionID {
			return bad()
		}
		if previous.Event == "SessionEnd" && event.Kind != "SessionStart" && event.Kind != "signal" {
			return bad()
		}
		if event.Kind == "SessionStart" && event.StartSource == "fork" {
			return bad()
		}
		next = *previous
		sessionScoped := event.Kind == "SessionStart" || event.Kind == "UserPromptSubmit" || event.Kind == "signal" || (event.Kind == "SessionEnd" && event.TurnID == "")
		if !sessionScoped && previous.TurnID != event.TurnID {
			return bad()
		}
		if previous.TurnOutcome != "unknown" && event.Kind != "SessionStart" && event.Kind != "UserPromptSubmit" && event.Kind != "SessionEnd" && event.Kind != "signal" {
			return bad()
		}
		if event.Kind == "UserPromptSubmit" && event.TurnID != "" && previous.TurnID == event.TurnID {
			return bad()
		}
	}
	if previous == nil {
		next.TurnID = event.TurnID
	}
	next.SessionID, next.Sequence, next.Event = event.SessionID, sequence, event.Kind
	next.StartSource = event.StartSource
	next.Source, next.Ordering = "hook:"+id.Provider, "bridge_arrival"
	next.ObservedAt, next.ReceivedAt = observed.UTC(), received.UTC()
	switch event.Kind {
	case "SessionStart":
		if event.StartSource != "compact" || previous == nil || !previous.TurnActive {
			next.Attention, next.TurnOutcome, next.TurnID, next.TurnCompleteAt = "unknown", "unknown", "", nil
			next.TurnActive = false
		}
	case "UserPromptSubmit":
		if next.LocalTurn == ^uint64(0) {
			return bad()
		}
		next.LocalTurn++
		next.TurnID, next.Attention, next.TurnOutcome, next.TurnCompleteAt = event.TurnID, "working", "unknown", nil
		next.TurnActive = true
	case "PermissionRequest":
		// The event can also precede auto-denial; it does not prove an open UI.
		next.Attention = "permission_requested"
	case "PostToolUse", "PostToolUseFailure":
		next.Attention = "working"
	case "Stop":
		next.Attention = "idle"
		if next.TurnActive {
			next.TurnOutcome = "completed"
			t := observed.UTC()
			next.TurnCompleteAt = &t
		}
		next.TurnActive = false
	case "StopFailure":
		next.Attention, next.TurnCompleteAt = "unknown", nil
		if next.TurnActive {
			next.TurnOutcome = "failed"
		}
		next.TurnActive = false
	case "Interrupt":
		next.Attention, next.TurnOutcome, next.TurnCompleteAt = "unknown", "interrupted", nil
		next.TurnActive = false
	case "SessionEnd":
		next.Attention, next.TurnActive = "unknown", false
	case "signal":
		next.Source = "authenticated_manual_signal"
		next.Attention = event.State
		if event.State == "working" {
			next.TurnOutcome, next.TurnCompleteAt = "unknown", nil
			next.TurnActive = true
		}
		if event.State == "turn_complete" {
			next.Attention, next.TurnOutcome = "idle", "completed"
			next.TurnActive = false
			t := observed.UTC()
			next.TurnCompleteAt = &t
		}
	}
	return next, nil
}

func ParseProviderEvent(provider string, data []byte) (ProviderEvent, error) {
	bad := func() (ProviderEvent, error) {
		return ProviderEvent{}, core.Failure("invalid_usage", "invalid provider event metadata")
	}
	if provider != "claude" && provider != "codex" {
		return ProviderEvent{}, core.Failure("unsupported", "provider observer unavailable")
	}
	if len(data) > 64<<10 {
		return bad()
	}
	var fields map[string]json.RawMessage
	if nativeprotocol.Decode(data, &fields) != nil {
		return bad()
	}
	read := func(key string) (string, bool) {
		b, ok := fields[key]
		if !ok {
			return "", true
		}
		var s string
		err := json.Unmarshal(b, &s)
		return s, err == nil && !bytes.Equal(bytes.TrimSpace(b), []byte("null"))
	}
	kind, k := read("hook_event_name")
	session, s := read("session_id")
	turn, t := read("turn_id")
	if b, ok := fields["turn_id"]; ok && bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		turn, t = "", true
	}
	if !k || !s || !t || kind == "" {
		return bad()
	}
	if b, ok := fields["agent_id"]; ok {
		if !bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
			agent, valid := read("agent_id")
			if !valid {
				return bad()
			}
			if agent != "" {
				return ProviderEvent{}, nil
			}
		}
	}
	if !providerSession().MatchString(session) || (turn != "" && !providerSession().MatchString(turn)) {
		return bad()
	}
	if provider == "claude" && kind != "SessionStart" && kind != "SessionEnd" {
		prompt, ok := read("prompt_id")
		if !ok || (prompt != "" && (!providerSession().MatchString(prompt) || (turn != "" && turn != prompt))) {
			return bad()
		}
		if prompt != "" {
			// Claude's documented prompt_id correlates the lifecycle of one
			// user prompt. Older payloads without it retain the local counter.
			turn = prompt
		}
	}
	event := ProviderEvent{Kind: kind, SessionID: session, TurnID: turn}
	if kind == "SessionStart" {
		var ok bool
		event.StartSource, ok = read("source")
		if !ok {
			return bad()
		}
	}
	if kind == "Stop" {
		var recursive bool
		b, present := fields["stop_hook_active"]
		if !present {
			return ProviderEvent{}, nil
		}
		if json.Unmarshal(b, &recursive) != nil || bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
			return bad()
		}
		if recursive {
			return ProviderEvent{}, nil
		}
	}
	if !event.valid(provider) {
		// Ignore unknown event types without retaining their bodies.
		switch kind {
		case "SessionStart", "UserPromptSubmit", "PermissionRequest", "PostToolUse", "PostToolUseFailure", "Stop", "StopFailure", "Interrupt", "SessionEnd":
			return bad()
		}
		return ProviderEvent{}, nil
	}
	return event, nil
}

func observationName(generation string) string { return "o-" + generation + ".json" }

func loadObservation(root *runtimepath.Root, r Record) (*Observation, error) {
	if r.Hook == nil {
		return nil, nil
	}
	b, err := root.ReadPrivate(observationName(r.Hook.Generation), 8192)
	if err != nil {
		return nil, err
	}
	var o Observation
	if nativeprotocol.Decode(b, &o) != nil || !o.valid() || o.Identity.ManagedID != r.ID || o.Identity.InstanceID != r.Hook.InstanceID || o.Identity.Generation != r.Hook.Generation || o.Identity.Provider != r.Hook.Provider || o.Identity.Version != r.Hook.Version || o.Sequence != r.Hook.Sequence || o.SessionID != r.Hook.SessionID || r.Hook.ObservedAt == nil || !o.ObservedAt.Equal(*r.Hook.ObservedAt) {
		return nil, core.Failure("conflict", "provider observation requires reconciliation")
	}
	attention := o.Attention
	if attention == "permission_requested" {
		attention = "unknown"
	}
	if attention != r.Hook.Attention || (o.TurnCompleteAt == nil) != (r.Hook.TurnCompleteAt == nil) || (o.TurnCompleteAt != nil && !o.TurnCompleteAt.Equal(*r.Hook.TurnCompleteAt)) {
		return nil, core.Failure("conflict", "provider observation projection mismatch")
	}
	return &o, nil
}
