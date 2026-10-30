package backend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/MohamedElashri/txm/pkg/core"
	p "github.com/MohamedElashri/txm/pkg/nativeprotocol"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
)

type NativeBackend struct{ ctx context.Context }

func NewNativeBackend(ctx ...context.Context) *NativeBackend {
	base := context.Background()
	if len(ctx) > 0 && ctx[0] != nil {
		base = ctx[0]
	}
	return &NativeBackend{ctx: base}
}
func (b *NativeBackend) context() context.Context {
	if b.ctx == nil {
		return context.Background()
	}
	return b.ctx
}
func (b *NativeBackend) Name() string      { return "native" }
func (b *NativeBackend) IsAvailable() bool { return runtimepath.Supported() && nativeServerSupported() }

type nativeSession struct {
	identity p.Identity
	path     string
}

func (b *NativeBackend) sessions(ctx context.Context) ([]nativeSession, error) {
	root, err := runtimepath.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	names, err := root.Names(ctx)
	if err != nil {
		return nil, err
	}
	var sessions []nativeSession
	for _, name := range names {
		if !strings.HasPrefix(name, "s-") || !strings.HasSuffix(name, ".sock") {
			continue
		}
		id := strings.TrimSuffix(strings.TrimPrefix(name, "s-"), ".sock")
		if !runtimepath.ValidID(id) {
			continue
		}
		if _, err = root.SocketInfo(id); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		path, err := root.SocketPath(id)
		if err != nil {
			return nil, err
		}
		c, err := p.Dial(ctx, path, "observer")
		if err != nil {
			var wire *p.Error
			if staleNativeSocket(err) || (errors.As(err, &wire) && wire.Code == "not_found") {
				continue
			}
			return nil, fmt.Errorf("native discovery: %w", err)
		}
		identity := c.Identity
		_ = c.Close()
		if identity.InstanceID != id {
			return nil, fmt.Errorf("native socket/instance mismatch")
		}
		sessions = append(sessions, nativeSession{identity, path})
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].identity.Name < sessions[j].identity.Name })
	return sessions, nil
}
func (b *NativeBackend) lookup(ctx context.Context, name string) (nativeSession, error) {
	if err := runtimepath.ValidateName(name); err != nil {
		return nativeSession{}, err
	}
	sessions, err := b.sessions(ctx)
	if err != nil {
		return nativeSession{}, err
	}
	var found *nativeSession
	for i := range sessions {
		if sessions[i].identity.Name == name {
			if found != nil {
				return nativeSession{}, p.Failure("conflict", "duplicate native session name")
			}
			found = &sessions[i]
		}
	}
	if found != nil {
		return *found, nil
	}
	if ownedLegacySocket(name) {
		return nativeSession{}, p.Failure("unsupported_version", "legacy native session: finish it with the old txm binary; its socket was left untouched")
	}
	return nativeSession{}, p.Failure("not_found", "native session does not exist")
}
func (b *NativeBackend) LookupSession(name string) (bool, error) {
	ctx, cancel := context.WithTimeout(b.context(), p.RequestTimeout)
	defer cancel()
	_, err := b.lookup(ctx, name)
	var wire *p.Error
	if errors.As(err, &wire) && wire.Code == "not_found" {
		return false, nil
	}
	return err == nil, err
}
func (b *NativeBackend) SessionExists(name string) bool {
	found, err := b.LookupSession(name)
	return err == nil && found
}
func (b *NativeBackend) CreateSession(name string, command ...string) error {
	found, err := b.LookupSession(name)
	if err != nil {
		return err
	}
	if found {
		return p.Failure("conflict", "native session already exists")
	}
	id, err := runtimepath.NewID()
	if err != nil {
		return err
	}
	return startNative(b.context(), name, id, command)
}
func (b *NativeBackend) GetSessions() ([]string, error) {
	ctx, cancel := context.WithTimeout(b.context(), p.RequestTimeout)
	defer cancel()
	sessions, err := b.sessions(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(sessions))
	for _, s := range sessions {
		names = append(names, s.identity.Name)
	}
	return names, nil
}
func (b *NativeBackend) client(name, role string) (*p.Client, error) {
	ctx, cancel := context.WithTimeout(b.context(), p.RequestTimeout)
	defer cancel()
	s, err := b.lookup(ctx, name)
	if err != nil {
		return nil, err
	}
	c, err := p.Dial(ctx, s.path, role)
	if err != nil {
		return nil, err
	}
	if c.Identity.InstanceID != s.identity.InstanceID || c.Identity.Name != name {
		_ = c.Close()
		return nil, p.Failure("conflict", "native instance changed")
	}
	return c, nil
}
func (b *NativeBackend) ListSessions() error {
	names, err := b.GetSessions()
	if err != nil {
		return err
	}
	for _, name := range names {
		c, err := b.client(name, "observer")
		if err != nil {
			return err
		}
		var status p.Status
		err = c.CallContext(b.context(), p.Operation{Op: "status"}, &status)
		_ = c.Close()
		if err != nil {
			return err
		}
		fmt.Printf("%s [Attached: %d]\n", name, status.AttachedClients)
	}
	if names := legacyNativeNames(); len(names) > 0 {
		fmt.Fprintln(os.Stderr, "Warning: incompatible legacy native sessions (left untouched):", strings.Join(names, ", "))
	}
	return nil
}
func (b *NativeBackend) DumpSession(name string) (string, error) {
	snap, err := b.Capture(b.context(), name, core.CaptureOptions{Lines: 100, Bytes: core.MaxBytes})
	return snap.Content, err
}
func (b *NativeBackend) AttachSession(name string) error {
	return b.attach(name, os.Getenv("TXM_READ_ONLY") == "1")
}
func (b *NativeBackend) AttachReadOnly(name string) error { return b.attach(name, true) }
func (b *NativeBackend) attach(name string, readOnly bool) error {
	role := "control"
	if readOnly {
		role = "observer"
	}
	c, err := b.client(name, role)
	if err != nil {
		return err
	}
	defer func() { _ = c.Close() }()
	return attachNative(c, readOnly)
}
func (b *NativeBackend) DetachSession() error {
	return unsupported("native", "CLI detach; use Ctrl+backslash")
}
func (b *NativeBackend) KillSession(name string) error {
	c, err := b.client(name, "control")
	if err != nil {
		return err
	}
	return b.killClient(c)
}
func (b *NativeBackend) killClient(c *p.Client) error {
	id := c.Identity.InstanceID
	root, err := runtimepath.Open()
	if err != nil {
		_ = c.Close()
		return err
	}
	defer func() { _ = root.Close() }()
	original, err := root.SocketInfo(id)
	if err != nil {
		_ = c.Close()
		return err
	}
	err = c.CallContext(b.context(), p.Operation{Op: "kill"}, nil)
	_ = c.Close()
	if err != nil {
		return err
	}
	deadline := time.Now().Add(p.RequestTimeout)
	for time.Now().Before(deadline) {
		if err := b.context().Err(); err != nil {
			return err
		}
		info, e := root.SocketInfo(id)
		if os.IsNotExist(e) {
			return nil
		}
		if e != nil {
			return e
		}
		if !os.SameFile(original, info) {
			return p.Failure("conflict", "native socket replaced during shutdown")
		}
		time.Sleep(20 * time.Millisecond)
	}
	return p.Failure("timeout", "native shutdown not confirmed")
}
func (b *NativeBackend) RenameSession(oldName, newName string) error {
	return unsupported("native", "session rename")
}

