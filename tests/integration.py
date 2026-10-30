#!/usr/bin/env python3
"""Isolated smoke checks and opt-in baseline defect observations.

--baseline reports desired behavior as pass/fail; existing bugs intentionally
exit nonzero and never count as a passing release/regression suite.
"""
import argparse
import fcntl
import json
import os
from pathlib import Path
import shutil
import socket
import statistics
import struct
import sys
import termios
import time
from sandbox import Sandbox, native_server, wait_for
from wire import Peer


def recorder(box, backend, listing, typed=False):
    if typed and backend=="tmux":
        listing="\n".join(f"${i+1}|{name}|0" for i,name in enumerate(listing.splitlines()))
    log = box.root / "calls.jsonl"
    box.executable(backend, f'''#!{sys.executable}
import json, sys
with open({str(log)!r}, 'a') as f:
    f.write(json.dumps(sys.argv[1:]) + '\\n')
if {typed!r} and sys.argv[1] == '-V': print('tmux 3.4')
if sys.argv[1] in ('list-sessions', '-ls'):
    print({listing!r})
if len(sys.argv) > 1 and sys.argv[1] == 'has-session':
    sys.exit(0 if sys.argv[-1] in {listing.splitlines()!r} else 1)
''')
    box.env["TXM_DEFAULT_BACKEND"] = backend
    return log


