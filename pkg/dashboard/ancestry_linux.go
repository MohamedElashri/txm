package dashboard

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/MohamedElashri/txm/pkg/runtimepath"
)

// CurrentNativeInstance is a conservative, deny-only feedback-loop guard. The
// kernel ancestry/controlling-TTY check is never a lifecycle or ownership proof,
// never persists a PID, and grants no action or authority. It also works with
// earlier framed native servers that supply no child environment marker.
const UnverifiedNativeContext = "unverified"

func CurrentNativeInstance() string {
	read := func(pid int, name string) string {
		f, e := os.Open(filepath.Join("/proc", strconv.Itoa(pid), name))
		if e != nil {
			return ""
		}
		defer func() { _ = f.Close() }()
		b, e := io.ReadAll(io.LimitReader(f, 8192))
		if e != nil {
			return ""
		}
		return string(b)
	}
	pid := os.Getpid()
	tty := ""
	childTTY := ""
	seen := map[int]bool{}
	for range 64 {
		if pid <= 1 {
			return ""
		}
		if seen[pid] {
			return UnverifiedNativeContext
		}
		seen[pid] = true
		parent, currentTTY, ok := procContext(read(pid, "stat"))
		if !ok {
			return UnverifiedNativeContext
		}
		if tty == "" {
			tty = currentTTY
		}
		if tty != "0" && childTTY == tty {
			command := read(pid, "cmdline")
			if command == "" {
				return UnverifiedNativeContext
			}
			if id := serverInstance(strings.Split(command, "\x00")); id != "" {
				return id
			}
		}
		childTTY = currentTTY
		pid = parent
	}
	return UnverifiedNativeContext
}
func procContext(stat string) (int, string, bool) {
	end := strings.LastIndex(stat, ")")
	if end < 0 {
		return 0, "", false
	}
	fields := strings.Fields(stat[end+1:])
	if len(fields) < 5 {
		return 0, "", false
	}
	parent, e := strconv.Atoi(fields[1])
	return parent, fields[4], e == nil
}
func serverInstance(args []string) string {
	if len(args) < 2 || args[1] != "server" {
		return ""
	}
	for i := 2; i < len(args); i++ {
		value := strings.TrimPrefix(args[i], "--instance=")
		if args[i] == "--instance" && i+1 < len(args) {
			value = args[i+1]
		} else if value == args[i] {
			continue
		}
		if runtimepath.ValidID(value) {
			return value
		}
	}
	return UnverifiedNativeContext
}
