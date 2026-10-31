package agent

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"time"

	"github.com/MohamedElashri/txm/pkg/core"
	"github.com/MohamedElashri/txm/pkg/localipc"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"github.com/MohamedElashri/txm/pkg/supervisor"
)

// Stream reconnects to a fresh authoritative snapshot. It never asks the
// coordinator to replay notifications or restart a managed process.
func Stream(ctx context.Context, id string, receive func(Event) error) error {
	if !supervisor.Supported() {
		return core.Failure("unsupported", "agent event transport unavailable")
	}
	if id != "" && !runtimepath.ValidID(id) {
		return core.Failure("invalid_target", "events require a full agent ID")
	}
	lastEpoch := ""
	var lastSequence uint64
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		_, e := Call(ctx, Request{Op: "ping"}, true)
		if e != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if !transientStreamError(e) {
				return e
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(250 * time.Millisecond):
				continue
			}
		}
		r, e := supervisor.Runtime()
		if e != nil {
			return e
		}
		q, e := identify(ctx, Request{Op: "subscribe", ID: id, LastEpoch: lastEpoch, LastSequence: lastSequence})
		if e != nil {
			_ = r.Close()
			return e
		}
		d, path, e := discover(r)
		_ = r.Close()
		if e != nil {
			if !transientStreamError(e) {
				return e
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(250 * time.Millisecond):
				continue
			}
		}
		q.SchemaVersion = 1
		q.Epoch = d.Epoch
		c, e := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", path)
		if e == nil {
			e = func() error {
				defer func() { _ = c.Close() }()
				closeOnCancel := context.AfterFunc(ctx, func() { _ = c.Close() })
				defer closeOnCancel()
				if e := runtimepath.VerifyPeer(c); e != nil {
					return e
				}
				_ = c.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if e := localipc.Write(c, q); e != nil {
					return e
				}
				for {
					_ = c.SetReadDeadline(time.Now().Add(15 * time.Second))
					var reply Reply
					if e := localipc.Read(c, &reply); e != nil {
						return e
					}
					if reply.SchemaVersion != 1 || reply.Epoch != d.Epoch {
						return core.Failure("protocol_error", "event response identity mismatch")
					}
					if reply.Error != nil {
						return reply.Error
					}
					v := reply.Event
					if v == nil || v.SchemaVersion != 1 || v.CoordinatorEpoch != d.Epoch || v.Time.IsZero() || v.Sequence == 0 {
						return core.Failure("protocol_error", "invalid event frame")
					}
					if lastEpoch == v.CoordinatorEpoch && v.Sequence <= lastSequence {
						return core.Failure("protocol_error", "nonmonotonic event sequence")
					}
					if lastEpoch != v.CoordinatorEpoch && v.Kind != "resync_required" {
						return core.Failure("protocol_error", "event epoch requires snapshot")
					}
					lastEpoch = v.CoordinatorEpoch
					lastSequence = v.Sequence
					if e := receive(*v); e != nil {
						return e
					}
				}
			}()
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var known *core.OperationError
		if errors.As(e, &known) {
			return e
		}
		if e != nil && !errors.Is(e, io.EOF) && !errors.Is(e, io.ErrUnexpectedEOF) {
			var transport *net.OpError
			if !errors.As(e, &transport) && !errors.Is(e, net.ErrClosed) {
				return e
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

var errWaitDone = errors.New("wait condition observed")

func Wait(ctx context.Context, id, until string, failOnNonzero bool) (View, error) {
	var latest View
	var unknownSince time.Time
	if until != "exited" && until != "needs-input" && until != "idle" {
		return latest, core.Failure("invalid_usage", "invalid wait condition")
	}
	if failOnNonzero && until != "exited" {
		return latest, core.Failure("invalid_usage", "--fail-on-nonzero requires --until exited")
	}
	evaluate := func(v View) error {
		latest = v
		if until == "exited" && !v.Supervised {
			return core.Failure("unsupported", "unsupervised registrations have no exit receipt")
		}
		if until != "exited" && v.Hooks == nil {
			return core.Failure("unsupported", "attention wait requires configured hooks")
		}
		if until == "exited" && v.Receipt != nil {
			if failOnNonzero && (v.Receipt.ExitCode == nil || *v.Receipt.ExitCode != 0) {
				return core.Failure("operation_failed", "managed process did not exit successfully")
			}
			return errWaitDone
		}
		if until != "exited" && v.Attention == map[string]string{"needs-input": "needs_input", "idle": "idle"}[until] {
			return errWaitDone
		}
		if v.Lifecycle == "exited" {
			return core.Failure("operation_failed", "process exited before the requested attention condition")
		}
		if v.Lifecycle == "unknown" && (v.Supervised || v.Source != "backend observation (unsupervised)") {
			if unknownSince.IsZero() {
				unknownSince = time.Now()
			} else if time.Since(unknownSince) >= time.Second {
				return core.Failure("operation_failed", "agent liveness unknown without an exit receipt")
			}
		} else {
			unknownSince = time.Time{}
		}
		return nil
	}
	err := Stream(ctx, id, func(event Event) error {
		for _, v := range event.Metadata.Agents {
			if v.ID == id {
				if e := evaluate(v); e != nil {
					return e
				}
			}
		}
		if event.Metadata.Agent != nil && event.Metadata.Agent.ID == id {
			return evaluate(*event.Metadata.Agent)
		}
		if event.Kind == "heartbeat" && !unknownSince.IsZero() {
			reply, e := Call(ctx, Request{Op: "status", ID: id}, false)
			if e != nil {
				return e
			}
			if len(reply.Agents) == 1 {
				return evaluate(reply.Agents[0])
			}
		}
		return nil
	})
	if errors.Is(err, errWaitDone) {
		return latest, nil
	}
	return latest, err
}

func transientStreamError(e error) bool {
	var transport *net.OpError
	return errors.Is(e, io.EOF) || errors.Is(e, io.ErrUnexpectedEOF) || errors.Is(e, net.ErrClosed) || os.IsNotExist(e) || errors.As(e, &transport)
}
