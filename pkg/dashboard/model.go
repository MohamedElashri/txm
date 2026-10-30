// Package dashboard renders M1 observations and explicit M2 subscriptions.
package dashboard

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/MohamedElashri/txm/pkg/agent"
	"github.com/MohamedElashri/txm/pkg/core"
)

const MaxSessions = 2048
const SourceTimeout = 2 * time.Second
const AgentFreshness = 5 * time.Second

var Filters = []string{"all", "sessions", "agents", "needs_input", "working", "exited", "unknown"}

type Options struct {
	CurrentNativeID string
	HostID          string
	Backends        []string
	Agents, Preview bool
	Refresh         time.Duration
}
type BackendState struct {
	Sessions  []core.Session
	Updated   time.Time
	Err       error
	Truncated bool
}
type Model struct {
	Options               Options
	Backends              map[string]BackendState
	Agents                []agent.View
	AgentUpdated          time.Time
	AgentError            error
	Query                 string
	Searching             bool
	Filter, BackendFilter int
	Selected              string
	Index                 int
	Message               string
}
type Row struct {
	Key           string
	Session       *core.Session
	Agent         *agent.View
	Stale         bool
	Current       bool
	ContextUnsafe bool
}

func NewModel(o Options) *Model { return &Model{Options: o, Backends: map[string]BackendState{}} }
func (m *Model) ApplyBackend(name string, sessions []core.Session, err error, now time.Time) {
	state := m.Backends[name]
	state.Err = err
	if err == nil {
		state.Truncated = len(sessions) > MaxSessions
		sessions = append([]core.Session(nil), sessions...)
		sort.SliceStable(sessions, func(i, j int) bool {
			if sessions[i].Locator.Name != sessions[j].Locator.Name {
				return sessions[i].Locator.Name < sessions[j].Locator.Name
			}
			return sessionKey(sessions[i]) < sessionKey(sessions[j])
		})
		if len(sessions) > MaxSessions {
			sessions = sessions[:MaxSessions]
		}
		for i := range sessions {
			sessions[i].Locator.HostID = m.Options.HostID
		}
		state.Sessions = sessions
		state.Updated = now
	}
	m.Backends[name] = state
}
func sessionKey(s core.Session) string {
	id := s.Locator.Name
	if s.BackendID != nil {
		id = *s.BackendID
	}
	if s.InstanceID != nil {
		id = *s.InstanceID
	}
	return "session:" + s.Locator.HostID + ":" + s.Locator.Backend + ":" + id
}
func (r Row) Backend() string {
	if r.Session != nil {
		return r.Session.Locator.Backend
	}
	return r.Agent.Locator.Backend
}
func (r Row) Name() string {
	if r.Session != nil {
		return r.Session.Locator.Name
	}
	return r.Agent.Locator.Name
}
func (r Row) Lifecycle() string {
	if r.Stale {
		return "unknown"
	}
	if r.Session != nil {
		return r.Session.ProcessState
	}
	return r.Agent.Lifecycle
}
func (r Row) Attention() string {
	if r.Stale || r.Agent == nil {
		return "unknown"
	}
	return r.Agent.Attention
}
func (m *Model) Rows(now time.Time) []Row {
	rows := []Row{}
	total := 0
	for _, name := range m.Options.Backends {
		state := m.Backends[name]
		stale := state.Err != nil || state.Updated.IsZero() || now.Sub(state.Updated) > m.Options.Refresh+SourceTimeout
		for i := range state.Sessions {
			if total >= MaxSessions {
				break
			}
			total++
			v := &state.Sessions[i]
			rows = append(rows, Row{Key: sessionKey(*v), Session: v, Stale: stale, Current: name == "native" && v.InstanceID != nil && *v.InstanceID == m.Options.CurrentNativeID, ContextUnsafe: name == "native" && m.Options.CurrentNativeID == UnverifiedNativeContext})
		}
	}
	if m.Options.Agents {
		stale := m.AgentError != nil || m.AgentUpdated.IsZero() || now.Sub(m.AgentUpdated) > AgentFreshness
		for i := range m.Agents {
			v := &m.Agents[i]
			inScope := false
			for _, name := range m.Options.Backends {
				if name == v.Locator.Backend {
					inScope = true
					break
				}
			}
			if inScope && v.Locator.HostID == m.Options.HostID {
				proofStale := v.Lifecycle == "running" && (v.ObservedAt.IsZero() || now.Sub(v.ObservedAt) > AgentFreshness)
				rows = append(rows, Row{Key: "agent:" + v.ID, Agent: v, Stale: stale || proofStale})
			}
		}
	}
	result := []Row{}
	query := strings.ToLower(m.Query)
	for _, r := range rows {
		if m.BackendFilter > 0 && r.Backend() != m.Options.Backends[m.BackendFilter-1] {
			continue
		}
		filter := Filters[m.Filter]
		if filter == "sessions" && r.Session == nil || filter == "agents" && r.Agent == nil {
			continue
		}
		if filter != "all" && filter != "sessions" && filter != "agents" && r.Lifecycle() != filter && r.Attention() != filter {
			continue
		}
		if query != "" {
			text := r.Backend() + " " + r.Name() + " " + r.Lifecycle() + " " + r.Attention() + " " + r.Key
			if r.Agent != nil {
				text += " " + r.Agent.Source + " " + r.Agent.AttentionSource
			}
			if !strings.Contains(strings.ToLower(text), query) {
				continue
			}
		}
		result = append(result, r)
	}
	return result
}
func (m *Model) Selection(rows []Row) *Row {
	if len(rows) == 0 {
		m.Index = 0
		m.Selected = ""
		return nil
	}
	if m.Selected != "" {
		for i := range rows {
			if rows[i].Key == m.Selected {
				m.Index = i
				break
			}
		}
	}
	m.Index = max(0, min(m.Index, len(rows)-1))
	m.Selected = rows[m.Index].Key
	return &rows[m.Index]
}
func (m *Model) Move(rows []Row, delta int) {
	m.Selected = ""
	m.Index = max(0, min(m.Index+delta, len(rows)-1))
	m.Selection(rows)
}
func (r Row) Action(operation string) error {
	if r.ContextUnsafe {
		return core.Failure("unsupported", "native terminal context unavailable; attach/preview disabled")
	}
	if r.Current {
		return core.Failure("unsupported", "current native terminal: attach/preview disabled to prevent a feedback loop")
	}
	if r.Stale {
		return core.Failure("conflict", "source is stale; refresh before acting")
	}
	if r.Session == nil {
		return core.Failure("unsupported", "registration is not a live session binding; select a session row")
	}
	if r.Session.BackendID == nil {
		return core.Failure("unsupported", "session has no verified backend ID")
	}
	if r.Session.Locator.Backend != "native" && r.Session.Locator.Backend != "tmux" {
		return core.Failure("unsupported", "dashboard actions are verified only for native/tmux")
	}
	cap, ok := r.Session.Capabilities[operation]
	if !ok || cap.Level != "supported" {
		reason := "capability unsupported or degraded"
		if cap.Reason != nil {
			reason = *cap.Reason
		}
		return core.Failure("unsupported", reason)
	}
	return nil
}
func age(now, timeValue time.Time) string {
	if timeValue.IsZero() {
		return "unknown"
	}
	d := now.Sub(timeValue)
	if d < 0 {
		return "clock skew"
	}
	if d < time.Second {
		return "<1s"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}
