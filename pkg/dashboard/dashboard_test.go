package dashboard

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MohamedElashri/txm/pkg/agent"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/gdamore/tcell/v2"
)

func session(name, id string) core.Session {
	return core.Session{Locator: core.SessionLocator{Backend: "native", Name: name}, BackendID: core.String(id), InstanceID: core.String(id), ProcessState: "running", ClientState: "detached", ObservedAt: time.Now(), Source: "authenticated fixture", Capabilities: core.Capabilities{"attach": {Level: "supported"}, "read_only_attach": {Level: "supported"}, "output_snapshot": {Level: "supported"}}}
}
func options() Options {
	return Options{HostID: "host", Backends: []string{"native", "tmux"}, Refresh: 2 * time.Second}
}
func TestModelIdentityFreshnessFilteringAndBounds(t *testing.T) {
	m := NewModel(options())
	now := time.Now()
	m.ApplyBackend("native", []core.Session{session("alpha", "1"), session("beta", "2")}, nil, now)
	rows := m.Rows(now)
	m.Move(rows, 1)
	key := m.Selected
	renamed := session("renamed", "2")
	m.ApplyBackend("native", []core.Session{renamed, session("alpha", "1")}, nil, now)
	if row := m.Selection(m.Rows(now)); row == nil || row.Key != key || row.Name() != "renamed" {
		t.Fatal("selection lost on reorder")
	}
	m.Options.Agents = true
	m.AgentUpdated = now
	m.Agents = []agent.View{{ID: "registered", Locator: core.SessionLocator{HostID: "host", Backend: "native", Name: "renamed"}, Lifecycle: "running", Attention: "working", ObservedAt: now}}
	rows = m.Rows(now)
	if len(rows) != 3 {
		t.Fatal("registration was incorrectly joined by name")
	}
	m.Agents = append(m.Agents, agent.View{ID: "other-backend", Locator: core.SessionLocator{HostID: "host", Backend: "screen", Name: "outside-scope"}})
	if len(m.Rows(now)) != 3 {
		t.Fatal("strict backend scope leaked another registration")
	}
	m.Agents = m.Agents[:1]
	if rows[2].Action("attach") == nil {
		t.Fatal("registration authorized attach without binding")
	}
	m.Filter = 4
	if got := m.Rows(now); len(got) != 1 || got[0].Agent == nil {
		t.Fatal("attention filtering", got)
	}
	m.Filter = 2
	m.Agents[0].ObservedAt = now.Add(-6 * time.Second)
	if !m.Rows(now)[0].Stale {
		t.Fatal("heartbeat falsely renewed old per-agent proof")
	}
	m.Agents[0].ObservedAt = now
	m.Filter = 0
	m.Query = "renamed"
	if len(m.Rows(now)) != 2 {
		t.Fatal("search")
	}
	m.Query = ""
	m.ApplyBackend("native", nil, core.Failure("timeout", "timed out"), now)
	rows = m.Rows(now)
	if len(rows) != 3 || !rows[0].Stale || rows[0].Lifecycle() != "unknown" || rows[0].Action("attach") == nil {
		t.Fatal("failed refresh left actionable data")
	}
	if got := m.Rows(now.Add(6 * time.Second)); !got[2].Stale || got[2].Attention() != "unknown" {
		t.Fatal("agent attention did not become stale")
	}
	many := make([]core.Session, MaxSessions+100)
	for i := range many {
		many[i] = session(fmt.Sprint(i), fmt.Sprint(i))
	}
	m.ApplyBackend("native", many, nil, now)
	if len(m.Backends["native"].Sessions) != MaxSessions || !m.Backends["native"].Truncated {
		t.Fatal("row bounds")
	}
}
func text(s tcell.SimulationScreen) string {
	cells, w, h := s.GetContents()
	var result strings.Builder
	for i, c := range cells {
		if i > 0 && w > 0 && i%w == 0 {
			result.WriteByte('\n')
		}
		result.WriteString(string(c.Runes))
	}
	_ = h
	return result.String()
}
func TestRenderSanitizesMetadataAndPreviewAndSmallSizes(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if e := s.Init(); e != nil {
		t.Fatal(e)
	}
	defer s.Fini()
	s.SetSize(140, 42)
	m := NewModel(options())
	m.Options.Preview = true
	m.ApplyBackend("native", []core.Session{session("safe\x1b]52;c;PRIVATE_ESCAPE\a界", "1")}, nil, time.Now())
	Render(s, m, "Preview unavailable: \x1b[31mfailed\x1b[0m", time.Now())
	got := text(s)
	if strings.Contains(got, "PRIVATE_ESCAPE") || strings.ContainsRune(got, '\x1b') || !strings.Contains(got, "safe") || !strings.Contains(got, "Preview unavailable") {
		t.Fatal(got)
	}
	for _, size := range [][2]int{{1, 1}, {20, 5}, {80, 24}, {200, 65}} {
		s.SetSize(size[0], size[1])
		Render(s, m, "", time.Now())
	}
}

type fakeSource struct {
	sessions func(context.Context, string) ([]core.Session, error)
	preview  func(context.Context, core.Session, core.CaptureOptions) (core.Snapshot, error)
	attach   func(context.Context, core.Session, bool) error
	agents   atomic.Int32
}

func (f *fakeSource) Sessions(ctx context.Context, name string) ([]core.Session, error) {
	return f.sessions(ctx, name)
}
func (f *fakeSource) WatchAgents(ctx context.Context, receive func([]agent.View)) error {
	f.agents.Add(1)
	defer f.agents.Add(-1)
	receive([]agent.View{})
	<-ctx.Done()
	return ctx.Err()
}
func (f *fakeSource) Preview(ctx context.Context, s core.Session, o core.CaptureOptions) (core.Snapshot, error) {
	if f.preview != nil {
		return f.preview(ctx, s, o)
	}
	return core.Snapshot{}, errors.New("no preview")
}
func (f *fakeSource) Attach(ctx context.Context, s core.Session, ro bool) error {
	if f.attach != nil {
		return f.attach(ctx, s, ro)
	}
	return nil
}

