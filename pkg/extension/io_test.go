package extension

import (
	"context"
	"github.com/MohamedElashri/txm/pkg/core"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOutputBoundsAndCancellation(t *testing.T) {
	if os.PathSeparator != '/' {
		t.Skip("POSIX fixture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, _, e := Output(ctx, "/bin/sh", "", nil, "-c", "/usr/bin/head -c 1100000 /dev/zero")
	if core.Normalize(e).Code != "protocol_error" {
		t.Fatal(e)
	}
	cancel()
	_, _, e = Output(ctx, "/bin/sleep", "", nil, "10")
	if core.Normalize(e).Code != "cancelled" {
		t.Fatal(e)
	}
}
func TestNoSymlinkAncestors(t *testing.T) {
	if os.PathSeparator != '/' {
		t.Skip("Unix paths")
	}
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	if e := os.Mkdir(real, 0700); e != nil {
		t.Fatal(e)
	}
	link := filepath.Join(dir, "link")
	if e := os.Symlink(real, link); e != nil {
		t.Fatal(e)
	}
	if NoSymlinks(filepath.Join(link, "missing")) == nil {
		t.Fatal("symlink ancestor accepted")
	}
	if e := NoSymlinks(filepath.Join(real, "missing")); e != nil {
		t.Fatal(e)
	}
}
