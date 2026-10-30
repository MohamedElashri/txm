//go:build (linux || darwin) && cgo

package nativeserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	p "github.com/MohamedElashri/txm/pkg/nativeprotocol"
	"go.mitchellh.com/libghostty"
	"net"
	"testing"
	"time"
)

func TestSlowClientQueueCannotBlockAnotherClient(t *testing.T) {
	a, b := net.Pipe()
	defer func() { _ = b.Close() }()
	slow := &client{conn: a, queue: make(chan outbound, p.QueueFrames), done: make(chan struct{})}
	defer slow.close()
	started := time.Now()
	for range p.QueueFrames + 1 {
		slow.enqueue(outbound{frame: p.Frame{Type: p.Data, Payload: []byte{1}}, size: 1})
	}
	select {
	case <-slow.done:
	default:
		t.Fatal("overflow did not disconnect")
	}
	if time.Since(started) > time.Second {
		t.Fatal("fanout blocked on a slow client")
	}
	a2, b2 := net.Pipe()
	defer func() { _ = b2.Close() }()
	fast := &client{conn: a2, queue: make(chan outbound, p.QueueFrames), done: make(chan struct{})}
	defer fast.close()
	if !fast.enqueue(outbound{frame: p.Frame{Type: p.Data, Payload: []byte{2}}, size: 1}) {
		t.Fatal("independent client was blocked")
	}
	bytesClient := &client{conn: a2, queue: make(chan outbound, p.QueueFrames), done: make(chan struct{})}
	for range p.QueueBytes/p.MaxData + 1 {
		bytesClient.enqueue(outbound{frame: p.Frame{Type: p.Data, Payload: make([]byte, p.MaxData)}, size: p.MaxData})
	}
	select {
	case <-bytesClient.done:
	default:
		t.Fatal("byte bound not enforced")
	}
}
func TestBoundedSnapshotAndOperationValidation(t *testing.T) {
	terminal, e := libghostty.NewTerminal(libghostty.WithSize(80, 24))
	if e != nil {
		t.Fatal(e)
	}
	defer terminal.Close()
	if _, e = terminal.Write([]byte("first line\r\nsecond line with enough text to exceed the tiny capture limit")); e != nil {
		t.Fatal(e)
	}
	s := &server{term: terminal}
	emptyTerminal, err := libghostty.NewTerminal(libghostty.WithSize(80, 24))
	if err != nil {
		t.Fatal(err)
	}
	defer emptyTerminal.Close()
	emptySnapshot, err := (&server{term: emptyTerminal}).snapshot(p.Operation{Op: "snapshot", Format: "plain", Lines: 100, MaxBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(emptySnapshot)
	if err != nil || !bytes.Contains(encoded, []byte(`"content":""`)) {
		t.Fatalf("empty snapshot must be base64 string: %s %v", encoded, err)
	}
	snapshot, e := s.snapshot(p.Operation{Op: "snapshot", Format: "plain", Lines: 24, MaxBytes: 8})
	if e != nil || !snapshot.Truncated || len(snapshot.Content) > 8 || snapshot.Lines != 1 {
		t.Fatal("snapshot cap", snapshot, e)
	}
	snapshot, e = s.snapshot(p.Operation{Op: "snapshot", Format: "plain", Lines: 1, MaxBytes: 1024})
	if e != nil || len(snapshot.Content) > 1024 {
		t.Fatal("selection", e)
	}
	for _, raw := range []string{`{"op":"status","data":"eA=="}`, `{"op":"input","data":""}`, `{"op":"snapshot","format":"plain","lines":0,"max_bytes":100}`, `{"op":"kill","format":"vt"}`} {
		var op p.Operation
		if e = json.Unmarshal([]byte(raw), &op); e != nil {
			t.Fatal(e)
		}
		if validateOperation([]byte(raw), op) == nil {
			t.Fatal("invalid operation accepted", raw)
		}
	}
}
func TestInputCancellationBeforeWrite(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := &server{ioCtx: context.Background(), fd: -1}
	if s.writeInput(ctx, []byte("must not write")) == nil {
		t.Fatal("cancelled input wrote")
	}
}

func TestFailedPTYResizeRestoresVTSize(t *testing.T) {
	terminal, err := libghostty.NewTerminal(libghostty.WithSize(80, 24))
	if err != nil {
		t.Fatal(err)
	}
	defer terminal.Close()
	s := &server{term: terminal, fd: -1}
	if err = s.resize(100, 40); err == nil {
		t.Fatal("invalid PTY accepted resize")
	}
	cols, err := terminal.Cols()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := terminal.Rows()
	if err != nil {
		t.Fatal(err)
	}
	if cols != 80 || rows != 24 {
		t.Fatalf("VT diverged from unchanged PTY: %dx%d", cols, rows)
	}
}

func TestHeartbeatAndMissingPongDisconnect(t *testing.T) {
	local, remote := net.Pipe()
	defer func() { _ = remote.Close() }()
	c := &client{conn: local, queue: make(chan outbound, p.QueueFrames), done: make(chan struct{})}
	defer c.close()
	c.attached.Store(true)
	c.activity.Store(time.Now().Add(-p.HeartbeatInterval - time.Second).UnixNano())
	writerDone := make(chan struct{})
	go func() { defer close(writerDone); c.writer() }()
	defer func() { c.close(); <-writerDone }()
	_ = remote.SetReadDeadline(time.Now().Add(3 * time.Second))
	frame, err := p.Read(remote)
	if err != nil || frame.Type != p.Ping || frame.ID == 0 {
		t.Fatal("heartbeat", frame, err)
	}
	c.pingAt.Store(time.Now().Add(-p.HeartbeatTimeout - time.Second).UnixNano())
	select {
	case <-c.done:
	case <-time.After(2 * time.Second):
		t.Fatal("unresponsive subscriber retained")
	}
}

func TestTerminalRepliesDoNotDuplicateControlTerminal(t *testing.T) {
	for _, role := range []string{"", "observer", "control"} {
		t.Run(role, func(t *testing.T) {
			s := &server{ioCtx: context.Background(), inputs: make(chan input, 16), attached: map[*client]bool{}}
			if role != "" {
				s.attached[&client{role: role, done: make(chan struct{})}] = true
			}
			terminal, err := libghostty.NewTerminal(libghostty.WithSize(80, 24), libghostty.WithWritePty(func(_ *libghostty.Terminal, data []byte) { s.terminalReply(data) }))
			if err != nil {
				t.Fatal(err)
			}
			defer terminal.Close()
			if _, err = terminal.Write([]byte("\x1b[6n")); err != nil {
				t.Fatal(err)
			}
			select {
			case reply := <-s.inputs:
				defer reply.cancel()
				if role == "control" || !bytes.Equal(reply.data, []byte("\x1b[1;1R")) {
					t.Fatalf("unexpected generated reply %q for %s", reply.data, role)
				}
			default:
				if role != "control" {
					t.Fatal("detached/observer query got no reply")
				}
			}
		})
	}
}

func TestCaptureAndLockRespectCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	output := &capture{limit: 1024, ctx: ctx}
	if _, err := output.Write([]byte("must not capture")); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled formatter kept writing", err)
	}
	s := &server{}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.lock(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled request remained queued", err)
	}
}
