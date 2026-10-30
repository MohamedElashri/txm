package nativeprotocol

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MohamedElashri/txm/pkg/runtimepath"
)

type Client struct {
	Conn     net.Conn
	Identity Identity
	mu       sync.Mutex
	next     atomic.Uint64
}

func Dial(ctx context.Context, path, role string) (*Client, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", path)
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	c := &Client{Conn: conn}
	c.next.Store(1)
	fail := func(err error) (*Client, error) {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		_ = conn.Close()
		return nil, err
	}
	if err = runtimepath.VerifyPeer(conn); err != nil {
		return fail(err)
	}
	deadline := time.Now().Add(RequestTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err = conn.SetDeadline(deadline); err != nil {
		return fail(err)
	}
	f, err := JSON(Hello, 1, Greeting{role})
	if err != nil {
		return fail(err)
	}
	if err = Write(conn, f); err != nil {
		return fail(err)
	}
	f, err = Read(conn)
	if err != nil {
		return fail(err)
	}
	if f.Type == ErrorFrame {
		var e Error
		if err = Decode(f.Payload, &e); err != nil {
			return fail(err)
		}
		return fail(&e)
	}
	if f.Type != HelloOK || f.ID != 1 {
		return fail(Failure("protocol_error", "expected handshake response"))
	}
	if err = Decode(f.Payload, &c.Identity); err != nil {
		return fail(err)
	}
	if c.Identity.ProtocolVersion != Version || c.Identity.Backend != "native" || c.Identity.Role != role || !runtimepath.ValidID(c.Identity.InstanceID) || c.Identity.ProcessState != "running" || c.Identity.ObservedAt.IsZero() {
		return fail(Failure("protocol_error", "invalid server identity/readiness"))
	}
	if err = runtimepath.ValidateName(c.Identity.Name); err != nil {
		return fail(err)
	}
	if err = conn.SetDeadline(time.Time{}); err != nil {
		return fail(err)
	}
	return c, nil
}
func (c *Client) Close() error { return c.Conn.Close() }
func (c *Client) Send(f Frame) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.Conn.SetWriteDeadline(time.Now().Add(WriteTimeout)); err != nil {
		return err
	}
	return Write(c.Conn, f)
}
func (c *Client) NextID() uint64 { return c.next.Add(1) }

// Call is used before attaching. Attached streams have exactly one reader.
func (c *Client) Call(op Operation, value any) error {
	return c.CallContext(context.Background(), op, value)
}
func (c *Client) CallContext(ctx context.Context, op Operation, value any) error {
	stop := context.AfterFunc(ctx, func() { _ = c.Close() })
	defer stop()
	id := c.NextID()
	f, err := JSON(Request, id, op)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(RequestTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err = c.Conn.SetReadDeadline(deadline); err != nil {
		return err
	}
	defer func() { _ = c.Conn.SetReadDeadline(time.Time{}) }()
	if err = c.Send(f); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	for {
		reply, e := Read(c.Conn)
		if e != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return e
		}
		if reply.Type == Ping {
			if e = c.Send(Frame{Type: Pong, ID: reply.ID}); e != nil {
				return e
			}
			continue
		}
		if reply.Type == ErrorFrame {
			var wire Error
			if e = Decode(reply.Payload, &wire); e != nil {
				return e
			}
			return &wire
		}
		if reply.Type != Response || reply.ID != id {
			return Failure("protocol_error", "unexpected control response")
		}
		var data Reply
		if e = Decode(reply.Payload, &data); e != nil {
			return e
		}
		if data.Error != nil {
			return data.Error
		}
		if value != nil {
			if e = Decode(data.Data, value); e != nil {
				return fmt.Errorf("invalid native response: %w", e)
			}
			switch observed := value.(type) {
			case *Snapshot:
				if observed.Type != "screen_snapshot" || observed.Encoding != "base64" || observed.ObservedAt.IsZero() || observed.Content == nil {
					return Failure("protocol_error", "invalid snapshot response")
				}
			case *Status:
				if observed.InstanceID != c.Identity.InstanceID || observed.Name != c.Identity.Name || observed.AttachedClients < 0 || observed.AttachedClients > MaxConnections {
					return Failure("protocol_error", "invalid status identity/count")
				}
				switch observed.ProcessState {
				case "starting", "running", "exited", "unknown":
				default:
					return Failure("protocol_error", "invalid process state")
				}
			}
		}
		return nil
	}
}
