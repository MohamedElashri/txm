package agent

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/localipc"
	"github.com/MohamedElashri/txm/pkg/nativeprotocol"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"github.com/MohamedElashri/txm/pkg/supervisor"
)

func discover(r *runtimepath.Root) (Discovery, string, error) {
	var result Discovery
	data, e := r.ReadPrivate("coordinator.json", 256)
	if e != nil {
		return result, "", e
	}
	if nativeprotocol.Decode(data, &result) != nil || result.SchemaVersion != 1 || !runtimepath.ValidID(result.Epoch) {
		return result, "", core.Failure("invalid_config", "corrupt coordinator discovery metadata")
	}
	if _, e = r.SocketInfo(result.Epoch); e != nil {
		return result, "", e
	}
	path, e := r.SocketPath(result.Epoch)
	return result, path, e
}
func call(ctx context.Context, r *runtimepath.Root, q Request) (Reply, error) {
	var reply Reply
	d, path, e := discover(r)
	if e != nil {
		return reply, e
	}
	c, e := (&net.Dialer{}).DialContext(ctx, "unix", path)
	if e != nil {
		return reply, e
	}
	defer func() { _ = c.Close() }()
	if e = runtimepath.VerifyPeer(c); e != nil {
		return reply, e
	}
	stop := context.AfterFunc(ctx, func() { _ = c.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	_ = c.SetDeadline(deadline)
	q.SchemaVersion = 1
	q.Epoch = d.Epoch
	if e = localipc.Write(c, q); e != nil {
		return reply, e
	}
	if e = localipc.Read(c, &reply); e != nil {
		return reply, e
	}
	if reply.SchemaVersion != 1 || reply.Epoch != d.Epoch || reply.Agents == nil || (reply.Error != nil && (reply.Error.Code == "" || reply.Error.Message == "")) {
		return reply, core.Failure("protocol_error", "coordinator response identity mismatch")
	}
	if reply.Error != nil {
		return reply, reply.Error
	}
	return reply, nil
}
func unavailable(e error) bool {
	if os.IsNotExist(e) {
		return true
	}
	var op *net.OpError
	return errors.As(e, &op) && op.Op == "dial" && staleCoordinator(e)
}
func identify(ctx context.Context, q Request) (Request, error) {
	host, e := runtimepath.HostID(ctx)
	if e != nil {
		return q, core.Failure("invalid_config", "agent state identity unavailable or unsafe")
	}
	q.HostID = host
	state, e := runtimepath.OpenState()
	if e != nil {
		return q, core.Failure("invalid_config", "agent state directory unavailable or unsafe")
	}
	q.StateRoot = filepath.Join(state.Path, "agents")
	_ = state.Close()
	return q, nil
}
func Call(parent context.Context, q Request, start bool) (reply Reply, retErr error) {
	if !supervisor.Supported() {
		return reply, core.Failure("unsupported", "agent runtime requires Unix and CGO")
	}
	ctx, cancel := Context(parent)
	defer cancel()
	defer func() {
		if ctx.Err() != nil {
			retErr = ctx.Err()
		}
	}()
	q, e := identify(ctx, q)
	if e != nil {
		return reply, e
	}
	r, e := supervisor.Runtime()
	if e != nil {
		return reply, e
	}
	defer func() { _ = r.Close() }()
	defer func() {
		if q.Op != "stop" || retErr != nil {
			return
		}
		for {
			_, e := r.SocketInfo(reply.Epoch)
			if os.IsNotExist(e) {
				return
			}
			if e != nil {
				retErr = e
				return
			}
			select {
			case <-ctx.Done():
				retErr = ctx.Err()
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
	}()
	reply, e = call(ctx, r, q)
	if e == nil || !unavailable(e) {
		return reply, e
	}
	if !start {
		return reply, core.Failure("not_found", "coordinator is not running")
	}
	for {
		lock, err := r.Lock("agent-startup")
		if err == nil {
			// Recheck after obtaining startup serialization. A competing client
			// may have published a ready coordinator while we waited.
			reply, e = call(ctx, r, q)
			if e != nil && unavailable(e) {
				e = startCoordinator(ctx)
				if e == nil {
					reply, e = call(ctx, r, q)
				}
			}
			_ = lock.Close()
			return reply, e
		}
		if !lockBusy(err) {
			return reply, err
		}
		select {
		case <-ctx.Done():
			return reply, ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
		reply, e = call(ctx, r, q)
		if e == nil || !unavailable(e) {
			return reply, e
		}
	}
}

// Notify is best effort after durable publication and never starts a service.
func Notify() {
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	_, _ = Call(ctx, Request{Op: "refresh"}, false)
}
