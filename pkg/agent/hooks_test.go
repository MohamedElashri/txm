package agent

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestProviderTranslationIsSelectiveAndConservative(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		for _, test := range []struct{ event, extra, want string }{{"SessionStart", "", "unknown"}, {"UserPromptSubmit", "", "working"}, {"PermissionRequest", "", "needs_input"}, {"Stop", `,"stop_hook_active":false`, "idle"}, {"Stop", `,"stop_hook_active":true`, ""}, {"Stop", "", ""}, {"Notification", "", ""}} {
			input := []byte(`{"session_id":"abc-123","hook_event_name":"` + test.event + `","prompt":"PRIVATE_PROMPT","transcript_path":"/must/not/open"` + test.extra + `}`)
			state, session, e := ProviderInput(provider, input)
			if e != nil || state != test.want || session != "abc-123" {
				t.Fatal(provider, test, state, session, e)
			}
		}
	}
	for _, body := range []string{`{"session_id":"a","session_id":"b"}`, `{"session_id":"../x"}`, `[]`, `null`, strings.Repeat("x", 65537)} {
		if _, _, e := ProviderInput("claude", []byte(body)); e == nil {
			t.Fatal("accepted invalid input")
		}
	}
}
func TestHookAttentionExpiresWithoutChangingLifecycle(t *testing.T) {
	now := time.Now().UTC()
	old := now.Add(-HookFreshness - time.Second)
	h := &HookState{Provider: "generic", Version: "1", InstanceID: strings.Repeat("a", 32), Generation: strings.Repeat("b", 32), Sequence: 1, ObservedAt: &now, Attention: "working", TurnCompleteAt: &now}
	r := Record{Hook: h}
	v := hookView(View{Lifecycle: "running", Attention: "unknown"}, r)
	if v.Attention != "working" || v.Lifecycle != "running" {
		t.Fatal(v)
	}
	h.ObservedAt = &old
	v = hookView(View{Lifecycle: "running", Attention: "unknown"}, r)
	if v.Attention != "unknown" || v.Lifecycle != "running" {
		t.Fatal(v)
	}
	h.ObservedAt = &now
	v = hookView(View{Lifecycle: "exited", Attention: "unknown"}, r)
	if v.Attention != "unknown" || v.Lifecycle != "exited" {
		t.Fatal(v)
	}
	data, _ := json.Marshal(v)
	if strings.Contains(string(data), "token") || strings.Contains(string(data), "session_id") {
		t.Fatal("private fields in public view")
	}
}
func TestHubOverflowRequiresSnapshotAndDoesNotBlockOtherSubscribers(t *testing.T) {
	h := newHub("epoch")
	slow := h.add("")
	fast := h.add("target")
	v := View{ID: "target"}
	for range 17 {
		h.emit("agent_changed", "test", &v)
		<-fast.queue
	}
	if !slow.overflow || len(slow.queue) != 0 || len(slow.resync) != 1 {
		t.Fatal("overflow did not discard and require resync")
	}
	h.emit("agent_changed", "test", &v)
	if len(slow.queue) != 0 || len(fast.queue) != 1 {
		t.Fatal("slow subscriber affected fast subscriber")
	}
	slow.overflow = false
	<-slow.resync
	snapshot := h.event("resync_required", "coordinator", nil, EventMetadata{Reason: "overflow", Agents: []View{v}})
	h.emit("agent_changed", "test", &v)
	if (<-slow.queue).Sequence <= snapshot.Sequence {
		t.Fatal("resync cut not monotonic")
	}
}
func TestProviderConfigNeverEmbedsCredentials(t *testing.T) {
	c := Credential{AgentID: strings.Repeat("a", 32), Generation: strings.Repeat("b", 32), Token: strings.Repeat("c", 64), Provider: "claude", Namespace: Namespace{StateHome: "/tmp/a' b", TempDir: "/tmp"}}
	data, e := ProviderConfig("/tmp/txm 'quoted", c)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(data), c.Token) {
		t.Fatal("credential embedded in provider configuration")
	}
	var config map[string]any
	if json.Unmarshal(data, &config) != nil {
		t.Fatal("invalid settings")
	}
}
