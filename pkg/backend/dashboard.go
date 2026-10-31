package backend

import (
	"context"
	"github.com/MohamedElashri/txm/pkg/core"
	p "github.com/MohamedElashri/txm/pkg/nativeprotocol"
	"github.com/MohamedElashri/txm/pkg/profile"
	"time"
)

// dashboardTarget revalidates a transient M1 observation, never creating a
// session or resolving an action by focus. Unmanaged tmux IDs are server scoped.
func dashboardTarget(ctx context.Context, b TerminalMultiplexer, old core.Session, operation string) (core.Session, error) {
	if b.Name() != old.Locator.Backend || old.BackendID == nil {
		return core.Session{}, core.Failure("conflict", "dashboard target has no matching backend identity")
	}
	if b.Name() != "native" && b.Name() != "tmux" {
		return core.Session{}, unsupported(b.Name(), "dashboard action")
	}
	target := *old.BackendID
	if b.Name() == "native" {
		target = old.Locator.Name
	}
	current, e := b.Lookup(ctx, target)
	if e != nil {
		return current, e
	}
	if current.BackendID == nil || *current.BackendID != *old.BackendID || current.Locator.Name != old.Locator.Name {
		return current, core.Failure("conflict", "dashboard target changed; refresh and reselect")
	}
	if b.Name() == "native" && (old.InstanceID == nil || current.InstanceID == nil || *old.InstanceID != *current.InstanceID) {
		return current, core.Failure("conflict", "native launch instance changed")
	}
	if current.Capabilities[operation].Level != "supported" {
		return current, unsupported(b.Name(), operation)
	}
	return current, nil
}
func AttachObserved(ctx context.Context, b TerminalMultiplexer, s core.Session, readOnly bool) error {
	operation := "attach"
	if readOnly {
		operation = "read_only_attach"
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if _, e := dashboardTarget(bounded, b, s, operation); e != nil {
		return e
	}
	switch adapter := b.(type) {
	case *NativeBackend:
		role := "control"
		if readOnly {
			role = "observer"
		}
		c, e := adapter.profileClient(bounded, profile.Applied{Name: s.Locator.Name, InstanceID: *s.InstanceID}, role)
		if e != nil {
			return nativeError(e)
		}
		defer func() { _ = c.Close() }()
		stop := context.AfterFunc(ctx, func() { _ = c.Close() })
		defer stop()
		e = attachNative(c, readOnly)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return e
	case *TmuxBackend:
		args := []string{"attach-session"}
		if readOnly {
			args = append(args, "-r")
		}
		bound := adapter.external
		bound.ctx = ctx
		return bound.interactive(append(args, "-t", *s.BackendID)...)
	}
	return unsupported(b.Name(), "dashboard attach")
}
func CaptureObserved(ctx context.Context, b TerminalMultiplexer, s core.Session, o core.CaptureOptions) (core.Snapshot, error) {
	if e := o.Validate(); e != nil {
		return core.Snapshot{}, e
	}
	if _, e := dashboardTarget(ctx, b, s, "output_snapshot"); e != nil {
		return core.Snapshot{}, e
	}
	if adapter, ok := b.(*NativeBackend); ok {
		if o.History {
			return core.Snapshot{}, unsupported("native", "output history")
		}
		c, e := adapter.profileClient(ctx, profile.Applied{Name: s.Locator.Name, InstanceID: *s.InstanceID}, "observer")
		if e != nil {
			return core.Snapshot{}, nativeError(e)
		}
		defer func() { _ = c.Close() }()
		return captureNativeClient(ctx, c, s.Locator.Name, o)
	}
	return b.Capture(ctx, *s.BackendID, o)
}
func captureNativeClient(ctx context.Context, c *p.Client, name string, o core.CaptureOptions) (core.Snapshot, error) {
	format := "plain"
	if o.Raw {
		format = "vt"
	}
	var snap p.Snapshot
	e := c.CallContext(ctx, p.Operation{Op: "snapshot", Format: format, Lines: o.Lines, MaxBytes: o.Bytes}, &snap)
	if e != nil {
		return core.Snapshot{}, nativeError(e)
	}
	result := core.MakeSnapshot("native", name, snap.Content, o, snap.Truncated)
	result.ObservedAt = snap.ObservedAt
	return result, nil
}
