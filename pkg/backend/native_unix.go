//go:build linux || darwin

package backend

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	p "github.com/MohamedElashri/txm/pkg/nativeprotocol"
	"github.com/MohamedElashri/txm/pkg/runtimepath"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

func staleNativeSocket(err error) bool {
	return errors.Is(err, unix.ECONNREFUSED) || errors.Is(err, os.ErrNotExist)
}
func ownedLegacySocket(name string) bool {
	if runtimepath.ValidateName(name) != nil {
		return false
	}
	info, err := os.Lstat(filepath.Join(os.TempDir(), "txm-"+name+".sock"))
	if err != nil || info.Mode()&os.ModeSocket == 0 {
		return false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && st.Uid == uint32(os.Geteuid())
}
func legacyNativeNames() []string {
	dir, err := os.Open(os.TempDir())
	if err != nil {
		return nil
	}
	defer func() { _ = dir.Close() }()
	entries, err := dir.ReadDir(4096)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, "txm-") && strings.HasSuffix(name, ".sock") {
			name = strings.TrimSuffix(strings.TrimPrefix(name, "txm-"), ".sock")
			if ownedLegacySocket(name) {
				names = append(names, name)
			}
		}
	}
	sort.Strings(names)
	return names
}
func startNative(parent context.Context, name, id string, command []string) error {
	return startNativeInDir(parent, name, id, command, "")
}
func startNativeInDir(parent context.Context, name, id string, command []string, dir string) error {
	root, err := runtimepath.Open()
	if err != nil {
		return err
	}
	path, err := root.SocketPath(id)
	_ = root.Close()
	if err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()
	args := append([]string{"server", "--instance", id, "--ready-fd", "3", name}, command...)
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.ExtraFiles = []*os.File{writer}
	if err = cmd.Start(); err != nil {
		_ = writer.Close()
		return err
	}
	_ = writer.Close()
	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(exited) }()
	type startup struct {
		InstanceID string `json:"instance_id"`
		Error      string `json:"error"`
	}
	ready := make(chan error, 1)
	go func() {
		var status startup
		e := json.NewDecoder(io.LimitReader(reader, 4096)).Decode(&status)
		if e == nil && status.Error != "" {
			e = errors.New(status.Error)
		}
		if e == nil && status.InstanceID != id {
			e = fmt.Errorf("native readiness instance mismatch")
		}
		ready <- e
	}()
	ctx, cancel := context.WithTimeout(parent, p.RequestTimeout)
	defer cancel()
	select {
	case err = <-ready:
	case <-ctx.Done():
		err = p.Failure("timeout", "native startup deadline exceeded")
	}
	if err == nil {
		var c *p.Client
		c, err = p.Dial(ctx, path, "observer")
		if err == nil {
			if c.Identity.InstanceID != id || c.Identity.Name != name {
				err = p.Failure("conflict", "native readiness identity mismatch")
			}
			_ = c.Close()
		}
	}
	if err != nil {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-exited:
		case <-time.After(3 * time.Second):
			_ = cmd.Process.Kill()
			<-exited
		}
		return fmt.Errorf("native startup: %w", err)
	}
	return nil
}
func attachNative(c *p.Client, readOnly bool) (retErr error) {
	fd := int(os.Stdin.Fd())
	out := int(os.Stdout.Fd())
	old, err := term.MakeRaw(fd)
	if err != nil {
		return err
	}
	defer func() { _ = term.Restore(fd, old) }()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer cancel()
	defer func() {
		if ctx.Err() != nil {
			retErr = ctx.Err()
		}
	}()
	stop := context.AfterFunc(ctx, func() { _ = c.Close() })
	defer stop()
	// Pollable nonblocking input avoids a goroutine stuck forever in stdin.Read.
	flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
	if err != nil {
		return err
	}
	if err = unix.SetNonblock(fd, true); err != nil {
		return err
	}
	defer func() { _, _ = unix.FcntlInt(uintptr(fd), unix.F_SETFL, flags) }()
	outFlags, err := unix.FcntlInt(uintptr(out), unix.F_GETFL, 0)
	if err != nil {
		return err
	}
	if err = unix.SetNonblock(out, true); err != nil {
		return err
	}
	defer func() { _, _ = unix.FcntlInt(uintptr(out), unix.F_SETFL, outFlags) }()
	var snapshot p.Snapshot
	if err = c.Call(p.Operation{Op: "attach"}, &snapshot); err != nil {
		return err
	}
	writeOutput := func(data []byte) error {
		for len(data) > 0 {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			fds := []unix.PollFd{{Fd: int32(out), Events: unix.POLLOUT}}
			_, e := unix.Poll(fds, 100)
			if e == unix.EINTR {
				continue
			}
			if e != nil {
				return e
			}
			if fds[0].Revents&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 {
				return io.ErrClosedPipe
			}
			if fds[0].Revents&unix.POLLOUT == 0 {
				continue
			}
			n, e := unix.Write(out, data[:min(len(data), 1024)])
			if e == unix.EAGAIN || e == unix.EINTR {
				continue
			}
			if e != nil {
				return e
			}
			if n == 0 {
				return io.ErrShortWrite
			}
			data = data[n:]
		}
		return nil
	}
	if err = writeOutput(snapshot.Content); err != nil {
		return err
	}
	resized := make(chan os.Signal, 1)
	signal.Notify(resized, syscall.SIGWINCH)
	defer signal.Stop(resized)
	resize := func() error {
		if readOnly {
			return nil
		}
		w, h, e := term.GetSize(fd)
		if e != nil {
			return e
		}
		if w <= 0 || h <= 0 {
			return nil
		}
		body := make([]byte, 4)
		binary.BigEndian.PutUint16(body, uint16(w))
		binary.BigEndian.PutUint16(body[2:], uint16(h))
		return c.Send(p.Frame{Type: p.Resize, ID: c.NextID(), Payload: body})
	}
	if err = resize(); err != nil {
		return err
	}
	uc := c.Conn.(*net.UnixConn)
	raw, err := uc.SyscallConn()
	if err != nil {
		return err
	}
	sockFD := 0
	if err = raw.Control(func(fd uintptr) { sockFD = int(fd) }); err != nil {
		return err
	}
	input := make([]byte, p.MaxData)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		select {
		case <-resized:
			if err = resize(); err != nil {
				return err
			}
		default:
		}
		fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}, {Fd: int32(sockFD), Events: unix.POLLIN}}
		if _, err = unix.Poll(fds, 100); err == unix.EINTR {
			continue
		} else if err != nil {
			return err
		}
		if fds[0].Revents&(unix.POLLHUP|unix.POLLERR|unix.POLLNVAL) != 0 {
			return io.EOF
		}
		if fds[0].Revents&unix.POLLIN != 0 {
			n, e := unix.Read(fd, input)
			if e == unix.EAGAIN || e == unix.EINTR {
				continue
			}
			if e != nil {
				return e
			}
			if n == 0 {
				return nil
			}
			end := n
			detach := false
			for i, b := range input[:n] {
				if b == 0x1c {
					end = i
					detach = true
					break
				}
			}
			if !readOnly && end > 0 {
				if e = c.Send(p.Frame{Type: p.Data, Payload: append([]byte{}, input[:end]...)}); e != nil {
					return e
				}
			}
			if detach {
				_ = c.Send(p.Frame{Type: p.Close})
				return nil
			}
		}
		if fds[1].Revents&(unix.POLLIN|unix.POLLHUP|unix.POLLERR|unix.POLLNVAL) != 0 {
			if err = c.Conn.SetReadDeadline(time.Now().Add(p.RequestTimeout)); err != nil {
				return err
			}
			f, e := p.Read(c.Conn)
			if e != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if errors.Is(e, io.EOF) {
					return nil
				}
				return e
			}
			switch f.Type {
			case p.Data:
				if e = writeOutput(f.Payload); e != nil {
					return e
				}
			case p.Ping:
				if e = c.Send(p.Frame{Type: p.Pong, ID: f.ID}); e != nil {
					return e
				}
			case p.Response:
				var reply p.Reply
				if e = p.Decode(f.Payload, &reply); e != nil {
					return e
				}
				if reply.Error != nil {
					return reply.Error
				}
			case p.Close:
				return nil
			case p.ErrorFrame:
				var wire p.Error
				if e = p.Decode(f.Payload, &wire); e != nil {
					return e
				}
				return &wire
			default:
				return p.Failure("protocol_error", "unexpected attached frame")
			}
		}
	}
}
