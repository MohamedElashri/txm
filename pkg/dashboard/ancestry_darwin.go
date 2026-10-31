package dashboard

import (
	"bytes"
	"encoding/binary"
	"os"

	"golang.org/x/sys/unix"
)

// Like the Linux guard, this is deny-only. A PID/TTY is never an authority to
// act on a session. Missing visibility or changing process identity fails closed.
func CurrentNativeInstance() string {
	pid := os.Getpid()
	var tty, childTTY int32 = -1, -1
	seen := map[int]bool{}
	for range 64 {
		if pid <= 1 {
			return ""
		}
		if seen[pid] {
			return UnverifiedNativeContext
		}
		seen[pid] = true
		info, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
		if err != nil || int(info.Proc.P_pid) != pid {
			return UnverifiedNativeContext
		}
		if pid == os.Getpid() {
			tty = info.Eproc.Tdev
		}
		if tty != -1 && childTTY == tty {
			// KERN_PROCARGS2 also contains the environment. Parse only argc
			// arguments; never expose, retain or interpret the trailing bytes.
			raw, err := unix.SysctlRaw("kern.procargs2", pid)
			if err != nil {
				return UnverifiedNativeContext
			}
			args, ok := darwinArguments(raw)
			if !ok {
				return UnverifiedNativeContext
			}
			after, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
			if err != nil || after.Proc.P_pid != info.Proc.P_pid || after.Proc.P_starttime != info.Proc.P_starttime || after.Eproc.Ppid != info.Eproc.Ppid || after.Eproc.Tdev != info.Eproc.Tdev {
				return UnverifiedNativeContext
			}
			if id := serverInstance(args); id != "" {
				return id
			}
		}
		childTTY = info.Eproc.Tdev
		pid = int(info.Eproc.Ppid)
	}
	return UnverifiedNativeContext
}

func darwinArguments(raw []byte) ([]string, bool) {
	if len(raw) < 5 || len(raw) > 1<<20 {
		return nil, false
	}
	argc := int(binary.NativeEndian.Uint32(raw[:4]))
	if argc < 1 || argc > 4096 {
		return nil, false
	}
	// Skip the executable path and its NUL padding before argv[0].
	rest := raw[4:]
	end := bytes.IndexByte(rest, 0)
	if end < 1 {
		return nil, false
	}
	rest = bytes.TrimLeft(rest[end+1:], "\x00")
	args := make([]string, 0, argc)
	for range argc {
		end = bytes.IndexByte(rest, 0)
		if end < 0 {
			return nil, false
		}
		args = append(args, string(rest[:end]))
		rest = rest[end+1:]
	}
	return args, true
}
