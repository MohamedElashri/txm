//go:build linux || darwin

package runtimepath

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestExistingAccessDoesNotCreateRepairOrLock(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(base, 0700); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(base, "absent")
	t.Setenv("XDG_STATE_HOME", missing)
	t.Setenv("XDG_RUNTIME_DIR", missing)
	if _, err = OpenExistingState(); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if _, err = OpenExisting(); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if _, err = ExistingHostID(context.Background()); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if _, err = os.Lstat(missing); !os.IsNotExist(err) {
		t.Fatal("read created root")
	}
	t.Setenv("XDG_STATE_HOME", base)
	id, err := HostID(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	r, err := OpenExistingState()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	before, err := os.ReadDir(r.Path)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := ExistingHostID(context.Background()); err != nil || got != id {
		t.Fatal(got, err)
	}
	if _, err = r.ChildExisting("absent"); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	after, _ := os.ReadDir(r.Path)
	if len(after) != len(before) {
		t.Fatal("read created child/lock")
	}
	child := filepath.Join(r.Path, "unsafe")
	if err = os.Mkdir(child, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err = r.ChildExisting("unsafe"); err == nil {
		t.Fatal("unsafe mode accepted")
	}
	info, _ := os.Stat(child)
	if info.Mode().Perm() != 0755 {
		t.Fatal("read repaired permissions")
	}
	if err = os.Symlink(child, filepath.Join(r.Path, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err = r.ChildExisting("link"); err == nil {
		t.Fatal("symlink accepted")
	}
}
