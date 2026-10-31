//go:build linux || darwin

package runtimepath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateAtomicFilesAndUnsafeObjects(t *testing.T) {
	base, canonicalErr := filepath.EvalSymlinks(t.TempDir())
	if canonicalErr != nil {
		t.Fatal(canonicalErr)
	}
	if e := os.Chmod(base, 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("XDG_STATE_HOME", base)
	r, e := OpenState()
	if e != nil {
		t.Fatal(e)
	}
	defer func(parent *Root) { _ = parent.Close() }(r)
	child, e := r.Child("agents")
	if e != nil {
		t.Fatal(e)
	}
	defer func() { _ = child.Close() }()
	r = child
	if e = r.WritePrivate("value.json", []byte("old"), false); e != nil {
		t.Fatal(e)
	}
	if r.WritePrivate("value.json", []byte("new"), false) == nil {
		t.Fatal("overwrote immutable receipt")
	}
	if e = r.WritePrivate("value.json", []byte("new"), true); e != nil {
		t.Fatal(e)
	}
	if v, e := r.ReadPrivate("value.json", 3); e != nil || string(v) != "new" {
		t.Fatal(string(v), e)
	}
	if _, e = r.ReadPrivate("value.json", 2); e == nil {
		t.Fatal("unbounded private read")
	}
	if _, e = r.Child("../elsewhere"); e == nil {
		t.Fatal("directory traversal accepted")
	}
	for _, name := range []string{"../x", "/x", ".", ".."} {
		if r.WritePrivate(name, nil, true) == nil {
			t.Fatal("file traversal accepted")
		}
	}
	other := filepath.Join(base, "protected")
	if e = os.WriteFile(other, []byte("protected"), 0600); e != nil {
		t.Fatal(e)
	}
	symlink := filepath.Join(r.Path, "link.json")
	if e = os.Symlink(other, symlink); e != nil {
		t.Fatal(e)
	}
	if r.WritePrivate("link.json", []byte("bad"), true) == nil || r.RemovePrivate("link.json") == nil {
		t.Fatal("unsafe symlink replaced/unlinked")
	}
	hard := filepath.Join(r.Path, "hard.json")
	if e = os.Link(other, hard); e != nil {
		t.Fatal(e)
	}
	if _, e = r.ReadPrivate("hard.json", 100); e == nil {
		t.Fatal("hard link accepted")
	}
	if data, _ := os.ReadFile(other); string(data) != "protected" {
		t.Fatal("foreign file modified")
	}
	if e = os.Chmod(filepath.Join(r.Path, "value.json"), 0644); e != nil {
		t.Fatal(e)
	}
	if r.WritePrivate("value.json", []byte("bad"), true) == nil {
		t.Fatal("unsafe mode repaired silently")
	}
}
