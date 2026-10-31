// Package runtimepath owns private native runtime paths, instances and name locks.
package runtimepath

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Root struct {
	Path string
	dir  *os.File
}

func (r *Root) Close() error { return r.dir.Close() }
func NewID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(id[:]), nil
}

var validName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
var validID = regexp.MustCompile(`^[0-9a-f]{32}$`)

func ValidID(id string) bool { return validID.MatchString(id) }
func ValidateName(name string) error {
	if !validName.MatchString(name) || len(name) > 128 {
		return fmt.Errorf("invalid native session name (1..128 ASCII letters, digits, dash or underscore)")
	}
	return nil
}
func (r *Root) SocketPath(id string) (string, error) {
	if !ValidID(id) {
		return "", fmt.Errorf("invalid native instance ID")
	}
	path := filepath.Join(r.Path, "s-"+id+".sock")
	if len(path) >= 104 {
		return "", fmt.Errorf("native runtime path exceeds portable Unix socket limit")
	}
	return path, nil
}

// Lock inodes remain stable across lifetimes. Enumerate in bounded batches so
// accumulated reusable name locks do not impose a lifetime session-name cap.
func (r *Root) Names(ctx context.Context) ([]string, error) {
	var names []string
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		entries, err := r.dir.ReadDir(128)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, "s-") && strings.HasSuffix(name, ".sock") && ValidID(strings.TrimSuffix(strings.TrimPrefix(name, "s-"), ".sock")) {
				names = append(names, name)
				if len(names) > 1024 {
					return nil, fmt.Errorf("native socket candidate limit exceeded")
				}
			}
		}
		if errors.Is(err, io.EOF) {
			return names, nil
		}
	}
}