func (b *NativeBackend) NewWindow(session, name string) error {
	return unsupported("native", "window management")
}

func (b *NativeBackend) ListWindows(session string) error {
	return unsupported("native", "window management")
}

func (b *NativeBackend) KillWindow(session, window string) error {
	return unsupported("native", "window management")
}

func (b *NativeBackend) NextWindow(session string) error {
	return unsupported("native", "window management")
}

func (b *NativeBackend) PreviousWindow(session string) error {
	return unsupported("native", "window management")
}

func (b *NativeBackend) RenameWindow(session, oldName, newName string) error {
	return unsupported("native", "window management")
}

func (b *NativeBackend) SplitWindow(session, window, direction string) error {
	return unsupported("native", "window management")
}

func (b *NativeBackend) ListPanes(session, window string) error {
	return unsupported("native", "pane management")
}

func (b *NativeBackend) KillPane(session, window, pane string) error {
	return unsupported("native", "pane management")
}

func (b *NativeBackend) Exec(session, window, pane, command string) error {
	if (window != "" && window != "0") || (pane != "" && pane != "0") {
		return unsupported("native", "input outside window 0 and pane 0")
	}
	if len(command)+1 > p.MaxData {
		return p.Failure("limit_exceeded", "native input exceeds 16 KiB")
	}
	c, err := b.client(session, "control")
	if err != nil {
		return err
	}
	defer func() { _ = c.Close() }()
	return c.CallContext(b.context(), p.Operation{Op: "input", Data: []byte(command + "\n")}, nil)
}
func (b *NativeBackend) NukeAllSessions() error {
	if len(legacyNativeNames()) > 0 {
		return p.Failure("unsupported_version", "legacy native sessions are incompatible; finish them with the old binary before nuke")
	}
	names, err := b.GetSessions()
	if err != nil {
		return err
	}
	var failures []error
	for _, name := range names {
		if e := b.KillSession(name); e != nil {
			failures = append(failures, e)
		}
	}
	return errors.Join(failures...)
}

// LegacyNativeSessions reports owned candidates without opening or unlinking them.
func LegacyNativeSessions() []string { return legacyNativeNames() }
