package dashboard

import (
	"context"
	"fmt"
	"sync"
	"time"
	"unicode"

	"github.com/MohamedElashri/txm/pkg/agent"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/gdamore/tcell/v2"
)

type boundedTTY struct{ tcell.Tty }

func (t boundedTTY) WindowSize() (tcell.WindowSize, error) {
	size, e := t.Tty.WindowSize()
	size.Width = max(1, min(size.Width, 512))
	size.Height = max(1, min(size.Height, 200))
	return size, e
}
func openScreen() (tcell.Screen, error) {
	tty, e := tcell.NewDevTty()
	if e != nil {
		return nil, e
	}
	screen, e := tcell.NewTerminfoScreenFromTty(boundedTTY{tty})
	if e != nil {
		_ = tty.Close()
	}
	return screen, e
}

type action struct {
	Session  core.Session
	ReadOnly bool
}
type backendResult struct {
	Name     string
	Sessions []core.Session
	Err      error
}
type agentResult struct {
	Views []agent.View
	Err   error
	At    time.Time
}
type previewJob struct {
	Ctx        context.Context
	Generation uint64
	Session    core.Session
	Key        string
}
type previewResult struct {
	Generation uint64
	Key        string
	Snapshot   core.Snapshot
	Err        error
}

// Run owns screen lifetime. Workers never draw, and attachment runs only after
// the screen, input reader and all asynchronous reads have stopped.
func Run(ctx context.Context, o Options, source Source, factory func() (tcell.Screen, error)) error {
	if o.Refresh < 500*time.Millisecond || o.Refresh > time.Minute || len(o.Backends) == 0 {
		return core.Failure("invalid_usage", "dashboard refresh must be 500ms..1m and a backend is required")
	}
	if factory == nil {
		factory = openScreen
	}
	m := NewModel(o)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		s, e := factory()
		if e != nil {
			return core.Failure("operation_failed", "could not open dashboard terminal")
		}
		next, e := interact(ctx, s, m, source)
		if e != nil {
			return e
		}
		if next == nil {
			return nil
		}
		e = func() error {
			restore := terminalGuard()
			defer restore()
			return source.Attach(ctx, next.Session, next.ReadOnly)
		}()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if e != nil {
			m.Message = "Attach failed: " + core.Normalize(e).Code + ": " + core.Normalize(e).Message
		} else {
			m.Message = "Detached; dashboard resumed"
		}
		// Old source snapshots cannot authorize actions on a newly resumed screen.
		for name, st := range m.Backends {
			st.Updated = time.Time{}
			m.Backends[name] = st
		}
		m.AgentUpdated = time.Time{}
	}
}
func interact(parent context.Context, s tcell.Screen, m *Model, source Source) (*action, error) {
	defer s.Fini()
	if e := s.Init(); e != nil {
		return nil, core.Failure("operation_failed", "could not initialize dashboard terminal")
	}
	s.SetStyle(normal)
	s.HideCursor()
	ctx, cancel := context.WithCancel(parent)
	var workers sync.WaitGroup
	eventQuit := make(chan struct{})
	events := make(chan tcell.Event, 16)
	workers.Add(1)
	go func() { defer workers.Done(); s.ChannelEvents(events, eventQuit) }()
	defer func() { cancel(); close(eventQuit); s.Fini(); workers.Wait() }()
	results := make(chan backendResult, len(m.Options.Backends))
	refresh := map[string]chan struct{}{}
	for _, name := range m.Options.Backends {
		requests := make(chan struct{}, 1)
		refresh[name] = requests
		workers.Add(1)
		go func(name string, requests <-chan struct{}) {
			defer workers.Done()
			for {
				bounded, done := context.WithTimeout(ctx, SourceTimeout)
				rows, e := source.Sessions(bounded, name)
				done()
				select {
				case results <- backendResult{name, rows, e}:
				case <-ctx.Done():
					return
				}
				timer := time.NewTimer(m.Options.Refresh)
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-requests:
					timer.Stop()
				case <-timer.C:
				}
			}
		}(name, requests)
	}
	agents := make(chan agentResult, 1)
	if m.Options.Agents {
		workers.Add(1)
		go func() {
			defer workers.Done()
			send := func(update agentResult) {
				select {
				case <-agents:
				default:
				}
				select {
				case agents <- update:
				case <-ctx.Done():
				}
			}
			for {
				e := source.WatchAgents(ctx, func(views []agent.View) { send(agentResult{Views: views, At: time.Now()}) })
				if ctx.Err() != nil {
					return
				}
				send(agentResult{Err: e})
				timer := time.NewTimer(2 * time.Second)
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
			}
		}()
	}
	jobs := make(chan previewJob, 1)
	previews := make(chan previewResult, 1)
	workers.Add(1)
	go func() {
		defer workers.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case job := <-jobs:
				if job.Ctx.Err() != nil {
					continue
				}
				bounded, done := context.WithTimeout(job.Ctx, SourceTimeout)
				snapshot, e := source.Preview(bounded, job.Session, core.CaptureOptions{Lines: 100, Bytes: 16384})
				done()
				select {
				case previews <- previewResult{job.Generation, job.Key, snapshot, e}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	preview := "Preview off"
	previewKey := ""
	var generation uint64
	var previewCancel context.CancelFunc
	var lastPreview time.Time
	stopPreview := func() {
		if previewCancel != nil {
			previewCancel()
			previewCancel = nil
		}
		generation++
		previewKey = ""
	}
	defer func() {
		if previewCancel != nil {
			previewCancel()
		}
	}()
	updatePreview := func(now time.Time) {
		if !m.Options.Preview {
			stopPreview()
			preview = "Preview off"
			return
		}
		rows := m.Rows(now)
		row := m.Selection(rows)
		if row == nil {
			stopPreview()
			preview = "Preview unavailable: no selected session"
			return
		}
		if e := row.Action("output_snapshot"); e != nil {
			stopPreview()
			preview = "Preview unavailable: " + core.Normalize(e).Message
			return
		}
		if previewKey == row.Key && now.Sub(lastPreview) < 2*time.Second {
			return
		}
		stopPreview()
		previewKey = row.Key
		lastPreview = now
		jobCtx, jobCancel := context.WithCancel(ctx)
		previewCancel = jobCancel
		job := previewJob{jobCtx, generation, *row.Session, row.Key}
		select {
		case <-jobs:
		default:
		}
		jobs <- job
		preview = "Loading preview..."
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	updatePreview(time.Now())
	Render(s, m, preview, time.Now())
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case result := <-results:
			m.ApplyBackend(result.Name, result.Sessions, result.Err, time.Now())
		case result := <-agents:
			m.AgentError = result.Err
			if result.Err == nil {
				m.Agents = result.Views
				m.AgentUpdated = result.At
			}
		case result := <-previews:
			if result.Generation == generation && result.Key == previewKey && m.Options.Preview {
				if result.Err != nil {
					preview = "Preview unavailable: " + core.Normalize(result.Err).Code + ": " + core.Normalize(result.Err).Message
				} else {
					preview = fmt.Sprintf("%s at %s (truncated=%t)\n%s", result.Snapshot.Type, result.Snapshot.ObservedAt.UTC().Format(time.RFC3339), result.Snapshot.Truncated, core.Plain([]byte(result.Snapshot.Content)))
				}
			}
		case <-ticker.C:
		case event, ok := <-events:
			if !ok {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				return nil, core.Failure("operation_failed", "dashboard terminal disconnected")
			}
			switch ev := event.(type) {
			case *tcell.EventResize:
				s.Sync()
			case *tcell.EventError:
				return nil, core.Failure("operation_failed", "dashboard terminal read failed")
			case *tcell.EventKey:
				rows := m.Rows(time.Now())
				row := m.Selection(rows)
				if ev.Key() == tcell.KeyCtrlC {
					return nil, context.Canceled
				}
				if m.Searching {
					switch ev.Key() {
					case tcell.KeyEscape:
						m.Query = ""
						m.Searching = false
					case tcell.KeyEnter:
						m.Searching = false
					case tcell.KeyBackspace, tcell.KeyBackspace2:
						r := []rune(m.Query)
						if len(r) > 0 {
							m.Query = string(r[:len(r)-1])
						}
					case tcell.KeyCtrlU:
						m.Query = ""
					case tcell.KeyRune:
						if len([]rune(m.Query)) < 128 && !unicode.IsControl(ev.Rune()) {
							m.Query += string(ev.Rune())
						}
					}
					m.Selected = ""
					m.Index = 0
					break
				}
				switch ev.Key() {
				case tcell.KeyEscape:
					if m.Query != "" {
						m.Query = ""
						m.Selected = ""
						m.Index = 0
					} else {
						return nil, nil
					}
				case tcell.KeyUp, tcell.KeyCtrlP:
					m.Move(rows, -1)
				case tcell.KeyDown, tcell.KeyCtrlN:
					m.Move(rows, 1)
				case tcell.KeyPgUp:
					m.Move(rows, -10)
				case tcell.KeyPgDn:
					m.Move(rows, 10)
				case tcell.KeyHome:
					m.Move(rows, -len(rows))
				case tcell.KeyEnd:
					m.Move(rows, len(rows))
				case tcell.KeyTab:
					m.Filter = (m.Filter + 1) % len(Filters)
					m.Selected = ""
					m.Index = 0
				case tcell.KeyEnter:
					if row != nil {
						if e := row.Action("attach"); e != nil {
							m.Message = core.Normalize(e).Message
						} else {
							return &action{Session: *row.Session}, nil
						}
					}
				case tcell.KeyF1:
					m.Message = "Sessions and registrations are separate. p preview | o read-only | Tab state/type filter | b backend | q quit"
				case tcell.KeyRune:
					switch ev.Rune() {
					case 'q':
						return nil, nil
					case 'j':
						m.Move(rows, 1)
					case 'k':
						m.Move(rows, -1)
					case '/':
						m.Searching = true
					case 'b':
						m.BackendFilter = (m.BackendFilter + 1) % (len(m.Options.Backends) + 1)
						m.Selected = ""
						m.Index = 0
					case 'p':
						m.Options.Preview = !m.Options.Preview
					case 'r':
						for _, ch := range refresh {
							select {
							case ch <- struct{}{}:
							default:
							}
						}
						lastPreview = time.Time{}
						m.Message = "Refresh requested"
					case 'o':
						if row != nil {
							if e := row.Action("read_only_attach"); e != nil {
								m.Message = core.Normalize(e).Message
							} else {
								return &action{Session: *row.Session, ReadOnly: true}, nil
							}
						}
					case '?':
						m.Message = "/ search | Tab all/sessions/agents/needs_input/working/exited/unknown | b backend | Esc/q quit | Ctrl-C cancel"
					}
				}
			}
		}
		updatePreview(time.Now())
		Render(s, m, preview, time.Now())
	}
}