type screenProbe struct {
	tcell.SimulationScreen
	frames  chan string
	stopped atomic.Bool
}

func probe() *screenProbe {
	return &screenProbe{SimulationScreen: tcell.NewSimulationScreen("UTF-8"), frames: make(chan string, 1)}
}
func (s *screenProbe) Init() error { e := s.SimulationScreen.Init(); s.SetSize(140, 42); return e }
func (s *screenProbe) Show() {
	s.SimulationScreen.Show()
	value := text(s.SimulationScreen)
	select {
	case <-s.frames:
	default:
	}
	s.frames <- value
}
func (s *screenProbe) Fini() { s.stopped.Store(true); s.SimulationScreen.Fini() }
func awaitFrame(t *testing.T, s *screenProbe, want string) {
	t.Helper()
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		select {
		case value := <-s.frames:
			if strings.Contains(value, want) {
				return
			}
		case <-timer.C:
			t.Fatal("missing frame", want)
		}
	}
}
func TestSlowBackendDoesNotBlockUIOrAgentDisabledPath(t *testing.T) {
	s := probe()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var blocked atomic.Int32
	source := &fakeSource{sessions: func(ctx context.Context, name string) ([]core.Session, error) {
		if name == "tmux" {
			blocked.Add(1)
			defer blocked.Add(-1)
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return []core.Session{session("responsive", "1")}, nil
	}}
	result := make(chan error, 1)
	go func() { result <- Run(ctx, options(), source, func() (tcell.Screen, error) { return s, nil }) }()
	awaitFrame(t, s, "responsive")
	started := time.Now()
	s.InjectKey(tcell.KeyRune, 'q', 0)
	select {
	case e := <-result:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("quit blocked behind a backend")
	}
	if time.Since(started) > 500*time.Millisecond || blocked.Load() != 0 || source.agents.Load() != 0 || !s.stopped.Load() {
		t.Fatal("worker/screen not stopped")
	}
}
func TestLatePreviewCannotReplaceSelectedSession(t *testing.T) {
	s := probe()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	first := make(chan struct{}, 1)
	var active, maxActive atomic.Int32
	source := &fakeSource{sessions: func(context.Context, string) ([]core.Session, error) {
		return []core.Session{session("alpha", "1"), session("beta", "2")}, nil
	}, preview: func(ctx context.Context, row core.Session, o core.CaptureOptions) (core.Snapshot, error) {
		n := active.Add(1)
		defer active.Add(-1)
		for old := maxActive.Load(); n > old; old = maxActive.Load() {
			if maxActive.CompareAndSwap(old, n) {
				break
			}
		}
		value := "RIGHT_NEW"
		if row.Locator.Name == "alpha" {
			select {
			case first <- struct{}{}:
			default:
			}
			<-ctx.Done()
			value = "WRONG_OLD"
		}
		if o.Raw || o.History || o.Bytes != 16384 || o.Lines != 100 {
			t.Error("unbounded or raw preview")
		}
		return core.Snapshot{Type: "screen_snapshot", Content: value, ObservedAt: time.Now()}, nil
	}}
	o := options()
	o.Backends = []string{"native"}
	o.Preview = true
	done := make(chan error, 1)
	go func() { done <- Run(ctx, o, source, func() (tcell.Screen, error) { return s, nil }) }()
	awaitFrame(t, s, "alpha")
	select {
	case <-first:
	case <-ctx.Done():
		t.Fatal("preview not started")
	}
	s.InjectKey(tcell.KeyRune, 'j', 0)
	awaitFrame(t, s, "RIGHT_NEW")
	s.InjectKey(tcell.KeyRune, 'q', 0)
	if e := <-done; e != nil {
		t.Fatal(e)
	}
	if maxActive.Load() != 1 {
		t.Fatal("more than one preview in flight")
	}
}
func TestReadOnlyAttachStopsWorkersAndScreenThenResumesAfterFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	screens := make(chan *screenProbe, 5)
	var previous *screenProbe
	var calls atomic.Int32
	source := &fakeSource{sessions: func(_ context.Context, name string) ([]core.Session, error) {
		if name != "native" {
			return nil, nil
		}
		return []core.Session{session("attachable", "1")}, nil
	}}
	source.attach = func(_ context.Context, row core.Session, ro bool) error {
		if !previous.stopped.Load() || source.agents.Load() != 0 || !ro || row.Locator.Name != "attachable" {
			t.Error("attach occurred before cleanup or with incorrect target/role")
		}
		calls.Add(1)
		return core.Failure("conflict", "target changed")
	}
	o := options()
	o.Agents = true
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, o, source, func() (tcell.Screen, error) { s := probe(); previous = s; screens <- s; return s, nil })
	}()
	for range 3 {
		s := <-screens
		awaitFrame(t, s, "running")
		s.InjectKey(tcell.KeyRune, 'o', 0)
	}
	var s *screenProbe
	select {
	case s = <-screens:
	case <-ctx.Done():
		t.Fatal("dashboard did not resume")
	}
	awaitFrame(t, s, "Attach failed: conflict")
	s.InjectKey(tcell.KeyRune, 'q', 0)
	if e := <-done; e != nil {
		t.Fatal(e)
	}
	if calls.Load() != 3 {
		t.Fatal("attach cycles", calls.Load())
	}
}
