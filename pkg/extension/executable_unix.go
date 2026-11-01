//go:build linux || darwin

package extension

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/MohamedElashri/txm/pkg/core"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"path/filepath"
)

// ExecutableHash rejects executable paths replaceable by another UID. Same-UID
// writers and privileged administrators remain within the explicit trust model.
func ExecutableHash(path string) (string, error) {
	if !filepath.IsAbs(path) || NoSymlinks(path) != nil {
		return "", core.Failure("denied", "reviewed executable requires an absolute path without symlinks")
	}
	// Use the filesystem root's mapped administrator UID; containers may map
	// host root to an overflow UID instead of exposing numeric zero.
	var rootStat unix.Stat_t
	if e := unix.Lstat(string(filepath.Separator), &rootStat); e != nil {
		return "", e
	}
	for current := path; ; current = filepath.Dir(current) {
		var stat unix.Stat_t
		if e := unix.Lstat(current, &stat); e != nil {
			return "", e
		}
		stickyRoot := stat.Mode&unix.S_IFMT == unix.S_IFDIR && stat.Mode&unix.S_ISVTX != 0 && stat.Uid == rootStat.Uid
		if (stat.Uid != rootStat.Uid && stat.Uid != uint32(os.Geteuid())) || (stat.Mode&0022 != 0 && !stickyRoot) {
			return "", core.Failure("denied", "reviewed executable path is writable by another user")
		}
		if current == filepath.Dir(current) {
			break
		}
	}
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer func() { _ = f.Close() }()
	s, e := f.Stat()
	if e != nil || !s.Mode().IsRegular() || s.Mode().Perm()&0111 == 0 || s.Mode().Perm()&0022 != 0 || s.Size() > 128<<20 {
		return "", core.Failure("denied", "invalid reviewed executable or size limit")
	}
	h := sha256.New()
	if _, e = io.Copy(h, io.LimitReader(f, (128<<20)+1)); e != nil {
		return "", e
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
