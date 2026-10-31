package agent

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func observationIdentity() ObservationIdentity {
	return ObservationIdentity{ManagedID: strings.Repeat("a", 32), InstanceID: strings.Repeat("b", 32), Generation: strings.Repeat("c", 32), Provider: "codex", Version: providerVersions["codex"]}
}

func TestObservationSeparatesTurnAttentionAndProcess(t *testing.T) {
	now := time.Now().UTC()
	id := observationIdentity()
	var state *Observation
	for i, event := range []ProviderEvent{
		{Kind: "SessionStart", SessionID: "thread", StartSource: "startup"},
		{Kind: "UserPromptSubmit", SessionID: "thread", TurnID: "turn-1"},
		{Kind: "PermissionRequest", SessionID: "thread", TurnID: "turn-1"},
		{Kind: "PostToolUse", SessionID: "thread", TurnID: "turn-1"},
		{Kind: "Stop", SessionID: "thread", TurnID: "turn-1"},
	} {
		at := now.Add(time.Duration(i) * time.Millisecond)
		next, err := ReduceObservation(state, id, event, uint64(i+1), at, at)
		if err != nil {
			t.Fatal(event, err)
		}
		state = &next
		if event.Kind == "PermissionRequest" && state.Attention != "permission_requested" {
			t.Fatal("invented confirmed user wait", state)
		}
		if event.Kind != "Stop" && state.TurnCompleteAt != nil {
			t.Fatal("premature turn completion")
		}
	}
	if state.TurnOutcome != "completed" || state.TurnCompleteAt == nil || state.LocalTurn != 1 {
		t.Fatal(state)
	}
	view := state.Fresh(now.Add(3 * time.Minute))
	if view.Attention != "unknown" || view.Freshness != "expired" {
		t.Fatal("expired event became fresh", view)
	}
	if state.Attention != "idle" {
		t.Fatal("read mutated persisted observation")
	}
	data, _ := json.Marshal(state)
	for _, secret := range []string{"token", "prompt", "transcript", "process_state", "exit_code"} {
		if strings.Contains(string(data), `"`+secret+`"`) {
			t.Fatal("observation leaked unrelated data", secret)
		}
	}
}

