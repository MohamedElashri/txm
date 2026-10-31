//go:build linux || darwin

package runtimepath

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// ChildExisting validates an already present private directory without creating
// it, fixing its permissions, or acquiring a creating writer lock.
func (r *Root) ChildExisting(name string) (*Root, error) {
	if !privateName().MatchString(name) {
		return nil, fmt.Errorf("invalid private directory name")
	}
	fd, err := unix.Openat(int(r.dir.Fd()), name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), name)
	if err = checkPrivate(f); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &Root{Path: filepath.Join(r.Path, name), dir: f}, nil
}

func existingRoot(base *os.File, path string) (*Root, error) {
	r := &Root{Path: path, dir: base}
	first, err := r.ChildExisting("txm")
	if err != nil {
		return nil, err
	}
	defer func() { _ = first.Close() }()
	return first.ChildExisting("v1")
}

// OpenExistingState has the same identity/mode rules as OpenState but never
// creates a parent, namespace, host identity, lock file, or private child.
func OpenExistingState() (*Root, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		base = filepath.Join(home, ".local", "state")
	}
	if !filepath.IsAbs(base) {
		return nil, fmt.Errorf("state path must be absolute")
	}
	parent, err := openDirectory(base)
	if err != nil {
		return nil, err
	}
	defer func() { _ = parent.Close() }()
	var st unix.Stat_t
	if err = unix.Fstat(int(parent.Fd()), &st); err != nil {
		return nil, err
	}
	if st.Uid != uint32(os.Geteuid()) || st.Mode&0022 != 0 {
		return nil, fmt.Errorf("unsafe state parent")
	}
	return existingRoot(parent, base)
}

func ExistingHostID(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	r, err := OpenExistingState()
	if err != nil {
		return "", err
	}
	defer func() { _ = r.Close() }()
	return readHost(r)
}

// OpenExisting discovers the existing runtime namespace using Open's path
// policy, including the canonical temporary-directory fallback.
func OpenExisting() (*Root, error) {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base != "" {
		if !filepath.IsAbs(base) {
			return nil, fmt.Errorf("XDG_RUNTIME_DIR must be absolute")
		}
		parent, err := openDirectory(base)
		if err != nil {
			return nil, err
		}
		defer func() { _ = parent.Close() }()
		if err = checkPrivate(parent); err != nil {
			return nil, err
		}
		return existingRoot(parent, base)
	}
	if !filepath.IsAbs(os.TempDir()) {
		return nil, fmt.Errorf("native TMPDIR must be absolute")
	}
	temp, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return nil, err
	}
	parent, err := openDirectory(temp)
	if err != nil {
		return nil, err
	}
	defer func() { _ = parent.Close() }()
	r := &Root{Path: temp, dir: parent}
	first, err := r.ChildExisting(fmt.Sprintf("txm-%d", os.Geteuid()))
	if err != nil {
		return nil, err
	}
	defer func() { _ = first.Close() }()
	return first.ChildExisting("v1")
}
