package dashboard

import (
	"fmt"
	"testing"
	"time"

	"github.com/MohamedElashri/txm/pkg/agent"
	"github.com/MohamedElashri/txm/pkg/core"
)

func TestTenAgentTriageUsesAttentionClockAndStableIdentity(t *testing.T) {
	now := time.Now()
	m := NewModel(options())
	m.Options.Agents = true
	m.AgentUpdated = now
	m.TaskGroups = map[string]string{}
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("%032x", i+1)
		at := now
		v := agent.View{ID: id, Locator: core.SessionLocator{HostID: "host", Backend: "native", Name: fmt.Sprintf("worker-%d", i)}, Lifecycle: "running", ObservedAt: now, Attention: "working", AttentionSource: "hook:generic", AttentionObservedAt: &at, Hooks: &agent.HookInfo{Provider: "generic"}}
		if i == 2 || i == 7 {
			v.Attention = "needs_input"
		}
		if i == 4 {
			old := now.Add(-3 * time.Minute)
			v.Attention = "needs_input"
			v.AttentionObservedAt = &old
		}
		if i == 5 {
			v.Attention = "needs_input"
			v.Hooks = nil
		}
		m.Agents = append(m.Agents, v)
		m.TaskGroups[id] = fmt.Sprintf("repo-%d / task-%d", i%2, i)
	}
	rows := m.Rows(now)
	if len(rows) != 10 || !rows[0].Blocked() || !rows[1].Blocked() {
		t.Fatal("fresh authenticated blocked agents not first", rows)
	}
	for _, r := range rows[2:] {
		if r.Blocked() {
			t.Fatal("more than two blocked")
		}
		if r.Agent.Locator.Name == "worker-4" && r.Attention() != "unknown" {
			t.Fatal("heartbeat revived stale attention")
		}
	}
	m.Selection(rows)
	first := m.Selected
	m.MoveBlocked(rows, 1)
	second := m.Selected
	if first == second {
		t.Fatal("blocked navigation did not move")
	}
	m.MoveBlocked(rows, -1)
	if m.Selected != first {
		t.Fatal("reverse blocked navigation")
	}
	m.GroupByTask = false
	if m.Selection(m.Rows(now)).Key != first {
		t.Fatal("selection lost on grouping")
	}
	m.GroupByTask = true
	m.Collapsed[m.Rows(now)[0].Group] = true
	if len(m.Rows(now)) != 9 || len(m.GroupCounts(now)) != 10 {
		t.Fatal("collapse or group counts")
	}
	m.Collapsed = map[string]bool{}
	// A canonical provider permission request stays visible but is not proof
	// that a person is actually blocked, even if a stale legacy hint disagrees.
	v := m.Agents[2]
	m.PublicAgents = map[string]agent.PublicView{v.ID: {View: v, ProviderObservation: &agent.ObservationView{Observation: agent.Observation{Attention: "permission_requested", ObservedAt: now}}}}
	for _, r := range m.Rows(now) {
		if r.Agent.ID == v.ID && (r.Attention() != "permission_requested" || r.Blocked()) {
			t.Fatal("permission request promoted to blocked")
		}
	}
}