func TestObservationRejectsReplayCrossTurnAndCrossAssociation(t *testing.T) {
	now := time.Now().UTC()
	id := observationIdentity()
	start := ProviderEvent{Kind: "UserPromptSubmit", SessionID: "thread", TurnID: "turn-1"}
	state, err := ReduceObservation(nil, id, start, 1, now, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range []ProviderEvent{
		{Kind: "Stop", SessionID: "other", TurnID: "turn-1"},
		{Kind: "Stop", SessionID: "thread", TurnID: "turn-2"},
		{Kind: "SessionStart", SessionID: "fork", StartSource: "fork"},
		{Kind: "SessionEnd", SessionID: "thread", TurnID: "other"},
	} {
		if _, err := ReduceObservation(&state, id, event, 2, now, now); err == nil {
			t.Fatal("accepted wrong identity", event)
		}
	}
	if _, err := ReduceObservation(&state, id, start, 1, now, now); err == nil {
		t.Fatal("accepted replay")
	}
	wrong := id
	wrong.InstanceID = strings.Repeat("d", 32)
	if _, err := ReduceObservation(&state, wrong, start, 2, now, now); err == nil {
		t.Fatal("accepted replaced process")
	}
	if _, err := ReduceObservation(&state, id, start, 2, now.Add(-time.Second), now); err == nil {
		t.Fatal("accepted old event")
	}
	if _, err := ReduceObservation(&state, id, start, 2, now.Add(6*time.Second), now); err == nil {
		t.Fatal("accepted future event")
	}
	completed, err := ReduceObservation(&state, id, ProviderEvent{Kind: "Stop", SessionID: "thread", TurnID: "turn-1"}, 2, now, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ReduceObservation(&completed, id, ProviderEvent{Kind: "Stop", SessionID: "thread", TurnID: "turn-1"}, 3, now, now); err == nil {
		t.Fatal("duplicate completion refreshed state")
	}
	resumed, err := ReduceObservation(&completed, id, ProviderEvent{Kind: "SessionStart", SessionID: "thread", StartSource: "resume"}, 3, now, now)
	if err != nil || resumed.TurnCompleteAt != nil || resumed.Attention != "unknown" {
		t.Fatal("resume inherited old completion", resumed, err)
	}
}

func TestParseProviderEventDiscardsContentAndRejectsMalformedIdentity(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		event, err := ParseProviderEvent(provider, []byte(`{"hook_event_name":"Stop","session_id":"session","turn_id":"turn","stop_hook_active":false,"prompt":"PRIVATE","tool_input":{"secret":"PRIVATE"},"transcript_path":"/do/not/read"}`))
		if err != nil || event.Kind != "Stop" || event.TurnID != "turn" {
			t.Fatal(event, err)
		}
		data, _ := json.Marshal(event)
		if strings.Contains(string(data), "PRIVATE") || strings.Contains(string(data), "transcript") {
			t.Fatal("retained sensitive payload")
		}
	}
	for _, body := range []string{
		`{"session_id":"a","session_id":"b","hook_event_name":"Stop"}`,
		`{"session_id":"a","turn_id":42,"hook_event_name":"Stop"}`,
		`{"session_id":"a","turn_id":"../bad","hook_event_name":"Stop"}`,
		`{"session_id":"a","hook_event_name":"Stop","stop_hook_active":"false"}`,
		strings.Repeat("x", 65537),
	} {
		if _, err := ParseProviderEvent("codex", []byte(body)); err == nil {
			t.Fatal("accepted malformed payload")
		}
	}
}

func TestClaudeSubagentsAndResumeCannotCompleteTheMainTurn(t *testing.T) {
	event, err := ParseProviderEvent("claude", []byte(`{"hook_event_name":"Stop","session_id":"main","agent_id":"subagent-1","stop_hook_active":false}`))
	if err != nil || event.Kind != "" {
		t.Fatal("subagent contaminated main association", event, err)
	}
	now := time.Now().UTC()
	id := observationIdentity()
	id.Provider = "claude"
	id.Version = providerVersions["claude"]
	working, err := ReduceObservation(nil, id, ProviderEvent{Kind: "UserPromptSubmit", SessionID: "main"}, 1, now, now)
	if err != nil {
		t.Fatal(err)
	}
	resumed, err := ReduceObservation(&working, id, ProviderEvent{Kind: "SessionStart", SessionID: "main", StartSource: "resume"}, 2, now, now)
	if err != nil {
		t.Fatal(err)
	}
	stopped, err := ReduceObservation(&resumed, id, ProviderEvent{Kind: "Stop", SessionID: "main"}, 3, now, now)
	if err != nil || stopped.TurnCompleteAt != nil || stopped.TurnOutcome != "unknown" {
		t.Fatal("resume reused old active turn", stopped, err)
	}
}

func TestClaudePromptIdentityRejectsStaleCompletion(t *testing.T) {
	id := observationIdentity()
	id.Provider, id.Version = "claude", currentClaudeVersion
	now := time.Now().UTC()
	var previous *Observation
	for i, row := range []struct{ kind, prompt string }{{"UserPromptSubmit", "prompt-one"}, {"Stop", "prompt-one"}, {"UserPromptSubmit", "prompt-two"}} {
		body, _ := json.Marshal(map[string]any{"hook_event_name": row.kind, "session_id": "thread", "prompt_id": row.prompt, "stop_hook_active": false})
		event, err := ParseProviderEvent("claude", body)
		if err != nil || event.TurnID != row.prompt {
			t.Fatal(event, err)
		}
		observation, err := ReduceObservation(previous, id, event, uint64(i+1), now, now)
		if err != nil {
			t.Fatal(err)
		}
		previous = &observation
	}
	stale, err := ParseProviderEvent("claude", []byte(`{"hook_event_name":"Stop","session_id":"thread","prompt_id":"prompt-one","stop_hook_active":false}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ReduceObservation(previous, id, stale, 4, now, now); err == nil {
		t.Fatal("stale Claude prompt completed a newer one")
	}
	for _, body := range []string{
		`{"hook_event_name":"Stop","session_id":"thread","prompt_id":42,"stop_hook_active":false}`,
		`{"hook_event_name":"Stop","session_id":"thread","prompt_id":"../bad","stop_hook_active":false}`,
		`{"hook_event_name":"Stop","session_id":"thread","prompt_id":"one","turn_id":"two","stop_hook_active":false}`,
	} {
		if _, err = ParseProviderEvent("claude", []byte(body)); err == nil {
			t.Fatal("malformed or conflicting prompt identity accepted")
		}
	}
}

func TestClaudeSessionEndDoesNotBindSlashCommandPromptIdentity(t *testing.T) {
	id := observationIdentity()
	id.Provider, id.Version = "claude", currentClaudeVersion
	now := time.Now().UTC()
	working, err := ReduceObservation(nil, id, ProviderEvent{Kind: "UserPromptSubmit", SessionID: "thread", TurnID: "model-prompt"}, 1, now, now)
	if err != nil {
		t.Fatal(err)
	}
	end, err := ParseProviderEvent("claude", []byte(`{"hook_event_name":"SessionEnd","session_id":"thread","prompt_id":"exit-slash-command"}`))
	if err != nil || end.TurnID != "" {
		t.Fatal(end, err)
	}
	ended, err := ReduceObservation(&working, id, end, 2, now, now)
	if err != nil || ended.TurnActive || ended.TurnCompleteAt != nil || ended.TurnID != "model-prompt" {
		t.Fatal(ended, err)
	}
}

func TestCodexCompactionPreservesActiveTurnAndOldTurnsCannotFinishNewOnes(t *testing.T) {
	now := time.Now().UTC()
	id := observationIdentity()
	working, err := ReduceObservation(nil, id, ProviderEvent{Kind: "UserPromptSubmit", SessionID: "thread", TurnID: "turn-1"}, 1, now, now)
	if err != nil {
		t.Fatal(err)
	}
	compacted, err := ReduceObservation(&working, id, ProviderEvent{Kind: "SessionStart", SessionID: "thread", StartSource: "compact"}, 2, now, now)
	if err != nil || compacted.TurnID != "turn-1" || !compacted.TurnActive {
		t.Fatal("compaction lost active turn", compacted, err)
	}
	completed, err := ReduceObservation(&compacted, id, ProviderEvent{Kind: "Stop", SessionID: "thread", TurnID: "turn-1"}, 3, now, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ReduceObservation(&completed, id, ProviderEvent{Kind: "UserPromptSubmit", SessionID: "thread", TurnID: "turn-2"}, 4, now, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ReduceObservation(&second, id, ProviderEvent{Kind: "Stop", SessionID: "thread", TurnID: "turn-1"}, 5, now, now); err == nil {
		t.Fatal("old turn completed new one")
	}
	interrupted, err := ReduceObservation(&second, id, ProviderEvent{Kind: "Interrupt", SessionID: "thread", TurnID: "turn-2"}, 6, now, now)
	if err != nil || interrupted.TurnOutcome != "interrupted" || interrupted.TurnCompleteAt != nil || interrupted.TurnActive {
		t.Fatal(interrupted, err)
	}
	if _, err = ReduceObservation(&second, id, ProviderEvent{Kind: "SessionStart", SessionID: "thread", StartSource: "fork"}, 7, now, now); err == nil {
		t.Fatal("fork reused a bound identity")
	}
}
