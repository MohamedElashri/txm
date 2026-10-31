package supervisor

import (
	"context"
	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/localipc"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"net"
	"time"
)

func Probe(ctx context.Context, managed, id string) (Status, error) {
	return probe(ctx, managed, id, false)
}

// ProbeExisting never creates a runtime namespace or starts a service.
func ProbeExisting(ctx context.Context, managed, id string) (Status, error) {
	return probe(ctx, managed, id, true)
}

func probe(ctx context.Context, managed, id string, existing bool) (Status, error) {
	var result Status
	if !runtimepath.ValidID(managed) || !runtimepath.ValidID(id) {
		return result, core.Failure("invalid_config", "invalid supervisor identity")
	}
	var r *runtimepath.Root
	var e error
	if existing {
		var parent *runtimepath.Root
		parent, e = runtimepath.OpenExisting()
		if e == nil {
			r, e = parent.ChildExisting("agents")
			_ = parent.Close()
		}
	} else {
		r, e = Runtime()
	}
	if e != nil {
		return result, e
	}
	defer func() { _ = r.Close() }()
	if _, e = r.SocketInfo(id); e != nil {
		return result, e
	}
	path, e := r.SocketPath(id)
	if e != nil {
		return result, e
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	c, e := (&net.Dialer{}).DialContext(ctx, "unix", path)
	if e != nil {
		return result, e
	}
	defer func() { _ = c.Close() }()
	stop := context.AfterFunc(ctx, func() { _ = c.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	_ = c.SetDeadline(deadline)
	if e = runtimepath.VerifyPeer(c); e != nil {
		return result, e
	}
	if e = localipc.Read(c, &result); e != nil {
		return result, e
	}
	if result.SchemaVersion != 1 || result.ManagedID != managed || result.InstanceID != id || result.ObservedAt.IsZero() {
		return result, core.Failure("protocol_error", "supervisor identity mismatch")
	}
	return result, nil
}
