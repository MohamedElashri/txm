package backend

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/supervisor"
)

var commonTmuxPaths = []string{"/usr/bin/tmux", "/usr/local/bin/tmux", "/opt/homebrew/bin/tmux", "/home/linuxbrew/.linuxbrew/bin/tmux"}

type TmuxBackend struct {
	external
	version atomic.Pointer[string]
}

func NewTmuxBackend(ctx ...context.Context) *TmuxBackend {
	return &TmuxBackend{external: newExternal("tmux", commonTmuxPaths, ctx...)}
}
func (b *TmuxBackend) Name() string      { return "tmux" }
func (b *TmuxBackend) IsAvailable() bool { return b.path != "" }

var tmuxSessionID = regexp.MustCompile(`^\$[0-9]+$`)
var tmuxWindowID = regexp.MustCompile(`^@[0-9]+$`)
var tmuxPaneID = regexp.MustCompile(`^%[0-9]+$`)

func tmuxEmpty(err error, diagnostic string) bool {
	// These English exit-1 diagnostics are exercised by the real/fake fixtures.
	// Permission, malformed responses and arbitrary command failures remain errors.
	var exitCode int
	if e := core.Normalize(err); e.Cause != nil {
		type exiter interface{ ExitCode() int }
		if e, ok := e.Cause.(exiter); ok {
			exitCode = e.ExitCode()
		}
	}
	d := strings.TrimSpace(diagnostic)
	return exitCode == 1 && (strings.HasPrefix(d, "no server running on ") || (strings.HasPrefix(d, "error connecting to ") && strings.HasSuffix(d, "(No such file or directory)")))
}
func (b *TmuxBackend) Sessions(ctx context.Context) ([]core.Session, error) {
	ctx, cancel := request(ctx)
	defer cancel()
	if _, e := b.probeVersion(ctx); e != nil {
		return nil, e
	}
	out, diag, _, err := b.output(ctx, core.MaxBytes, false, "list-sessions", "-F", "#{session_id}|#{session_name}|#{session_attached}")
	if err != nil {
		if tmuxEmpty(err, diag) {
			return []core.Session{}, nil
		}
		return nil, err
	}
	result := []core.Session{}
	backendCaps, capErr := b.BackendCapabilities(ctx)
	if capErr != nil {
		return nil, capErr
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, "|")
		if len(f) != 3 || !tmuxSessionID.MatchString(f[0]) || f[1] == "" || seen[f[0]] {
			return nil, core.Scope(core.Failure("protocol_error", "invalid tmux session listing"), b.Name())
		}
		n, e := strconv.Atoi(f[2])
		if e != nil || n < 0 {
			return nil, core.Failure("protocol_error", "invalid tmux attachment count")
		}
		seen[f[0]] = true
		s := observed("tmux", f[1], core.String(f[0]))
		s.Capabilities["managed_launch"] = backendCaps.Capabilities["managed_launch"]
		s.AttachedClients = &n
		s.ClientState = "detached"
		if n > 0 {
			s.ClientState = "attached"
		}
		result = append(result, s)
	}
	sortSessions(result)
	return result, nil
}
func (b *TmuxBackend) session(ctx context.Context, name string) (core.Session, error) {
	s, e := b.Sessions(ctx)
	if e != nil {
		return core.Session{}, e
	}
	v, e := exact(s, name)
	if e != nil {
		return v, core.Scope(e, b.Name())
	}
	return v, nil
}
func (b *TmuxBackend) GetSessions() ([]string, error) { s, e := b.Sessions(b.ctx); return names(s), e }
func (b *TmuxBackend) SessionExists(name string) bool {
	ok, e := b.LookupSession(name)
	return e == nil && ok
}
func (b *TmuxBackend) LookupSession(name string) (bool, error) {
	_, e := b.session(b.ctx, name)
	if e != nil && core.Normalize(e).Code == "not_found" {
		return false, nil
	}
	return e == nil, e
}
func (b *TmuxBackend) CreateSession(name string, command ...string) error {
	found, e := b.LookupSession(name)
	if e != nil {
		return e
	}
	if found {
		return core.Failure("conflict", "session already exists")
	}
	args := []string{"new-session", "-d", "-s", name}
	if len(command) > 0 {
		args = append(args, "--")
		args = append(args, command...)
	}
	return b.run(args...)
}
func (b *TmuxBackend) Windows(ctx context.Context, session string) ([]core.Window, error) {
	ctx, cancel := request(ctx)
	defer cancel()
	s, e := b.session(ctx, session)
	if e != nil {
		return nil, e
	}
	return b.windows(ctx, *s.BackendID)
}
func (b *TmuxBackend) windows(ctx context.Context, id string) ([]core.Window, error) {
	out, _, _, e := b.output(ctx, core.MaxBytes, false, "list-windows", "-t", id, "-F", "#{window_id}|#{window_index}|#{window_name}")
	if e != nil {
		return nil, e
	}
	result := []core.Window{}
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, "|")
		if len(f) != 3 || !tmuxWindowID.MatchString(f[0]) || seen[f[0]] {
			return nil, core.Failure("protocol_error", "invalid tmux window listing")
		}
		n, e := strconv.Atoi(f[1])
		if e != nil || n < 0 {
			return nil, core.Failure("protocol_error", "invalid tmux window index")
		}
		seen[f[0]] = true
		result = append(result, core.Window{ID: f[0], Index: n, Name: f[2]})
	}
	return result, nil
}
func (b *TmuxBackend) window(ctx context.Context, session, target string) (core.Window, error) {
	windows, e := b.Windows(ctx, session)
	if e != nil {
		return core.Window{}, e
	}
	if target == "" && len(windows) > 1 {
		return core.Window{}, core.Failure("ambiguous_target", "specify a window ID when multiple windows exist")
	}
	matches := []core.Window{}
	for _, w := range windows {
		if target == w.ID || target == w.Name || target == strconv.Itoa(w.Index) || (target == "" && len(windows) == 1) {
			matches = append(matches, w)
		}
	}
	if len(matches) == 0 {
		return core.Window{}, core.Failure("not_found", "exact window not found in session")
	}
	if len(matches) > 1 {
		return core.Window{}, core.Failure("ambiguous_target", "window target is ambiguous; use window ID")
	}
	return matches[0], nil
}
func (b *TmuxBackend) Panes(ctx context.Context, session, window string) ([]core.Pane, error) {
	ctx, cancel := request(ctx)
	defer cancel()
	w, e := b.window(ctx, session, window)
	if e != nil {
		return nil, e
	}
	return b.panes(ctx, w.ID)
}
func (b *TmuxBackend) panes(ctx context.Context, window string) ([]core.Pane, error) {
	out, _, _, e := b.output(ctx, core.MaxBytes, false, "list-panes", "-t", window, "-F", "#{pane_id}|#{pane_index}|#{pane_height}")
	if e != nil {
		return nil, e
	}
	result := []core.Pane{}
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, "|")
		if len(f) != 3 || !tmuxPaneID.MatchString(f[0]) || seen[f[0]] {
			return nil, core.Failure("protocol_error", "invalid tmux pane listing")
		}
		index, e := strconv.Atoi(f[1])
		height, he := strconv.Atoi(f[2])
		if e != nil || he != nil || index < 0 || height < 1 {
			return nil, core.Failure("protocol_error", "invalid tmux pane dimensions")
		}
		seen[f[0]] = true
		result = append(result, core.Pane{ID: f[0], Index: index, WindowID: window, Height: height})
	}
	return result, nil
}
func (b *TmuxBackend) pane(ctx context.Context, session, window, target string) (core.Pane, error) {
	panes, e := b.Panes(ctx, session, window)
	if e != nil {
		return core.Pane{}, e
	}
	if target == "" && len(panes) > 1 {
		return core.Pane{}, core.Failure("ambiguous_target", "specify a pane ID when multiple panes exist")
	}
	matches := []core.Pane{}
	for _, p := range panes {
		if target == p.ID || target == strconv.Itoa(p.Index) || (target == "" && len(panes) == 1) {
			matches = append(matches, p)
		}
	}
	if len(matches) == 0 {
		return core.Pane{}, core.Failure("not_found", "exact pane not found in window")
	}
	if len(matches) > 1 {
		return core.Pane{}, core.Failure("ambiguous_target", "pane target is ambiguous; use pane ID")
	}
	return matches[0], nil
}
func (b *TmuxBackend) AttachSession(name string) error  { return b.attach(name, false) }
func (b *TmuxBackend) AttachReadOnly(name string) error { return b.attach(name, true) }
func (b *TmuxBackend) attach(name string, readOnly bool) error {
	s, e := b.session(b.ctx, name)
	if e != nil {
		return e
	}
	args := []string{"attach-session"}
	if readOnly {
		args = append(args, "-r")
	}
	return b.interactive(append(args, "-t", *s.BackendID)...)
}
func (b *TmuxBackend) DetachSession() error {
	if _, e := b.probeVersion(b.ctx); e != nil {
		return e
	}
	return b.run("detach-client")
}
func (b *TmuxBackend) KillSession(name string) error {
	s, e := b.session(b.ctx, name)
	if e != nil {
		return e
	}
	return b.run("kill-session", "-t", *s.BackendID)
}
func (b *TmuxBackend) RenameSession(oldName, newName string) error {
	s, e := b.session(b.ctx, oldName)
	if e != nil {
		return e
	}
	found, e := b.LookupSession(newName)
	if e != nil {
		return e
	}
	if found {
		return core.Failure("conflict", "new session name already exists")
	}
	return b.run("rename-session", "-t", *s.BackendID, newName)
}
func (b *TmuxBackend) NewWindow(session, name string) error {
	s, e := b.session(b.ctx, session)
	if e != nil {
		return e
	}
	args := []string{"new-window", "-t", *s.BackendID + ":"}
	if name != "" {
		args = append(args, "-n", name)
	}
	return b.run(args...)
}
func (b *TmuxBackend) KillWindow(session, window string) error {
	w, e := b.window(b.ctx, session, window)
	if e != nil {
		return e
	}
	return b.run("kill-window", "-t", w.ID)
}
func (b *TmuxBackend) NextWindow(session string) error {
	s, e := b.session(b.ctx, session)
	if e != nil {
		return e
	}
	return b.run("next-window", "-t", *s.BackendID)
}
func (b *TmuxBackend) PreviousWindow(session string) error {
	s, e := b.session(b.ctx, session)
	if e != nil {
		return e
	}
	return b.run("previous-window", "-t", *s.BackendID)
}
func (b *TmuxBackend) RenameWindow(session, oldName, newName string) error {
	w, e := b.window(b.ctx, session, oldName)
	if e != nil {
		return e
	}
	return b.run("rename-window", "-t", w.ID, newName)
}
func (b *TmuxBackend) SplitWindow(session, window, direction string) error {
	if direction != "h" && direction != "v" {
		return core.Failure("invalid_usage", "direction must be h or v")
	}
	w, e := b.window(b.ctx, session, window)
	if e != nil {
		return e
	}
	return b.run("split-window", "-"+direction, "-t", w.ID)
}
func (b *TmuxBackend) KillPane(session, window, pane string) error {
	p, e := b.pane(b.ctx, session, window, pane)
	if e != nil {
		return e
	}
	return b.run("kill-pane", "-t", p.ID)
}
func (b *TmuxBackend) Exec(session, window, pane, command string) error {
	if len(command) > core.MaxBytes {
		return core.Failure("invalid_usage", "input exceeds 1 MiB")
	}
	ctx, cancel := request(b.ctx)
	defer cancel()
	p, e := b.pane(ctx, session, window, pane)
	if e != nil {
		return e
	}
	sync, _, _, e := b.output(ctx, 64, false, "display-message", "-p", "-t", p.ID, "#{synchronize-panes}")
	if e != nil {
		return e
	}
	if strings.TrimSpace(sync) != "0" {
		return unsupported(b.Name(), "exact input while pane synchronization is enabled or unknown")
	}
	sender := b.external
	sender.ctx = ctx
	// Check again in the server queue for every write: another client may
	// enable synchronization after the observation above or between batches.
	send := func(args ...string) error {
		condition := "#{&&:#{==:#{pane_id}," + p.ID + "},#{==:#{synchronize-panes},0}}"
		return sender.run("if-shell", "-F", "-t", p.ID, condition, tmuxCommand(args...), "show-options -t "+p.ID+" __txm_exact_input_refused")
	}
	// Hex mode sends UTF-8 bytes literally and avoids tmux's command-separator
	// parsing (even -l drops a terminal semicolon). Bound each argv batch.
	data := []byte(command)
	const digits = "0123456789abcdef"
	for len(data) > 0 {
		chunk := data[:min(len(data), 4096)]
		args := []string{"send-keys", "-H", "-t", p.ID, "--"}
		for _, c := range chunk {
			args = append(args, string([]byte{digits[c>>4], digits[c&15]}))
		}
		if e = send(args...); e != nil {
			return e
		}
		data = data[len(chunk):]
	}
	return send("send-keys", "-t", p.ID, "Enter")
}
func (b *TmuxBackend) NukeAllSessions() error {
	s, e := b.Sessions(b.ctx)
	if e != nil || len(s) == 0 {
		return e
	}
	return b.run("kill-server")
}
func (b *TmuxBackend) Capture(ctx context.Context, name string, o core.CaptureOptions) (core.Snapshot, error) {
	if e := o.Validate(); e != nil {
		return core.Snapshot{}, e
	}
	ctx, cancel := request(ctx)
	defer cancel()
	s, e := b.session(ctx, name)
	if e != nil {
		return core.Snapshot{}, e
	}
	// Select the active pane once, within the exact session, then capture by ID.
	out, _, _, e := b.output(ctx, 4096, false, "display-message", "-p", "-t", *s.BackendID, "#{pane_id}|#{pane_height}|#{history_size}")
	if e != nil {
		return core.Snapshot{}, e
	}
	f := strings.Split(strings.TrimSpace(out), "|")
	if len(f) != 3 || !tmuxPaneID.MatchString(f[0]) {
		return core.Snapshot{}, core.Failure("protocol_error", "invalid tmux active-pane observation")
	}
	height, e := strconv.Atoi(f[1])
	if e != nil || height < 1 {
		return core.Snapshot{}, core.Failure("protocol_error", "invalid tmux pane height")
	}
	history, he := strconv.Atoi(f[2])
	if he != nil || history < 0 {
		return core.Snapshot{}, core.Failure("protocol_error", "invalid tmux history size")
	}
	start := max(0, height-o.Lines)
	args := []string{"capture-pane", "-p", "-t", f[0], "-S", strconv.Itoa(start), "-E", "-"}
	if o.History {
		args[5] = strconv.Itoa(height - o.Lines)
	}
	if o.Raw {
		args = append(args, "-e")
	}
	output, _, limited, e := b.output(ctx, o.Bytes, true, args...)
	if e != nil {
		return core.Snapshot{}, e
	}
	return core.MakeSnapshot("tmux", s.Locator.Name, []byte(output), o, limited || (height > o.Lines) || (o.History && history+height > o.Lines)), nil
}
func (b *TmuxBackend) probeVersion(ctx context.Context) (string, error) {
	if v := b.version.Load(); v != nil {
		return *v, nil
	}
	out, _, _, e := b.output(ctx, 4096, false, "-V")
	if e != nil {
		return "", e
	}
	v := strings.TrimSpace(out)
	if !regexp.MustCompile(`^tmux 3\.[0-9]+[a-z]?$`).MatchString(v) {
		return "", core.Scope(core.Failure("unsupported_version", "unsupported tmux version; requires tmux 3.x"), b.Name())
	}
	b.version.CompareAndSwap(nil, &v)
	return v, nil
}
func (b *TmuxBackend) BackendCapabilities(ctx context.Context) (core.BackendCapabilities, error) {
	c := core.BackendCapabilities{Backend: b.Name(), Available: b.IsAvailable(), Capabilities: capabilities(b.Name())}
	if !c.Available {
		disableCapabilities(&c, "backend executable unavailable")
		return c, core.Scope(core.Failure("backend_unavailable", "tmux executable unavailable"), b.Name())
	}
	v, e := b.probeVersion(ctx)
	if e != nil {
		disableCapabilities(&c, "compatible backend version could not be verified")
		return c, e
	}
	c.Version = core.String(v)
	if supervisor.Supported() {
		cap := c.Capabilities["managed_launch"]
		minor, e := strconv.Atoi(strings.TrimRight(strings.TrimPrefix(v, "tmux 3."), "abcdefghijklmnopqrstuvwxyz"))
		if e == nil && minor >= 4 {
			cap.Level = "supported"
			cap.Reason = nil
			cap.Constraints = []string{"tmux 3.4+; structured supervisor wrapper", "missing receipt after wrapper/server loss means unknown"}
		} else {
			cap.Level = "unsupported"
			cap.Reason = core.String("managed launch requires tmux 3.4+")
		}
		c.Capabilities["managed_launch"] = cap
	}
	return c, nil
}

// Legacy read wrappers remain for callers migrating to Reader; CLI owns rendering.
func (b *TmuxBackend) ListSessions() error {
	ss, e := b.Sessions(b.ctx)
	if e != nil {
		return e
	}
	for _, s := range ss {
		fmt.Println(s.Locator.Name)
	}
	return nil
}
func (b *TmuxBackend) ListWindows(s string) error {
	ws, e := b.Windows(b.ctx, s)
	if e != nil {
		return e
	}
	for _, w := range ws {
		fmt.Printf("%d: %s (%s)\n", w.Index, w.Name, w.ID)
	}
	return nil
}
func (b *TmuxBackend) ListPanes(s, w string) error {
	ps, e := b.Panes(b.ctx, s, w)
	if e != nil {
		return e
	}
	for _, p := range ps {
		fmt.Printf("%d: %s\n", p.Index, p.ID)
	}
	return nil
}
func (b *TmuxBackend) DumpSession(name string) (string, error) {
	s, e := b.Capture(b.ctx, name, core.CaptureOptions{Lines: 100, Bytes: core.MaxBytes})
	return s.Content, e
}
