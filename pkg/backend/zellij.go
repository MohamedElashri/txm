package backend

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/MohamedElashri/txm/pkg/core"
)

var commonZellijPaths = []string{"/usr/bin/zellij", "/usr/local/bin/zellij", "/opt/homebrew/bin/zellij", "/home/linuxbrew/.linuxbrew/bin/zellij"}

type ZellijBackend struct{ external }

func NewZellijBackend(ctx ...context.Context) *ZellijBackend {
	return &ZellijBackend{newExternal("zellij", commonZellijPaths, ctx...)}
}
func (b *ZellijBackend) Name() string      { return "zellij" }
func (b *ZellijBackend) IsAvailable() bool { return b.path != "" }
func (b *ZellijBackend) Sessions(ctx context.Context) ([]core.Session, error) {
	out, diag, _, e := b.output(ctx, core.MaxBytes, false, "list-sessions", "--short", "--no-formatting")
	empty := "No active zellij sessions found."
	if strings.TrimSpace(out) == empty || strings.TrimSpace(diag) == empty {
		if e == nil {
			return []core.Session{}, nil
		}
		var exit interface{ ExitCode() int }
		if errors.As(e, &exit) && exit.ExitCode() == 1 {
			return []core.Session{}, nil
		}
	}
	if e != nil {
		return nil, e
	}
	result := []core.Session{}
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		if strings.TrimSpace(line) != line || strings.ContainsAny(line, "\t\r\x1b") || seen[line] {
			return nil, core.Failure("protocol_error", "invalid or duplicate zellij session listing")
		}
		seen[line] = true
		result = append(result, observed("zellij", line, core.String(line)))
	}
	sortSessions(result)
	return result, nil
}
func (b *ZellijBackend) session(name string) (core.Session, error) {
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
func (b *ZellijBackend) GetSessions() ([]string, error) {
	s, e := b.Sessions(b.ctx)
	return names(s), e
}
func (b *ZellijBackend) LookupSession(name string) (bool, error) {
	_, e := b.session(name)
	if e != nil && core.Normalize(e).Code == "not_found" {
		return false, nil
	}
	return e == nil, e
}
func (b *ZellijBackend) SessionExists(name string) bool {
	ok, e := b.LookupSession(name)
	return ok && e == nil
}
func (b *ZellijBackend) CreateSession(name string, command ...string) error {
	found, e := b.LookupSession(name)
	if e != nil {
		return e
	}
	if found {
		return core.Failure("conflict", "session already exists")
	}
	args := []string{"attach", "--create-background", name}
	if len(command) > 0 {
		args = append(args, "--")
		args = append(args, command...)
	}
	if e = b.run(args...); e != nil {
		return e
	}
	found, e = b.LookupSession(name)
	if e != nil {
		return e
	}
	if !found {
		return core.Failure("operation_failed", "created session could not be observed")
	}
	return nil
}
func (b *ZellijBackend) AttachSession(name string) error {
	s, e := b.session(name)
	if e != nil {
		return e
	}
	return b.interactive("attach", s.Locator.Name)
}
func (b *ZellijBackend) KillSession(name string) error {
	s, e := b.session(name)
	if e != nil {
		return e
	}
	return b.run("kill-session", s.Locator.Name)
}
func (b *ZellijBackend) RenameSession(string, string) error {
	return unsupported(b.Name(), "session rename")
}
func (b *ZellijBackend) NewWindow(session, name string) error {
	s, e := b.session(session)
	if e != nil {
		return e
	}
	args := []string{"-s", s.Locator.Name, "action", "new-tab"}
	if name != "" {
		args = append(args, "--name", name)
	}
	return b.run(args...)
}
func (b *ZellijBackend) NextWindow(session string) error {
	s, e := b.session(session)
	if e != nil {
		return e
	}
	return b.run("-s", s.Locator.Name, "action", "go-to-next-tab")
}
func (b *ZellijBackend) PreviousWindow(session string) error {
	s, e := b.session(session)
	if e != nil {
		return e
	}
	return b.run("-s", s.Locator.Name, "action", "go-to-previous-tab")
}
func (b *ZellijBackend) KillWindow(string, string) error {
	return unsupported(b.Name(), "exact window close")
}
func (b *ZellijBackend) RenameWindow(string, string, string) error {
	return unsupported(b.Name(), "exact window rename")
}
func (b *ZellijBackend) SplitWindow(string, string, string) error {
	return unsupported(b.Name(), "exact window split")
}
func (b *ZellijBackend) KillPane(string, string, string) error {
	return unsupported(b.Name(), "exact pane close")
}
func (b *ZellijBackend) Exec(string, string, string, string) error {
	return unsupported(b.Name(), "exact pane input")
}
func (b *ZellijBackend) DetachSession() error { return unsupported(b.Name(), "CLI detach") }
func (b *ZellijBackend) Windows(context.Context, string) ([]core.Window, error) {
	return nil, unsupported(b.Name(), "typed window listing")
}
func (b *ZellijBackend) Panes(context.Context, string, string) ([]core.Pane, error) {
	return nil, unsupported(b.Name(), "pane listing")
}
func (b *ZellijBackend) Capture(context.Context, string, core.CaptureOptions) (core.Snapshot, error) {
	return core.Snapshot{}, unsupported(b.Name(), "output snapshot")
}
func (b *ZellijBackend) DumpSession(string) (string, error) {
	return "", unsupported(b.Name(), "output snapshot")
}
func (b *ZellijBackend) ListSessions() error {
	s, e := b.Sessions(b.ctx)
	if e != nil {
		return e
	}
	for _, v := range s {
		fmt.Println(v.Locator.Name)
	}
	return nil
}
func (b *ZellijBackend) ListWindows(string) error {
	return unsupported(b.Name(), "typed window listing")
}
func (b *ZellijBackend) ListPanes(string, string) error { return unsupported(b.Name(), "pane listing") }
func (b *ZellijBackend) NukeAllSessions() error {
	s, e := b.Sessions(b.ctx)
	if e != nil {
		return e
	}
	var failures []error
	for _, v := range s {
		if e = b.KillSession(v.Locator.Name); e != nil {
			failures = append(failures, e)
		}
	}
	return errors.Join(failures...)
}
func (b *ZellijBackend) BackendCapabilities(ctx context.Context) (core.BackendCapabilities, error) {
	return externalCapabilities(ctx, b.external, "--version")
}
