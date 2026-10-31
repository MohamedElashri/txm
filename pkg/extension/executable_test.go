//go:build linux || darwin

package extension

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReviewedExecutablePermissionAndHashDrift(t *testing.T) {
	root, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if e = os.Chmod(root, 0700); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(root, "worker")
	if e = os.WriteFile(path, []byte("fixture"), 0700); e != nil {
		t.Fatal(e)
	}
	first, e := ExecutableHash(path)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, []byte("changed"), 0700); e != nil {
		t.Fatal(e)
	}
	next, e := ExecutableHash(path)
	if e != nil || next == first {
		t.Fatal("executable drift not detected", e)
	}
	if e = os.Chmod(path, 0777); e != nil {
		t.Fatal(e)
	}
	if _, e = ExecutableHash(path); e == nil {
		t.Fatal("world-writable executable trusted")
	}
	link := filepath.Join(root, "link")
	if e = os.Symlink(path, link); e != nil {
		t.Fatal(e)
	}
	if _, e = ExecutableHash(link); e == nil {
		t.Fatal("executable symlink trusted")
	}
}
