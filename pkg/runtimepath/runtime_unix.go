//go:build linux || darwin

package runtimepath

import (
	"crypto/sha256"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

func Supported() bool { return true }
func Open() (*Root, error) {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base != "" && !filepath.IsAbs(base) {
		return nil, fmt.Errorf("XDG_RUNTIME_DIR must be absolute")
	}
	if base == "" {
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
		dir, err := privateChild(parent, fmt.Sprintf("txm-%d", os.Geteuid()))
		if err != nil {
			return nil, err
		}
		base = filepath.Join(temp, fmt.Sprintf("txm-%d", os.Geteuid()))
		defer func() { _ = dir.Close() }()
		last, e := privateChild(dir, "v1")
		if e != nil {
			return nil, e
		}
		return &Root{filepath.Join(base, "v1"), last}, nil
	}
	dir, err := openDirectory(base)
	if err != nil {
		return nil, err
	}
	defer func() { _ = dir.Close() }()
	if err = checkPrivate(dir); err != nil {
		return nil, err
	}
	return privateRoot(dir, base)
}
func privateRoot(base *os.File, path string) (*Root, error) {
	first, err := privateChild(base, "txm")
	if err != nil {
		return nil, err
	}
	defer func() { _ = first.Close() }()
	last, err := privateChild(first, "v1")
	if err != nil {
		return nil, err
	}
	return &Root{filepath.Join(path, "txm", "v1"), last}, nil
}
func openDirectory(path string) (*os.File, error) {
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	for _, part := range strings.Split(filepath.Clean(path), "/") {
		if part == "" {
			continue
		}
		next, e := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		_ = unix.Close(fd)
		if e != nil {
			return nil, e
		}
		fd = next
	}
	return os.NewFile(uintptr(fd), path), nil
}
func checkPrivate(f *os.File) error {
	var st unix.Stat_t
	if err := unix.Fstat(int(f.Fd()), &st); err != nil {
		return err
	}
	if st.Uid != uint32(os.Geteuid()) || st.Mode&0777 != 0700 || st.Mode&unix.S_IFMT != unix.S_IFDIR {
		return fmt.Errorf("unsafe native runtime directory: require owned mode 0700")
	}
	return nil
}
func privateChild(parent *os.File, name string) (*os.File, error) {
	if err := unix.Mkdirat(int(parent.Fd()), name, 0700); err != nil && err != unix.EEXIST {
		return nil, err
	}
	fd, err := unix.Openat(int(parent.Fd()), name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), name)
	if err = checkPrivate(f); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}
func (r *Root) Lock(name string) (*os.File, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	key := fmt.Sprintf("n-%x.lock", sha256.Sum256([]byte(name)))
	fd, err := openNameLock(int(r.dir.Fd()), key)
	if err != nil {
		return nil, fmt.Errorf("open native name lock: %w", err)
	}
	f := os.NewFile(uintptr(fd), key)
	var st unix.Stat_t
	if err = unix.Fstat(fd, &st); err == nil && (st.Uid != uint32(os.Geteuid()) || st.Mode&0777 != 0600 || st.Mode&unix.S_IFMT != unix.S_IFREG || st.Nlink != 1) {
		err = fmt.Errorf("unsafe native name lock")
	}
	if err == nil {
		err = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)
	}
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("session name locked or unsafe: %w", err)
	}
	return f, nil
}
func (r *Root) SocketInfo(id string) (os.FileInfo, error) {
	path, err := r.SocketPath(id)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	// os.FileInfo uses syscall.Stat_t; use Fstatat for portable Unix ownership.
	var stat unix.Stat_t
	if err = unix.Fstatat(int(r.dir.Fd()), filepath.Base(path), &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return nil, err
	}
	if stat.Uid != uint32(os.Geteuid()) || stat.Mode&unix.S_IFMT != unix.S_IFSOCK || stat.Mode&0777 != 0600 {
		return nil, fmt.Errorf("unsafe native socket")
	}
	return info, nil
}
func (r *Root) RemoveSocket(id string, original os.FileInfo) error {
	info, err := r.SocketInfo(id)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !os.SameFile(original, info) {
		return fmt.Errorf("native socket instance changed; refusing unlink")
	}
	path, _ := r.SocketPath(id)
	return unix.Unlinkat(int(r.dir.Fd()), filepath.Base(path), 0)
}
func VerifyPeer(c net.Conn) error {
	uc, ok := c.(*net.UnixConn)
	if !ok {
		return fmt.Errorf("native transport is not Unix")
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return err
	}
	var peerErr error
	if err = raw.Control(func(fd uintptr) { peerErr = checkPeer(int(fd)) }); err != nil {
		return err
	}
	return peerErr
}

// OpenLog never follows a final symlink or appends terminal data to a public,
// foreign or multiply linked existing file. Logging remains explicitly opt-in.
func OpenLog(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_WRONLY|unix.O_APPEND|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK|unix.O_NOCTTY, 0600)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	var st unix.Stat_t
	if err = unix.Fstat(fd, &st); err == nil && (st.Uid != uint32(os.Geteuid()) || st.Mode&0777 != 0600 || st.Mode&unix.S_IFMT != unix.S_IFREG || st.Nlink != 1) {
		err = fmt.Errorf("native log must be an owned, mode-0600 regular file with one link")
	}
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}
