//go:build linux || darwin

package runtimepath

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"golang.org/x/sys/unix"
)

var privateName = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,100}$`) })

// OpenState uses the same private namespace and checks as host identity.
func OpenState() (*Root, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, e := os.UserHomeDir()
		if e != nil {
			return nil, e
		}
		base = filepath.Join(home, ".local", "state")
	}
	if !filepath.IsAbs(base) {
		return nil, fmt.Errorf("state path must be absolute")
	}
	parent, e := stateDirectory(base)
	if e != nil {
		return nil, e
	}
	defer func() { _ = parent.Close() }()
	var st unix.Stat_t
	if e = unix.Fstat(int(parent.Fd()), &st); e != nil {
		return nil, e
	}
	if st.Uid != uint32(os.Geteuid()) || st.Mode&0022 != 0 {
		return nil, fmt.Errorf("unsafe state parent")
	}
	return privateRoot(parent, base)
}
func (r *Root) Child(name string) (*Root, error) {
	if !privateName().MatchString(name) {
		return nil, fmt.Errorf("invalid private directory name")
	}
	f, e := privateChild(r.dir, name)
	if e != nil {
		return nil, e
	}
	return &Root{Path: filepath.Join(r.Path, name), dir: f}, nil
}
func (r *Root) openPrivate(name string) (*os.File, error) {
	if !privateName().MatchString(name) {
		return nil, fmt.Errorf("invalid private file name")
	}
	fd, e := unix.Openat(int(r.dir.Fd()), name, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(fd), name)
	var st unix.Stat_t
	if e = unix.Fstat(fd, &st); e == nil && (st.Uid != uint32(os.Geteuid()) || st.Mode&0777 != 0600 || st.Mode&unix.S_IFMT != unix.S_IFREG || st.Nlink != 1) {
		e = fmt.Errorf("unsafe private file")
	}
	if e != nil {
		_ = f.Close()
		return nil, e
	}
	return f, nil
}
func (r *Root) ReadPrivate(name string, max int) ([]byte, error) {
	f, e := r.openPrivate(name)
	if e != nil {
		return nil, e
	}
	defer func() { _ = f.Close() }()
	data, e := io.ReadAll(io.LimitReader(f, int64(max)+1))
	if e != nil {
		return nil, e
	}
	if len(data) > max {
		return nil, fmt.Errorf("private file exceeds limit")
	}
	return data, nil
}
func (r *Root) RemovePrivate(name string) error {
	f, e := r.openPrivate(name)
	if e != nil {
		return e
	}
	defer func() { _ = f.Close() }()
	var original, current unix.Stat_t
	if e = unix.Fstat(int(f.Fd()), &original); e != nil {
		return e
	}
	if e = unix.Fstatat(int(r.dir.Fd()), name, &current, unix.AT_SYMLINK_NOFOLLOW); e != nil {
		return e
	}
	if original.Dev != current.Dev || original.Ino != current.Ino {
		return fmt.Errorf("private file changed; refusing unlink")
	}
	if e = unix.Unlinkat(int(r.dir.Fd()), name, 0); e != nil {
		return e
	}
	return r.dir.Sync()
}

// WritePrivate atomically publishes a file. replace=false never overwrites a
// receipt/manifest. Existing replacement targets must pass private-file checks.
func (r *Root) WritePrivate(name string, data []byte, replace bool) error {
	if !privateName().MatchString(name) || len(data) > 1<<20 {
		return fmt.Errorf("invalid private file/limit")
	}
	if replace {
		f, e := r.openPrivate(name)
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		if f != nil {
			_ = f.Close()
		}
	}
	id, e := NewID()
	if e != nil {
		return e
	}
	temp := ".tmp-" + id
	fd, e := unix.Openat(int(r.dir.Fd()), temp, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if e != nil {
		return e
	}
	f := os.NewFile(uintptr(fd), temp)
	defer func() { _ = f.Close(); _ = unix.Unlinkat(int(r.dir.Fd()), temp, 0) }()
	if _, e = f.Write(data); e != nil {
		return e
	}
	if e = f.Sync(); e != nil {
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if replace {
		e = unix.Renameat(int(r.dir.Fd()), temp, int(r.dir.Fd()), name)
	} else {
		e = publishExclusive(int(r.dir.Fd()), temp, name)
	}
	if e != nil {
		return e
	}
	return r.dir.Sync()
}
