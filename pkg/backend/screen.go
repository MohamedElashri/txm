package backend

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/MohamedElashri/txm/pkg/core"
)

type ScreenBackend struct{ external }

func NewScreenBackend(ctx ...context.Context) *ScreenBackend {
	return &ScreenBackend{newExternal("screen", nil, ctx...)}
}
func (b *ScreenBackend) Name() string      { return "screen" }
func (b *ScreenBackend) IsAvailable() bool { return b.path != "" }

var screenID = regexp.MustCompile(`^[0-9]+\..+$`)
var screenSummary = regexp.MustCompile(`(?m)^([0-9]+) Sockets? in [^\r\n]+\.\r?$`)

func (b *ScreenBackend) Sessions(ctx context.Context) ([]core.Session, error) {
	out, diag, _, e := b.output(ctx, core.MaxBytes, false, "-ls")
	legacyDarwin := false
	if e != nil {
		var exit interface{ ExitCode() int }
		if errors.As(e, &exit) && exit.ExitCode() == 1 && diag == "" && strings.HasPrefix(strings.TrimSpace(out), "No Sockets found in ") {
			return []core.Session{}, nil
		}
		// Apple's Screen 4.00.03 exits 1 for a valid populated listing too.
		// Accept only that platform/status, no stderr, and a matching parsed
		// session-count summary below. Other command failures remain errors.
		legacyDarwin = runtime.GOOS == "darwin" && errors.As(e, &exit) && exit.ExitCode() == 1 && diag == ""
		if !legacyDarwin {
			return nil, e
		}
	}
	result := []core.Session{}
	seen := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "(Attached)") && !strings.Contains(line, "(Detached)") {
			continue
		}
		f := strings.Fields(line)
		if len(f) == 0 || !screenID.MatchString(f[0]) || seen[f[0]] {
			return nil, core.Failure("protocol_error", "invalid screen session listing")
		}
		seen[f[0]] = true
		parts := strings.SplitN(f[0], ".", 2)
		s := observed("screen", parts[1], core.String(f[0]))
		s.ClientState = "detached"
		if strings.Contains(line, "(Attached)") {
			s.ClientState = "attached"
		}
		result = append(result, s)
	}
	if legacyDarwin {
		summary := screenSummary.FindStringSubmatch(out)
		if len(result) == 0 || len(summary) != 2 || summary[1] != strconv.Itoa(len(result)) {
			return nil, e
		}
	}
	// Unknown successful output must not be mistaken for an empty server.
	if len(result) == 0 && strings.TrimSpace(out) != "" && !strings.HasPrefix(strings.TrimSpace(out), "No Sockets found in ") {
		return nil, core.Failure("protocol_error", "unrecognized screen session listing")
	}
	sortSessions(result)
	return result, nil
}
func (b *ScreenBackend) session(name string) (core.Session, error) {
	s, e := b.Sessions(b.ctx)
	if e != nil {
		return core.Session{}, e
	}
	v, e := exact(s, name)
	if e != nil {
		return v, core.Scope(e, b.Name())
	}
	return v, nil
}
func (b *ScreenBackend) GetSessions() ([]string, error) {
	s, e := b.Sessions(b.ctx)
	return names(s), e
}
func (b *ScreenBackend) LookupSession(name string) (bool, error) {
	_, e := b.session(name)
	if e != nil && core.Normalize(e).Code == "not_found" {
		return false, nil
	}
	return e == nil, e
}
func (b *ScreenBackend) SessionExists(name string) bool {
	ok, e := b.LookupSession(name)
	return ok && e == nil
}
func (b *ScreenBackend) CreateSession(name string, command ...string) error {
	found, e := b.LookupSession(name)
	if e != nil {
		return e
	}
	if found {
		return core.Failure("conflict", "session already exists")
	}
	return b.run(append([]string{"-dmS", name}, command...)...)
}
func (b *ScreenBackend) AttachSession(name string) error {
	s, e := b.session(name)
	if e != nil {
		return e
	}
	return b.interactive("-r", *s.BackendID)
}
func (b *ScreenBackend) DetachSession() error {
	return unsupported(b.Name(), "CLI detach; use Ctrl-A d")
}
func (b *ScreenBackend) KillSession(name string) error {
	s, e := b.session(name)
	if e != nil {
		return e
	}
	return b.run("-S", *s.BackendID, "-X", "quit")
}
func (b *ScreenBackend) RenameSession(string, string) error {
	return unsupported(b.Name(), "session rename")
}
func (b *ScreenBackend) NewWindow(session, name string) error {
	s, e := b.session(session)
	if e != nil {
		return e
	}
	return b.run("-S", *s.BackendID, "-X", "screen", "-t", name)
}
func (b *ScreenBackend) NextWindow(session string) error {
	s, e := b.session(session)
	if e != nil {
		return e
	}
	return b.run("-S", *s.BackendID, "-X", "next")
}
func (b *ScreenBackend) PreviousWindow(session string) error {
	s, e := b.session(session)
	if e != nil {
		return e
	}
	return b.run("-S", *s.BackendID, "-X", "prev")
}
func (b *ScreenBackend) KillWindow(string, string) error {
	return unsupported(b.Name(), "exact window close")
}
func (b *ScreenBackend) RenameWindow(string, string, string) error {
	return unsupported(b.Name(), "exact window rename")
}
func (b *ScreenBackend) SplitWindow(string, string, string) error {
	return unsupported(b.Name(), "exact window split")
}
func (b *ScreenBackend) KillPane(string, string, string) error {
	return unsupported(b.Name(), "exact pane close")
}
func (b *ScreenBackend) Exec(string, string, string, string) error {
	return unsupported(b.Name(), "exact pane input")
}
func (b *ScreenBackend) Windows(context.Context, string) ([]core.Window, error) {
	return nil, unsupported(b.Name(), "typed window listing")
}
func (b *ScreenBackend) Panes(context.Context, string, string) ([]core.Pane, error) {
	return nil, unsupported(b.Name(), "pane listing")
}
func (b *ScreenBackend) Capture(context.Context, string, core.CaptureOptions) (core.Snapshot, error) {
	return core.Snapshot{}, unsupported(b.Name(), "output snapshot")
}
func (b *ScreenBackend) DumpSession(string) (string, error) {
	return "", unsupported(b.Name(), "output snapshot")
}
func (b *ScreenBackend) ListSessions() error {
	s, e := b.Sessions(b.ctx)
	if e != nil {
		return e
	}
	for _, v := range s {
		fmt.Println(v.Locator.Name)
	}
	return nil
}
func (b *ScreenBackend) ListWindows(string) error {
	return unsupported(b.Name(), "typed window listing")
}
func (b *ScreenBackend) ListPanes(string, string) error { return unsupported(b.Name(), "pane listing") }
func (b *ScreenBackend) NukeAllSessions() error {
	s, e := b.Sessions(b.ctx)
	if e != nil {
		return e
	}
	var failures []error
	for _, v := range s {
		if e = b.KillSession(*v.BackendID); e != nil {
			failures = append(failures, e)
		}
	}
	return errors.Join(failures...)
}
func (b *ScreenBackend) BackendCapabilities(ctx context.Context) (core.BackendCapabilities, error) {
	return externalCapabilities(ctx, b.external, "-v")
}
func externalCapabilities(ctx context.Context, b external, flag string) (core.BackendCapabilities, error) {
	c := core.BackendCapabilities{Backend: b.name, Available: b.path != "", Capabilities: capabilities(b.name)}
	if !c.Available {
		disableCapabilities(&c, "backend executable unavailable")
		return c, core.Scope(core.Failure("backend_unavailable", "backend executable unavailable"), b.name)
	}
	out, diag, _, e := b.output(ctx, 4096, false, flag)
	if e != nil && runtime.GOOS == "darwin" && b.name == "screen" && flag == "-v" && diag == "" && strings.TrimSpace(out) == "Screen version 4.00.03 (FAU) 23-Oct-06" {
		var exit interface{ ExitCode() int }
		if errors.As(e, &exit) && exit.ExitCode() == 1 {
			e = nil // Apple's legacy version banner has the same exit-1 convention.
		}
	}
	if e != nil {
		disableCapabilities(&c, "backend version probe failed")
		return c, e
	}
	v := strings.TrimSpace(out)
	prefix := "zellij "
	if b.name == "screen" {
		prefix = "Screen version "
	}
	if !strings.HasPrefix(v, prefix) {
		disableCapabilities(&c, "unrecognized backend version")
		return c, core.Failure("protocol_error", "unrecognized backend version")
	}
	suffix := strings.TrimPrefix(v, prefix)
	if len(suffix) == 0 {
		disableCapabilities(&c, "empty backend version")
		return c, core.Failure("protocol_error", "empty backend version")
	}
	first := strings.SplitN(suffix, ".", 2)[0]
	if _, e = strconv.Atoi(first); e != nil {
		disableCapabilities(&c, "invalid backend version")
		return c, core.Failure("protocol_error", "invalid backend version")
	}
	c.Version = core.String(v)
	return c, nil
}
