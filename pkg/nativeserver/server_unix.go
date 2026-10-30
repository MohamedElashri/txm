//go:build (linux || darwin) && cgo

package nativeserver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MohamedElashri/txm/pkg/nativeprocess"
	p "github.com/MohamedElashri/txm/pkg/nativeprotocol"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"github.com/creack/pty"
	"go.mitchellh.com/libghostty"
	"golang.org/x/sys/unix"
)

type server struct {
	opts      Options
	term      *libghostty.Terminal
	pty       *os.File
	fd        int
	mu        sync.Mutex // VT, position and attachment membership; never held on socket I/O.
	attached  map[*client]bool
	position  uint64
	clientsMu sync.Mutex
	clients   map[*client]bool
	workers   sync.WaitGroup
	stop      chan struct{}
	fatal     chan error
	stopOnce  sync.Once
	ioCtx     context.Context
	inputs    chan input
	childDone chan struct{}
	childPID  int
	childErr  error // published by close(childDone), before reaping
}
type input struct {
	ctx    context.Context
	data   []byte
	result chan error
	cancel context.CancelFunc
}
type outbound struct {
	frame p.Frame
	ack   chan error
	size  int
}
type client struct {
	s        *server
	conn     net.Conn
	queue    chan outbound
	done     chan struct{}
	once     sync.Once
	bytes    atomic.Int64
	attached atomic.Bool
	activity atomic.Int64
	awaiting atomic.Uint64
	pingAt   atomic.Int64
	role     string
	lastID   uint64
}

func (s *server) requestStop() { s.stopOnce.Do(func() { close(s.stop) }) }
func (c *client) close()       { c.once.Do(func() { close(c.done); _ = c.conn.Close() }) }

