//go:build linux || darwin

package runtimepath

import (
	"context"
	"fmt"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"testing"
)

func TestReusableNameLocksDoNotExhaustDiscovery(t *testing.T) {
	privateRuntime(t)
	r, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	for i := range 1100 {
		lock, err := r.Lock(fmt.Sprintf("session%d", i))
		if err != nil {
			t.Fatal(err)
		}
		_ = lock.Close()
	}
	names, err := r.Names(context.Background())
	if err != nil || len(names) != 0 {
		t.Fatal("lock history broke discovery", err)
	}
}

func privateRuntime(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if e := os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("XDG_RUNTIME_DIR", dir)
	return dir
}
func TestNameLockAndPrivateDirectories(t *testing.T) {
	privateRuntime(t)
	r, e := Open()
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = r.Close() }()
	lock, e := r.Lock("fixture")
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = lock.Close() }()
	if second, e := r.Lock("fixture"); e == nil {
		_ = second.Close()
		t.Fatal("duplicate lock allowed")
	}
	if _, e = r.Lock("../escape"); e == nil {
		t.Fatal("invalid name accepted")
	}
}
func TestUnsafeRuntimeAndSymlinks(t *testing.T) {
	base := privateRuntime(t)
	if e := os.Chmod(base, 0755); e != nil {
		t.Fatal(e)
	}
	if r, e := Open(); e == nil {
		_ = r.Close()
		t.Fatal("public runtime allowed")
	}
	_ = os.Chmod(base, 0700)
	if e := os.Symlink(t.TempDir(), filepath.Join(base, "txm")); e != nil {
		t.Fatal(e)
	}
	if r, e := Open(); e == nil {
		_ = r.Close()
		t.Fatal("symlink runtime allowed")
	}
}
func TestFallbackAndSocketLength(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	temp := t.TempDir()
	t.Setenv("TMPDIR", temp)
	r, e := Open()
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = r.Close() }()
	if filepath.Base(filepath.Dir(r.Path)) == "txm" {
		t.Fatal("unexpected extra fallback path component")
	}
	id, e := NewID()
	if e != nil || !ValidID(id) {
		t.Fatal(e)
	}
	r.Path = string(make([]byte, 104))
	if _, e = r.SocketPath(id); e == nil {
		t.Fatal("long socket path accepted")
	}
}

func TestLogPermissionsSymlinkAndFIFO(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "output.log")
	file, err := OpenLog(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("private log mode", err)
	}
	_ = os.Chmod(path, 0644)
	if f, e := OpenLog(path); e == nil {
		_ = f.Close()
		t.Fatal("public log accepted")
	}
	linked := filepath.Join(root, "linked")
	if err = os.Symlink(path, linked); err != nil {
		t.Fatal(err)
	}
	if f, e := OpenLog(linked); e == nil {
		_ = f.Close()
		t.Fatal("log symlink followed")
	}
	fifo := filepath.Join(root, "fifo")
	if err = unix.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	if f, e := OpenLog(fifo); e == nil {
		_ = f.Close()
		t.Fatal("FIFO log accepted")
	}
}

func TestRelativeTemporaryRuntimeIsRejected(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("TMPDIR", "relative")
	if r, e := Open(); e == nil {
		_ = r.Close()
		t.Fatal("relative temporary runtime accepted")
	}
}