def status(sock):
    with Peer(sock) as c:
        if c.legacy:
            c.socket.sendall(b"\x04")
            return c.socket.recv(1)
        return bytes([c.request('status')['attached_clients']])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", required=True, type=Path)
    parser.add_argument("--baseline", action="store_true")
    parser.add_argument("--report", type=Path)
    args = parser.parse_args()
    binary = args.binary.resolve(strict=True)
    results = []
    metrics = {}

    def record(name, passed, detail):
        results.append({"check": name, "passed": bool(passed), "detail": detail})
        print(f"{'PASS' if passed else 'FAIL'} {name}: {detail}", flush=True)

    tmux = shutil.which("tmux")
    if tmux:
        with Sandbox() as protected, Sandbox() as box:
            protected.start_tmux(tmux)
            box.start_tmux(tmux)
            created = box.run([binary, "create", "sample"], check=False)
            listed = box.run([binary, "list"], check=False)
            record("isolated_tmux_cli", created.returncode == 0 and "sample:" in listed.stdout
                   and protected.guard_name not in listed.stdout, "create/list through guarded -S wrapper")
            box.run([binary, "nuke"], check=False)
            wait_for(lambda: box.tmux_server.poll() is not None)
            protected.verify_tmux()
            record("tmux_nuke_isolation", True, "second disposable server survives target cleanup")
    else:
        results.append({"check": "isolated_tmux_cli", "skipped": "tmux unavailable"})

    with Sandbox() as box:
        durations = []
        for _ in range(10):
            start = time.perf_counter()
            box.run([binary, "version"])
            durations.append((time.perf_counter() - start) * 1000)
        metrics["version_median_ms"] = statistics.median(durations)
        with native_server(box, binary, legacy=args.baseline) as (server, sock, _):
            start = time.perf_counter()
            box.run([binary, "list"])
            metrics["native_one_session_list_ms"] = (time.perf_counter() - start) * 1000
            start = time.perf_counter()
            with Peer(sock) as client:
                client.attach()
                wait_for(lambda: status(sock) == b"\x01")
                metrics["native_socket_attach_ms"] = (time.perf_counter() - start) * 1000
            wait_for(lambda: status(sock) == b"\x00")
            record("R08_native_socket_detach", server.poll() is None and sock.exists(),
                   "real PTY child/server survive socket attach/disconnect; not a raw-TTY restoration test")
            master, slave = os.openpty()
            try:
                fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 24, 80, 0, 0))
                original_tty = termios.tcgetattr(slave)
                start = time.perf_counter()
                attached = box.start([binary, "attach", "sample"], stdin=slave, stdout=slave, stderr=slave)
                wait_for(lambda: status(sock) == b"\x01")
                metrics["native_cli_pty_attach_ms"] = (time.perf_counter() - start) * 1000
                os.write(master, b"\x1c")
                attached.wait(timeout=5)
                record("R08_native_tty_restore", attached.returncode == 0
                       and original_tty == termios.tcgetattr(slave) and server.poll() is None,
                       "CLI attach on real PTY, Ctrl-backslash detach, terminal attributes restored")
                wait_for(lambda: status(sock) == b"\x00")
            finally:
                os.close(master)
                os.close(slave)
            if sys.platform.startswith("linux"):
                for line in Path(f"/proc/{server.pid}/status").read_text().splitlines():
                    if line.startswith("VmRSS:"):
                        metrics["native_server_rss_kib"] = int(line.split()[1])
            if args.baseline:
                killed = box.run([binary, "delete", "sample"], check=False)
                time.sleep(0.1)
                record("R01_native_kill", server.poll() is not None and not sock.exists(),
                       f"CLI exit {killed.returncode}; actual server live={server.poll() is None}")
                if server.poll() is None:
                    injected = box.run([binary, "exec", "sample", "unused", "unused", "TXM_INPUT_SENTINEL"], check=False)
                    dumped = box.run([binary, "dump", "sample"], check=False)
                    record("R09_native_input", "TXM_INPUT_SENTINEL" in dumped.stdout,
                           f"CLI exit {injected.returncode}; no PTY echo means standalone input was rejected")

    # One small real-PTY throughput sample, with a draining attached client.
    # Gate the child until attach completes; include VT parsing and fanout time.
    with Sandbox() as box:
        gate = box.root / "emit"
        emitter = box.root / "emitter.py"
        emitter.write_text(f'''import sys,time
from pathlib import Path
gate=Path({str(gate)!r})
while not gate.exists(): time.sleep(0.01)
for _ in range(64): sys.stdout.buffer.write(b'x'*4096+b'\\n')
sys.stdout.buffer.write(b'TXM_THROUGHPUT_END\\n')
sys.stdout.buffer.flush()
time.sleep(120)
''')
        with native_server(box, binary, "throughput", [sys.executable, emitter], legacy=args.baseline) as (_, sock, _):
            with Peer(sock) as client:
                client.attach()
                wait_for(lambda: status(sock) == b"\x01")
                start = time.perf_counter()
                gate.touch()
                tail = b""
                count = 0
                while b"TXM_THROUGHPUT_END" not in tail:
                    chunk = client.recv_data()
                    if not chunk:
                        raise RuntimeError("PTY stream closed before throughput sentinel")
                    count += len(chunk)
                    tail = (tail + chunk)[-128:]
                elapsed = time.perf_counter() - start
                metrics["native_pty_bytes"] = count
                metrics["native_pty_mib_per_s"] = count / (1024 * 1024) / elapsed
                record("native_pty_output", count >= 64 * 4096, "sentinel reached attached client through PTY/VT fanout")

    if args.baseline:
        with Sandbox() as box:
            out = box.run([binary, "window", "list", "missing"], check=False)
            record("R03_cli_failure", out.returncode != 0 and "[ERROR]" not in out.stdout,
                   f"unsupported operation exit={out.returncode}, error_on_stdout={'[ERROR]' in out.stdout}")
            (Path(box.env["HOME"]) / ".txm").mkdir()
            (Path(box.env["HOME"]) / ".txm/config").mkdir()  # deterministic read error, even as root
            out = box.run([binary, "version"], check=False)
            record("R10_config_error", "panic:" not in out.stderr,
                   f"config path is a directory; exit={out.returncode}, panic={'panic:' in out.stderr}")
        with Sandbox() as box:
            log = recorder(box, "tmux", "sample")
            box.run([binary, "attach", "--read-only", "sample"], check=False)
            calls = [json.loads(line) for line in log.read_text().splitlines()]
            record("R04_tmux_read_only", any(c[0] == "attach-session" and "-r" in c for c in calls),
                   "argv probe only: adapter must request enforced read-only attach")
        with Sandbox() as box:
            log = recorder(box, "zellij", "foobar")
            box.run([binary, "create", "foo"], check=False)
            calls = [json.loads(line) for line in log.read_text().splitlines()]
            record("R05_zellij_exact_name", any(c[0] == "attach" for c in calls),
                   "fake backend lists foobar; creating foo must reach create, not conflict")
        with Sandbox() as box:
            log = recorder(box, "screen", "1234.foobar (Detached)")
            box.run([binary, "delete", "foo"], check=False)
            calls = [json.loads(line) for line in log.read_text().splitlines()]
            record("R05_screen_exact_name", not any("quit" in c for c in calls),
                   "fake backend lists only foobar; delete foo must not quit it")
        with Sandbox() as box:
            log = recorder(box, "tmux", "")
            profile = box.root / "profile.toml"
            profile.write_text('name="profile"\nbackend="native"\n[[window]]\nname="editor"\n[[window.pane]]\ncommand="printf sentinel"\ndir="/does-not-exist"\n')
            box.env["TXM_SESSION_PREFIX"] = "pref_"
            box.run([binary, "profile", profile], check=False)
            calls = [json.loads(line) for line in log.read_text().splitlines()]
            record("R06_R11_profile_preflight", not any(c[0] == "new-session" for c in calls),
                   "invalid cwd and unsupported layout must fail before creation; backend/prefix/target need fixes")
        with Sandbox() as box:
            with native_server(box, binary, "duplicate", legacy=args.baseline) as (first, sock, _):
                original = sock.stat().st_ino
                with native_server(box, binary, "duplicate", legacy=args.baseline):
                    record("R02_duplicate_native", sock.stat().st_ino == original,
                           "direct duplicate server launch must not replace an active socket")
        with Sandbox() as box:
            out = box.run([binary, "create", "badexec", "/does-not-exist"], check=False)
            record("R12_startup_exec_failure", out.returncode != 0,
                   f"missing executable must fail creation; CLI exit={out.returncode}")

    report = {"mode": "baseline" if args.baseline else "smoke", "checks": results,
              "metrics": metrics, "metric_limits": "10 startup samples; one list/socket-attach/CLI-PTY-attach/256-KiB PTY sample; no performance threshold claim"}
    if args.report:
        args.report.write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps({"metrics": metrics}), flush=True)
    return 1 if any(r.get("passed") is False for r in results) else 0


if __name__ == "__main__":
    sys.exit(main())
