package backend

import (
	"context"
	"github.com/MohamedElashri/txm/pkg/core"
	p "github.com/MohamedElashri/txm/pkg/nativeprotocol"
	"github.com/MohamedElashri/txm/pkg/profile"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"os"
	"time"
)

func (b *NativeBackend) profileClient(ctx context.Context, owned profile.Applied, role string) (*p.Client, error) {
	if !runtimepath.ValidID(owned.InstanceID) {
		return nil, core.Failure("conflict", "missing native profile instance receipt")
	}
	root, e := runtimepath.Open()
	if e != nil {
		return nil, e
	}
	defer func() { _ = root.Close() }()
	if _, e = root.SocketInfo(owned.InstanceID); e != nil {
		return nil, e
	}
	path, e := root.SocketPath(owned.InstanceID)
	if e != nil {
		return nil, e
	}
	client, e := p.Dial(ctx, path, role)
	if e != nil {
		return nil, e
	}
	if client.Identity.InstanceID != owned.InstanceID || client.Identity.Name != owned.Name {
		_ = client.Close()
		return nil, core.Failure("conflict", "native profile identity changed")
	}
	return client, nil
}
func (b *NativeBackend) rollbackProfile(owned profile.Applied) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, e := b.profileClient(ctx, owned, "control")
	if os.IsNotExist(e) {
		return true
	}
	if e != nil {
		return false
	}
	return NewNativeBackend(ctx).killClient(client) == nil
}
func (b *NativeBackend) applyProfile(ctx context.Context, plan profile.Plan) (owned profile.Applied, retErr error) {
	owned = profile.Applied{Backend: "native", Name: plan.Name}
	if !b.IsAvailable() {
		return owned, core.Failure("backend_unavailable", "native runtime unavailable")
	}
	found, e := NewNativeBackend(ctx).LookupSession(plan.Name)
	if e != nil {
		return owned, nativeError(e)
	}
	if found {
		return owned, core.Failure("conflict", "profile session already exists")
	}
	id, e := runtimepath.NewID()
	if e != nil {
		return owned, e
	}
	owned.InstanceID = id
	owned.SessionID = id
	if e = startNativeInDir(ctx, plan.Name, id, []string{plan.Shell, "-i"}, plan.Windows[0].Panes[0].Dir); e != nil {
		return owned, nativeError(e)
	}
	defer func() {
		if retErr != nil {
			retErr = profileFailure(retErr, owned, b.rollbackProfile(owned))
		}
	}()
	command := plan.Windows[0].Panes[0].Command
	if command != "" {
		c, e := b.profileClient(ctx, owned, "control")
		if e != nil {
			return owned, nativeError(e)
		}
		defer func() { _ = c.Close() }()
		if e = c.CallContext(ctx, p.Operation{Op: "input", Data: []byte(command + "\n")}, nil); e != nil {
			return owned, nativeError(e)
		}
	}
	return owned, nil
}
func (b *NativeBackend) attachProfile(ctx context.Context, owned profile.Applied, readOnly bool) error {
	role := "control"
	if readOnly {
		role = "observer"
	}
	c, e := b.profileClient(ctx, owned, role)
	if e != nil {
		return nativeError(e)
	}
	defer func() { _ = c.Close() }()
	return attachNative(c, readOnly)
}
