//go:build linux || darwin

package runtimepath

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestConcurrentHostIdentityAndCorruption(t *testing.T) {
	root, canonicalErr := filepath.EvalSymlinks(t.TempDir())
	if canonicalErr != nil {
		t.Fatal(canonicalErr)
	}
	if e := os.Chmod(root, 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("XDG_STATE_HOME", root)
	ids := make(chan string, 16)
	failures := make(chan error, 16)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			id, e := HostID(ctx)
			if e != nil {
				failures <- e
			} else {
				ids <- id
			}
		}()
	}
	wg.Wait()
	close(ids)
	close(failures)
	for e := range failures {
		t.Fatal(e)
	}
	want := ""
	for id := range ids {
		if !ValidID(id) || (want != "" && id != want) {
			t.Fatalf("identities diverged %s != %s", id, want)
		}
		want = id
	}
	path := filepath.Join(root, "txm", "v1", "host.json")
	info, e := os.Stat(path)
	if e != nil || info.Mode().Perm() != 0600 {
		t.Fatal("unsafe identity permissions", e)
	}
	for _, bad := range []string{`{`, `{"schema_version":1,"host_id":"bad"}`, `{"schema_version":2,"host_id":"0123456789abcdef0123456789abcdef"}`, `{"schema_version":1,"host_id":"0123456789abcdef0123456789abcdef","host_id":"0123456789abcdef0123456789abcdef"}`} {
		if e = os.WriteFile(path, []byte(bad), 0600); e != nil {
			t.Fatal(e)
		}
		if _, e = HostID(context.Background()); e == nil {
			t.Fatal("corrupt identity silently replaced")
		}
		got, _ := os.ReadFile(path)
		if string(got) != bad {
			t.Fatal("corrupt evidence overwritten")
		}
	}
}
func TestHostIdentityRefusesSymlinksAndPublicFiles(t *testing.T) {
	root, canonicalErr := filepath.EvalSymlinks(t.TempDir())
	if canonicalErr != nil {
		t.Fatal(canonicalErr)
	}
	if e := os.Chmod(root, 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("XDG_STATE_HOME", root)
	if _, e := HostID(context.Background()); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(root, "txm", "v1", "host.json")
	if e := os.Chmod(path, 0644); e != nil {
		t.Fatal(e)
	}
	if _, e := HostID(context.Background()); e == nil {
		t.Fatal("public identity accepted")
	}
	target := filepath.Join(root, "untouched")
	_ = os.WriteFile(target, []byte("sentinel"), 0600)
	_ = os.Remove(path)
	if e := os.Symlink(target, path); e != nil {
		t.Fatal(e)
	}
	if _, e := HostID(context.Background()); e == nil {
		t.Fatal("identity symlink followed")
	}
	data, _ := os.ReadFile(target)
	if string(data) != "sentinel" {
		t.Fatal("symlink target changed")
	}
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "alias"))
	if e := os.Symlink(filepath.Join(root, "txm"), filepath.Join(root, "alias")); e != nil {
		t.Fatal(e)
	}
	if _, e := HostID(context.Background()); e == nil {
		t.Fatal("symlink ancestor followed")
	}
}