func Serve(ctx context.Context, opts Options) (retErr error) {
	notified := false
	defer func() {
		if !notified && opts.Ready != nil {
			_ = opts.Ready(retErr)
		}
	}()
	if err := runtimepath.ValidateName(opts.Name); err != nil {
		return err
	}
	if opts.InstanceID == "" {
		var err error
		opts.InstanceID, err = runtimepath.NewID()
		if err != nil {
			return err
		}
	}
	root, err := runtimepath.Open()
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	lock, err := root.Lock(opts.Name)
	if err != nil {
		return err
	}
	defer func() { _ = lock.Close() }()
	path, err := root.SocketPath(opts.InstanceID)
	if err != nil {
		return err
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return err
	}
	listener.SetUnlinkOnClose(false)
	defer func() { _ = listener.Close() }()
	if err = os.Chmod(path, 0600); err != nil {
		return err
	}
	original, err := root.SocketInfo(opts.InstanceID)
	if err != nil {
		return err
	}
	defer func() {
		if e := root.RemoveSocket(opts.InstanceID, original); e != nil {
			retErr = errors.Join(retErr, e)
		}
	}()
	if opts.ScrollbackLines <= 0 || opts.ScrollbackLines > 1_000_000 {
		return fmt.Errorf("native scrollback_size must be 1..1000000")
	}
	var engine *server
	term, err := libghostty.NewTerminal(libghostty.WithWritePty(func(_ *libghostty.Terminal, data []byte) {
		if engine != nil {
			engine.terminalReply(data)
		}
	}), libghostty.WithSize(80, 24), libghostty.WithMaxScrollbackLines(uint(opts.ScrollbackLines)))
	if err != nil {
		return err
	}
	defer term.Close()
	argv := opts.Command
	if len(argv) == 0 {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
		argv = []string{shell}
	}
	child := exec.Command(argv[0], argv[1:]...)
	child.Env = os.Environ()
	master, err := pty.StartWithSize(child, &pty.Winsize{Rows: 24, Cols: 80})
	if err != nil {
		return fmt.Errorf("native child start: %w", err)
	}
	defer func() { _ = master.Close() }()
	// Do not reap before the last group signal: a zombie leader reserves its PGID.
	childDone := make(chan struct{})
	ioCtx, cancelIO := context.WithCancel(context.Background())
	defer cancelIO()
	s := &server{opts: opts, term: term, pty: master, attached: map[*client]bool{}, clients: map[*client]bool{}, stop: make(chan struct{}), fatal: make(chan error, 1), ioCtx: ioCtx, inputs: make(chan input, 16), childDone: childDone, childPID: child.Process.Pid}
	go func() { s.childErr = nativeprocess.WaitExited(child.Process.Pid); close(childDone) }()
	engine = s
	s.fd = int(master.Fd())
	ioReady := unix.SetNonblock(s.fd, true)
	outputDone, inputDone := make(chan struct{}), make(chan struct{})
	go func() { defer close(outputDone); s.output() }()
	go func() { defer close(inputDone); s.inputLoop() }()
	acceptDone := make(chan struct{})
	go func() { defer close(acceptDone); s.accept(listener) }()
	if ioReady != nil {
		retErr = ioReady
		s.requestStop()
	}
	exited, observeErr := nativeprocess.Exited(child.Process.Pid)
	if observeErr != nil || exited {
		retErr = errors.Join(retErr, observeErr, errors.New("native child exited or could not be observed before readiness"))
		s.requestStop()
	}
	notified = true
	if opts.Ready != nil {
		if e := opts.Ready(retErr); e != nil {
			retErr = errors.Join(retErr, e)
			s.requestStop()
		}
	}
	select {
	case <-ctx.Done():
	case <-s.stop:
	case <-childDone:
	case <-outputDone:
	}
	_ = listener.Close()
	<-acceptDone
	// Waitid errors mean ownership cannot be established; do not guess at a group.
	safeToSignal := true
	select {
	case <-childDone:
		if s.childErr != nil {
			retErr = errors.Join(retErr, s.childErr)
			safeToSignal = false
		}
	default:
	}
	if safeToSignal {
		if e := unix.Kill(-child.Process.Pid, unix.SIGTERM); e != nil && e != unix.ESRCH {
			retErr = errors.Join(retErr, e)
		}
		timer := time.NewTimer(2 * time.Second)
		<-timer.C
		if e := unix.Kill(-child.Process.Pid, unix.SIGKILL); e != nil && e != unix.ESRCH {
			retErr = errors.Join(retErr, e)
		}
	}
	<-childDone
	_ = child.Wait() // Reap after all group signals; ordinary child exit isn't CLI failure.
	cancelIO()
	s.clientsMu.Lock()
	for c := range s.clients {
		c.close()
	}
	s.clientsMu.Unlock()
	s.workers.Wait()
	<-outputDone
	<-inputDone
	// Numeric fds stay live until all raw I/O and request workers are joined.
	_ = master.Close()
	select {
	case failure := <-s.fatal:
		retErr = errors.Join(retErr, failure)
	default:
	}
	return retErr
}
func (s *server) accept(listener *net.UnixListener) {
	slots := make(chan struct{}, p.MaxConnections)
	for {
		conn, err := listener.AcceptUnix()
		if err != nil {
			return
		}
		select {
		case slots <- struct{}{}:
		default:
			_ = conn.Close()
			continue
		}
		c := &client{s: s, conn: conn, queue: make(chan outbound, p.QueueFrames), done: make(chan struct{})}
		s.clientsMu.Lock()
		s.clients[c] = true
		s.clientsMu.Unlock()
		s.workers.Add(1)
		go func() {
			defer s.workers.Done()
			defer func() { <-slots; s.clientsMu.Lock(); delete(s.clients, c); s.clientsMu.Unlock() }()
			c.run()
		}()
	}
}
func (s *server) output() {
	buf := make([]byte, p.MaxData)
	for {
		if err := poll(s.ioCtx, s.fd, unix.POLLIN, time.Time{}); err != nil {
			return
		}
		n, err := unix.Read(s.fd, buf)
		if err == unix.EAGAIN || err == unix.EINTR {
			continue
		}
		if n > 0 {
			data := append([]byte{}, buf[:n]...)
			s.mu.Lock()
			_, writeErr := s.term.Write(data)
			s.position++
			if writeErr == nil {
				for c := range s.attached {
					c.enqueue(outbound{frame: p.Frame{Type: p.Data, Payload: data}, size: n})
				}
			}
			s.mu.Unlock()
			if writeErr != nil {
				s.abort(writeErr)
				return
			}
			if s.opts.Log != nil {
				if _, e := s.opts.Log.Write(data); e != nil {
					s.abort(fmt.Errorf("native output log: %w", e))
					return
				}
			}
		}
		if err != nil || n == 0 {
			return
		}
	}
}
func poll(ctx context.Context, fd int, event int16, deadline time.Time) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !deadline.IsZero() && time.Now().After(deadline) {
			return context.DeadlineExceeded
		}
		fds := []unix.PollFd{{Fd: int32(fd), Events: event}}
		_, err := unix.Poll(fds, 100)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return err
		}
		if fds[0].Revents&unix.POLLNVAL != 0 {
			return os.ErrClosed
		}
		if fds[0].Revents&(event|unix.POLLHUP|unix.POLLERR) != 0 {
			return nil
		}
	}
}
func (s *server) inputLoop() {
	for {
		select {
		case <-s.ioCtx.Done():
			return
		case in := <-s.inputs:
			err := s.writeInput(in.ctx, in.data)
			if in.cancel != nil {
				in.cancel()
			}
			if in.result != nil {
				in.result <- err
			}
		}
	}
}
func (s *server) writeInput(ctx context.Context, b []byte) error {
	for len(b) > 0 {
		select {
		case <-s.ioCtx.Done():
			return context.Canceled
		default:
		}
		if err := poll(ctx, s.fd, unix.POLLOUT, time.Now().Add(p.RequestTimeout)); err != nil {
			return err
		}
		n, err := unix.Write(s.fd, b)
		if err == unix.EAGAIN || err == unix.EINTR {
			continue
		}
		if err != nil {
			return err
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}
func (s *server) input(ctx context.Context, data []byte, ack bool) error {
	item := input{ctx: ctx, data: append([]byte{}, data...)}
	if ack {
		item.result = make(chan error, 1)
	}
	select {
	case s.inputs <- item:
	default:
		return p.Failure("limit_exceeded", "PTY input queue full")
	}
	if !ack {
		return nil
	}
	select {
	case err := <-item.result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-s.ioCtx.Done():
		return context.Canceled
	}
}
func (c *client) enqueue(out outbound) bool {
	if out.size > 0 && c.bytes.Add(int64(out.size)) > p.QueueBytes {
		c.bytes.Add(-int64(out.size))
		c.close()
		return false
	}
	select {
	case <-c.done:
		if out.size > 0 {
			c.bytes.Add(-int64(out.size))
		}
		return false
	default:
	}
	select {
	case c.queue <- out:
		return true
	default:
		if out.size > 0 {
			c.bytes.Add(-int64(out.size))
		}
		c.close()
		return false
	}
}
func (c *client) send(f p.Frame) error {
	ack := make(chan error, 1)
	if !c.enqueue(outbound{frame: f, ack: ack}) {
		return io.ErrClosedPipe
	}
	select {
	case e := <-ack:
		return e
	case <-c.done:
		return io.ErrClosedPipe
	}
}
func (c *client) reply(id uint64, value any, err error) error {
	var wire *p.Error
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			wire = &p.Error{Code: "timeout", Message: "native operation deadline exceeded"}
		} else if !errors.As(err, &wire) {
			wire = &p.Error{Code: "internal", Message: "native operation failed"}
		}
	}
	data, e := json.Marshal(value)
	if e != nil {
		return e
	}
	frame, e := p.JSON(p.Response, id, p.Reply{Data: data, Error: wire})
	if e != nil {
		return e
	}
	return c.send(frame)
}
func (c *client) fail(id uint64, err error) {
	var wire *p.Error
	if !errors.As(err, &wire) {
		wire = &p.Error{Code: "protocol_error", Message: "invalid or incomplete frame"}
	}
	f, e := p.JSON(p.ErrorFrame, id, wire)
	if e == nil {
		_ = c.send(f)
	}
}
func (c *client) writer() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var seq uint64
	write := func(f p.Frame) error {
		if err := c.conn.SetWriteDeadline(time.Now().Add(p.WriteTimeout)); err != nil {
			return err
		}
		return p.Write(c.conn, f)
	}
	for {
		select {
		case <-c.done:
			return
		case out := <-c.queue:
			if out.size > 0 {
				c.bytes.Add(-int64(out.size))
			}
			err := write(out.frame)
			if out.ack != nil {
				out.ack <- err
			}
			if err != nil {
				c.close()
				return
			}
			c.activity.Store(time.Now().UnixNano())
		case <-ticker.C:
			if !c.attached.Load() {
				continue
			}
			if c.awaiting.Load() != 0 {
				if time.Since(time.Unix(0, c.pingAt.Load())) > p.HeartbeatTimeout {
					c.close()
					return
				}
				continue
			}
			if time.Since(time.Unix(0, c.activity.Load())) >= p.HeartbeatInterval {
				seq++
				id := (uint64(1) << 63) | seq
				c.awaiting.Store(id)
				c.pingAt.Store(time.Now().UnixNano())
				if err := write(p.Frame{Type: p.Ping, ID: id}); err != nil {
					c.close()
					return
				}
			}
		}
	}
}
func (c *client) run() {
	defer c.close()
	defer func() { c.s.mu.Lock(); delete(c.s.attached, c); c.s.mu.Unlock() }()
	if err := runtimepath.VerifyPeer(c.conn); err != nil {
		return
	}
	writerDone := make(chan struct{})
	go func() { defer close(writerDone); c.writer() }()
	defer func() { c.close(); <-writerDone }()
	if err := c.conn.SetReadDeadline(time.Now().Add(p.RequestTimeout)); err != nil {
		return
	}
	reader := bufio.NewReader(c.conn)
	f, err := p.Read(reader)
	if err != nil {
		c.fail(0, err)
		return
	}
	var greeting p.Greeting
	if f.Type != p.Hello || p.Decode(f.Payload, &greeting) != nil || (greeting.Role != "observer" && greeting.Role != "control") {
		c.fail(1, p.Failure("protocol_error", "expected valid HELLO"))
		return
	}
	c.role = greeting.Role
	c.lastID = 1
	c.activity.Store(time.Now().UnixNano())
	exited, err := nativeprocess.Exited(c.s.childPID)
	if err != nil {
		c.fail(1, p.Failure("internal", "native child observation failed"))
		return
	}
	if exited {
		c.fail(1, p.Failure("not_found", "native child already exited"))
		return
	}
	caps := []string{"status", "snapshot", "attach"}
	if c.role == "control" {
		caps = append(caps, "input", "resize", "kill")
	}
	f, err = p.JSON(p.HelloOK, 1, p.Identity{ProtocolVersion: p.Version, InstanceID: c.s.opts.InstanceID, Backend: "native", Name: c.s.opts.Name, Role: c.role, ProcessState: "running", Capabilities: caps, ObservedAt: time.Now().UTC()})
	if err != nil || c.send(f) != nil {
		return
	}
	for {
		deadline := time.Time{}
		if !c.attached.Load() {
			deadline = time.Now().Add(p.RequestTimeout)
		}
		if err = c.conn.SetReadDeadline(deadline); err != nil {
			return
		}
		if _, err = reader.Peek(1); err != nil {
			return
		}
		if err = c.conn.SetReadDeadline(time.Now().Add(p.RequestTimeout)); err != nil {
			return
		}
		f, err = p.Read(reader)
		if err != nil {
			c.fail(0, err)
			return
		}
		c.activity.Store(time.Now().UnixNano())
		switch f.Type {
		case p.Close:
			return
		case p.Pong:
			if c.awaiting.Load() != f.ID {
				c.fail(f.ID, p.Failure("protocol_error", "unexpected PONG"))
				return
			}
			c.awaiting.Store(0)
			continue
		case p.Ping:
			if c.send(p.Frame{Type: p.Pong, ID: f.ID}) != nil {
				return
			}
			continue
		case p.Data:
			if !c.attached.Load() || c.role != "control" {
				c.fail(0, p.Failure("denied", "input requires control attach"))
				continue
			}
			ctx, cancel := context.WithTimeout(c.s.ioCtx, p.RequestTimeout)
			err = c.s.input(ctx, f.Payload, true)
			cancel()
			if err != nil {
				c.fail(0, err)
				return
			}
			continue
		case p.Request, p.Resize:
			if f.ID <= c.lastID || f.ID >= uint64(1)<<63 {
				c.fail(f.ID, p.Failure("protocol_error", "request IDs must increase"))
				return
			}
			c.lastID = f.ID
		default:
			c.fail(f.ID, p.Failure("protocol_error", "unexpected client frame"))
			return
		}
		if f.Type == p.Resize {
			if c.role != "control" {
				if c.reply(f.ID, nil, p.Failure("denied", "observer resize denied")) != nil {
					return
				}
				continue
			}
			cols, rows := binary.BigEndian.Uint16(f.Payload[:2]), binary.BigEndian.Uint16(f.Payload[2:])
			if cols == 0 || rows == 0 || cols > 4096 || rows > 4096 || uint32(cols)*uint32(rows) > 1_048_576 {
				c.fail(f.ID, p.Failure("protocol_error", "invalid terminal dimensions"))
				return
			}
			opCtx, cancel := context.WithTimeout(c.s.ioCtx, p.RequestTimeout)
			err = c.s.resize(cols, rows, opCtx)
			cancel()
			if c.reply(f.ID, nil, err) != nil {
				return
			}
			continue
		}
		var op p.Operation
		if err = p.Decode(f.Payload, &op); err != nil {
			c.fail(f.ID, err)
			return
		}
		if err = validateOperation(f.Payload, op); err != nil {
			c.fail(f.ID, err)
			return
		}
		if (op.Op == "kill" || op.Op == "input") && c.role != "control" {
			if c.reply(f.ID, nil, p.Failure("denied", "observer mutation denied")) != nil {
				return
			}
			continue
		}
		switch op.Op {
		case "status":
			c.s.mu.Lock()
			count := len(c.s.attached)
			c.s.mu.Unlock()
			state := "running"
			exited, observeErr := nativeprocess.Exited(c.s.childPID)
			if observeErr != nil {
				state = "unknown"
			} else if exited {
				state = "exited"
			}
			if c.reply(f.ID, p.Status{InstanceID: c.s.opts.InstanceID, Name: c.s.opts.Name, ProcessState: state, AttachedClients: count}, nil) != nil {
				return
			}
		case "snapshot", "attach":
			if op.Op == "attach" && c.attached.Load() {
				if c.reply(f.ID, nil, p.Failure("conflict", "already attached")) != nil {
					return
				}
				continue
			}
			opCtx, cancel := context.WithTimeout(c.s.ioCtx, p.RequestTimeout)
			if e := c.s.lock(opCtx); e != nil {
				cancel()
				if c.reply(f.ID, nil, e) != nil {
					return
				}
				continue
			}
			snapshot, e := c.s.snapshot(op, opCtx)
			cancel()
			if e == nil && op.Op == "attach" && snapshot.Truncated {
				e = p.Failure("limit_exceeded", "initial terminal snapshot too large")
			}
			if e != nil {
				c.s.mu.Unlock()
				if c.reply(f.ID, nil, e) != nil {
					return
				}
				continue
			}
			data, e := json.Marshal(snapshot)
			if e != nil {
				c.s.mu.Unlock()
				return
			}
			frame, e := p.JSON(p.Response, f.ID, p.Reply{Data: data})
			if e != nil {
				c.s.mu.Unlock()
				return
			}
			ack := make(chan error, 1)
			ok := c.enqueue(outbound{frame: frame, ack: ack})
			if ok && op.Op == "attach" {
				c.attached.Store(true)
				c.s.attached[c] = true
			}
			c.s.mu.Unlock()
			if !ok {
				return
			}
			select {
			case e = <-ack:
				if e != nil {
					return
				}
			case <-c.done:
				return
			}
		case "input":
			ctx, cancel := context.WithTimeout(c.s.ioCtx, p.RequestTimeout)
			err = c.s.input(ctx, op.Data, true)
			cancel()
			if c.reply(f.ID, nil, err) != nil {
				return
			}
		case "kill":
			if c.reply(f.ID, nil, nil) == nil {
				c.s.requestStop()
			}
			return
		}
	}
}
func validateOperation(raw []byte, op p.Operation) error {
	allowed := map[string]bool{"op": true}
	switch op.Op {
	case "status", "attach", "kill":
	case "snapshot":
		allowed["format"] = true
		allowed["lines"] = true
		allowed["max_bytes"] = true
	case "input":
		allowed["data"] = true
		if len(op.Data) == 0 || len(op.Data) > p.MaxData {
			return p.Failure("protocol_error", "invalid input length")
		}
	default:
		return p.Failure("protocol_error", "unknown native operation")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	for k := range fields {
		if !allowed[k] {
			return p.Failure("protocol_error", "unexpected operation field")
		}
	}
	if op.Op == "snapshot" && ((op.Format != "plain" && op.Format != "vt") || op.Lines < 1 || op.Lines > 10000 || op.MaxBytes < 1 || op.MaxBytes > p.MaxPayload) {
		return p.Failure("protocol_error", "invalid snapshot bounds/format")
	}
	return nil
}

var errCaptureLimit = errors.New("snapshot limit")

type capture struct {
	bytes.Buffer
	limit int
	ctx   context.Context
}

func (b *capture) Write(data []byte) (int, error) {
	if b.ctx != nil && b.ctx.Err() != nil {
		return 0, b.ctx.Err()
	}
	n := min(len(data), b.limit-b.Len())
	_, _ = b.Buffer.Write(data[:n])
	if n < len(data) {
		return n, errCaptureLimit
	}
	return n, nil
}

// snapshot is called only while holding the VT lock; WriteTo stops at the cap.
func (s *server) snapshot(op p.Operation, budget ...context.Context) (p.Snapshot, error) {
	lineTruncated := false
	limit := (p.MaxPayload - 4096) * 3 / 4
	format := libghostty.FormatterFormatVT
	options := []libghostty.FormatterOption{libghostty.WithFormatterFormat(format)}
	if op.Op == "snapshot" {
		limit = min(limit, op.MaxBytes)
		if op.Format == "plain" {
			format = libghostty.FormatterFormatPlain
			options[0] = libghostty.WithFormatterFormat(format)
		}
		rows, e := s.term.Rows()
		if e != nil {
			return p.Snapshot{}, e
		}
		cols, e := s.term.Cols()
		if e != nil {
			return p.Snapshot{}, e
		}
		if op.Lines < int(rows) {
			lineTruncated = true
			start, e := s.term.GridRef(libghostty.Point{Tag: libghostty.PointTagActive, Y: uint32(int(rows) - op.Lines)})
			if e != nil {
				return p.Snapshot{}, e
			}
			end, e := s.term.GridRef(libghostty.Point{Tag: libghostty.PointTagActive, X: cols - 1, Y: uint32(rows - 1)})
			if e != nil {
				return p.Snapshot{}, e
			}
			options = append(options, libghostty.WithFormatterSelection(&libghostty.Selection{Start: *start, End: *end}))
		}
	} else {
		options = append(options, libghostty.WithFormatterExtraCursor(true), libghostty.WithFormatterExtraModes(true))
	}
	formatter, err := libghostty.NewFormatter(s.term, options...)
	if err != nil {
		return p.Snapshot{}, err
	}
	defer formatter.Close()
	output := &capture{limit: limit}
	if len(budget) > 0 {
		output.ctx = budget[0]
	}
	_, err = formatter.WriteTo(output)
	if err != nil && !errors.Is(err, errCaptureLimit) {
		return p.Snapshot{}, err
	}
	content := append([]byte{}, output.Bytes()...)
	lines := bytes.Count(content, []byte{'\n'})
	if len(content) > 0 && content[len(content)-1] != '\n' {
		lines++
	}
	return p.Snapshot{Type: "screen_snapshot", Encoding: "base64", Content: content, Lines: lines, Truncated: lineTruncated || errors.Is(err, errCaptureLimit), ObservedAt: time.Now().UTC(), Position: s.position}, nil
}

// Terminal query replies must never block the VT output reader.
func (s *server) terminalReply(data []byte) {
	// Called by term.Write while the VT/attachment lock is held. A live control
	// terminal receives raw query bytes and supplies its own reply; do not also
	// inject the emulator's reply. Observers discard input, so detached and
	// observer-only sessions use the emulator response.
	for c := range s.attached {
		if c.role == "control" {
			select {
			case <-c.done:
			default:
				return
			}
		}
	}
	if len(data) == 0 {
		return
	}
	if len(data) > p.MaxData {
		s.requestStop()
		return
	}
	ctx, cancel := context.WithTimeout(s.ioCtx, p.RequestTimeout)
	// A callback item owns its cancel until consumed by the input worker.
	item := input{ctx: ctx, data: append([]byte{}, data...), cancel: cancel}
	select {
	case s.inputs <- item:
	default:
		cancel()
		s.requestStop()
	}
}

func (s *server) resize(cols, rows uint16, budget ...context.Context) error {
	ctx := s.ioCtx
	if ctx == nil {
		ctx = context.Background()
	}
	if len(budget) > 0 {
		ctx = budget[0]
	}
	if err := s.lock(ctx); err != nil {
		return err
	}
	defer s.mu.Unlock()
	oldCols, err := s.term.Cols()
	if err != nil {
		return err
	}
	oldRows, err := s.term.Rows()
	if err != nil {
		return err
	}
	if err = s.term.Resize(cols, rows, 0, 0); err != nil {
		return err
	}
	if err = unix.IoctlSetWinsize(s.fd, unix.TIOCSWINSZ, &unix.Winsize{Row: rows, Col: cols}); err != nil {
		if rollback := s.term.Resize(oldCols, oldRows, 0, 0); rollback != nil {
			s.requestStop()
			return errors.Join(err, rollback)
		}
		return err
	}
	return nil
}

func (s *server) abort(err error) {
	select {
	case s.fatal <- err:
	default:
	}
	s.requestStop()
}

// A request must not spend an unbounded lifetime queued behind VT work.
func (s *server) lock(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.mu.TryLock() {
		return nil
	}
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if s.mu.TryLock() {
				return nil
			}
		}
	}
}
