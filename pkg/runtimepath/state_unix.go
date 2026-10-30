//go:build linux || darwin

package runtimepath

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// HostID initializes private local identity without a coordinator. Existing
// identity is never overwritten, repaired or regenerated after corruption.
func HostID(ctx context.Context) (string, error) {
	if e := ctx.Err(); e != nil {
		return "", e
	}
	base := os.Getenv("XDG_STATE_HOME")
	if base != "" && !filepath.IsAbs(base) {
		return "", fmt.Errorf("XDG_STATE_HOME must be absolute")
	}
	if base == "" {
		home, e := os.UserHomeDir()
		if e != nil {
			return "", e
		}
		base = filepath.Join(home, ".local", "state")
	}
	if !filepath.IsAbs(base) {
		return "", fmt.Errorf("state path must be absolute")
	}
	parent, e := stateDirectory(base)
	if e != nil {
		return "", e
	}
	defer func() { _ = parent.Close() }()
	var st unix.Stat_t
	if e = unix.Fstat(int(parent.Fd()), &st); e != nil {
		return "", e
	}
	if st.Uid != uint32(os.Geteuid()) || st.Mode&0022 != 0 {
		return "", fmt.Errorf("unsafe state parent")
	}
	r, e := privateRoot(parent, base)
	if e != nil {
		return "", e
	}
	defer func() { _ = r.Close() }()
	var lock *os.File
	for {
		lock, e = r.Lock("host-id")
		if e == nil {
			break
		}
		if !errors.Is(e, unix.EWOULDBLOCK) && !errors.Is(e, unix.EAGAIN) {
			return "", e
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
	defer func() { _ = lock.Close() }()
	id, e := readHost(r)
	if e == nil {
		return id, nil
	}
	if !errors.Is(e, os.ErrNotExist) {
		return "", e
	}
	if e = ctx.Err(); e != nil {
		return "", e
	}
	id, e = NewID()
	if e != nil {
		return "", e
	}
	temp := ".host-" + id
	fd, e := unix.Openat(int(r.dir.Fd()), temp, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if e != nil {
		return "", e
	}
	f := os.NewFile(uintptr(fd), temp)
	defer func() { _ = f.Close(); _ = unix.Unlinkat(int(r.dir.Fd()), temp, 0) }()
	data, _ := json.Marshal(struct {
		Schema int    `json:"schema_version"`
		ID     string `json:"host_id"`
	}{1, id})
	if _, e = f.Write(append(data, '\n')); e != nil {
		return "", e
	}
	if e = f.Sync(); e != nil {
		return "", e
	}
	// Link publication is atomic and fails if an identity already exists.
	if e = unix.Linkat(int(r.dir.Fd()), temp, int(r.dir.Fd()), "host.json", 0); e != nil {
		return "", e
	}
	if e = unix.Unlinkat(int(r.dir.Fd()), temp, 0); e != nil {
		return "", e
	}
	if e = r.dir.Sync(); e != nil {
		return "", e
	}
	return id, nil
}
func stateDirectory(path string) (*os.File, error) {
	fd, e := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if e != nil {
		return nil, e
	}
	for _, part := range strings.Split(filepath.Clean(path), "/") {
		if part == "" {
			continue
		}
		next, err := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if err == unix.ENOENT {
			if err = unix.Mkdirat(fd, part, 0700); err != nil && err != unix.EEXIST {
				_ = unix.Close(fd)
				return nil, err
			}
			next, err = unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		}
		_ = unix.Close(fd)
		if err != nil {
			return nil, err
		}
		fd = next
	}
	return os.NewFile(uintptr(fd), path), nil
}
func readHost(r *Root) (string, error) {
	fd, e := unix.Openat(int(r.dir.Fd()), "host.json", unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if e != nil {
		return "", e
	}
	f := os.NewFile(uintptr(fd), "host.json")
	defer func() { _ = f.Close() }()
	var st unix.Stat_t
	if e = unix.Fstat(fd, &st); e != nil {
		return "", e
	}
	if st.Uid != uint32(os.Geteuid()) || st.Mode&0777 != 0600 || st.Mode&unix.S_IFMT != unix.S_IFREG || st.Nlink != 1 || st.Size > 256 {
		return "", fmt.Errorf("unsafe host identity file")
	}
	dec := json.NewDecoder(io.LimitReader(f, 257))
	token, e := dec.Token()
	if e != nil || token != json.Delim('{') {
		return "", fmt.Errorf("corrupt host identity")
	}
	seen := map[string]bool{}
	id := ""
	schema := 0
	for dec.More() {
		key, e := dec.Token()
		if e != nil {
			return "", e
		}
		s, ok := key.(string)
		if !ok || seen[s] {
			return "", fmt.Errorf("corrupt host identity fields")
		}
		seen[s] = true
		switch s {
		case "host_id":
			e = dec.Decode(&id)
		case "schema_version":
			e = dec.Decode(&schema)
		default:
			return "", fmt.Errorf("unknown host identity field")
		}
		if e != nil {
			return "", e
		}
	}
	if _, e = dec.Token(); e != nil {
		return "", e
	}
	var extra any
	if dec.Decode(&extra) != io.EOF || schema != 1 || !ValidID(id) || len(seen) != 2 {
		return "", fmt.Errorf("corrupt or unsupported host identity")
	}
	return id, nil
}
