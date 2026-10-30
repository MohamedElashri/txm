"""POSIX-only disposable fixtures. Never discovers a user's mux server.

This is an isolation guard for trusted project tests, not a sandbox against
malicious same-UID code. All subprocesses receive a constructed environment.
"""
import contextlib
import os
from pathlib import Path
import secrets
import shlex
import shutil
import signal
import stat
import subprocess
import sys
import tempfile
import time


class Sandbox:
    def __init__(self):
        if os.name != "posix":
            raise RuntimeError("native/tmux fixture requires POSIX")
        self.root = Path(tempfile.mkdtemp(prefix="txm-test-", dir="/tmp")).resolve()
        self.token = secrets.token_hex(16)
        self.marker = self.root / ".owner"
        self.marker.write_text(self.token)
        self.identity = (self.root.stat().st_dev, self.root.stat().st_ino)
        self.children = []
        self.tmux = None
        self.tmux_pid = None
        self.tmux_socket_identity = None
        self.socket = self.root / "tmux.sock"
        self.guard_name = "guard_" + self.token
        for name in ("home", "config", "state", "runtime", "tmp", "bin", "repo", "screen"):
            (self.root / name).mkdir(mode=0o700)
        self.env = {
            "HOME": str(self.root / "home"),
            "XDG_CONFIG_HOME": str(self.root / "config"),
            "XDG_STATE_HOME": str(self.root / "state"),
            "XDG_RUNTIME_DIR": str(self.root / "runtime"),
            "TMPDIR": str(self.root / "tmp"),
            "SCREENDIR": str(self.root / "screen"),
            "PATH": str(self.root / "bin"),
            "SHELL": "/bin/sh", "TERM": "xterm-256color", "LANG": "C",
            "TXM_DEFAULT_BACKEND": "native",
        }
        # A small explicit tool set for fixture shells; no inherited user PATH.
        for name in ("sh", "bash", "sleep", "cat", "stty", "printf"):
            path = shutil.which(name, path="/usr/bin:/bin")
            if path:
                (self.root / "bin" / name).symlink_to(path)
        for name in ("tmux", "zellij", "screen"):
            self.executable(name, "#!/bin/sh\necho 'backend disabled in fixture' >&2\nexit 125\n")

    def guard(self):
        info = self.root.lstat()
        if (not stat.S_ISDIR(info.st_mode) or info.st_uid != os.getuid()
                or stat.S_IMODE(info.st_mode) != 0o700
                or (info.st_dev, info.st_ino) != self.identity
                or self.marker.is_symlink() or self.marker.read_text() != self.token):
            raise RuntimeError("fixture ownership changed; refusing operation/cleanup")

    def executable(self, name, content):
        if Path(name).name != name:
            raise ValueError("executable must be a basename")
        path = self.root / "bin" / name
        if path.is_symlink():
            raise ValueError("refusing to overwrite a symlinked fixture executable")
        path.write_text(content)
        path.chmod(0o700)
        return path

    def run(self, argv, *, timeout=5, check=True, env=None, **kwargs):
        self.guard()
        return subprocess.run([str(x) for x in argv], cwd=self.root / "repo",
                              env=self.env if env is None else env, timeout=timeout,
                              check=check, text=True, capture_output=True, **kwargs)

    def start(self, argv, **kwargs):
        self.guard()
        child = subprocess.Popen([str(x) for x in argv], cwd=self.root / "repo",
                                 env=self.env, start_new_session=True, **kwargs)
        self.children.append(child)
        return child

    def start_tmux(self, executable):
        """Foreground server gives us a live process handle, not a reused PID."""
        self.guard()
        self.tmux = str(Path(executable).resolve(strict=True))
        logfile = self.root / "tmux.log"
        with logfile.open("wb") as log:
            self.tmux_server = self.start(
                [self.tmux, "-D", "-S", self.socket, "-f", "/dev/null"],
                stdout=log, stderr=log)
        wait_for(lambda: self.socket.exists() or self.tmux_server.poll() is not None)
        if self.tmux_server.poll() is not None:
            raise RuntimeError("isolated tmux server failed: " + logfile.read_text()[:4096])
        self.tmux_pid = self.tmux_server.pid
        info = self.socket.lstat()
        if not stat.S_ISSOCK(info.st_mode) or info.st_uid != os.getuid():
            raise RuntimeError("fixture tmux socket has wrong type/owner")
        self.tmux_socket_identity = (info.st_dev, info.st_ino)
        self.run([self.tmux, "-S", self.socket, "new-session", "-d", "-s",
                  self.guard_name, "exec /bin/sleep 120"])
        self.verify_tmux()
        # Route EVERY baseline adapter invocation through the same -S socket.
        # Reject attempts to supply a different global socket/config selector.
        self.executable("tmux", f'''#!{sys.executable}
import os, stat, subprocess, sys
from pathlib import Path
root = Path({str(self.root)!r})
info = root.lstat()
if (not stat.S_ISDIR(info.st_mode) or info.st_uid != os.getuid()
    or stat.S_IMODE(info.st_mode) != 0o700
    or (info.st_dev, info.st_ino) != {self.identity!r}
    or (root / '.owner').is_symlink()
    or (root / '.owner').read_text() != {self.token!r}):
    sys.exit('unsafe fixture root')
if len(sys.argv) < 2 or (sys.argv[1].startswith('-') and sys.argv[1] != '-V'):
    sys.exit('global tmux options denied by fixture')
sock = root / 'tmux.sock'
si = sock.lstat()
if not stat.S_ISSOCK(si.st_mode) or (si.st_dev, si.st_ino) != {self.tmux_socket_identity!r}:
    sys.exit('fixture socket changed')
prefix = [{self.tmux!r}, '-S', str(sock)]
if sys.argv[1] == 'kill-server':
    pid = subprocess.check_output(prefix + ['display-message', '-p', '#{{pid}}'], text=True).strip()
    if pid != {str(self.tmux_pid)!r}:
        sys.exit('fixture server identity changed')
    subprocess.run(prefix + ['has-session', '-t', {('=' + self.guard_name)!r}], check=True)
os.execv(prefix[0], prefix + sys.argv[1:])
''')
        self.env["TXM_DEFAULT_BACKEND"] = "tmux"

    def verify_tmux(self):
        self.guard()
        if self.tmux_server.poll() is not None:
            raise RuntimeError("fixture tmux server exited")
        info = self.socket.lstat()
        if (info.st_dev, info.st_ino) != self.tmux_socket_identity:
            raise RuntimeError("fixture socket changed; refusing cleanup")
        pid = self.run([self.tmux, "-S", self.socket, "display-message", "-p", "#{pid}"]).stdout.strip()
        if pid != str(self.tmux_pid):
            raise RuntimeError("fixture server PID does not match live child")
        self.run([self.tmux, "-S", self.socket, "has-session", "-t", "=" + self.guard_name])

    def close(self):
        self.guard()
        if self.tmux and self.tmux_server.poll() is None:
            self.verify_tmux()
            self.run([self.tmux, "-S", self.socket, "kill-server"])
        # Only process groups whose original Popen handle is still live.
        for child in reversed(self.children):
            if child.poll() is None:
                os.killpg(child.pid, signal.SIGTERM)
                try:
                    child.wait(timeout=3)
                except subprocess.TimeoutExpired:
                    os.killpg(child.pid, signal.SIGKILL)
                    child.wait(timeout=3)
            else:
                child.wait()
        self.guard()
        shutil.rmtree(self.root)

    def __enter__(self):
        return self

    def __exit__(self, *_):
        self.close()


def wait_for(predicate, timeout=5):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if predicate():
            return
        time.sleep(0.02)
    raise TimeoutError("fixture condition not reached")


@contextlib.contextmanager
def native_server(box, binary, name="sample", argv=None, legacy=False):
    """Owned foreground server + PTY child, even on the unsafe legacy runtime.

    The shell uses only builtins and exec: no descendant can escape teardown.
    Future process-tree cases must track child instance identity explicitly.
    """
    if not name or any(c not in "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-" for c in name):
        raise ValueError("unsafe native fixture name")
    pidfile = box.root / (name + "-" + secrets.token_hex(4) + ".pid")
    child = shlex.join([str(x) for x in (argv or ["/bin/sleep", "120"])])
    command = f"printf '%s' $$ > {shlex.quote(str(pidfile))}; exec {child}"
    instance=secrets.token_hex(16)
    server_args=["server"] if legacy else ["server","--instance",instance]
    server_log = box.root / (name + '-' + instance + '.stderr')
    with server_log.open('wb') as log:
        proc = box.start([binary, *server_args, name, "/bin/sh", "-c", command],
                         stdout=subprocess.DEVNULL, stderr=log)
    sock = box.root / "tmp" / ("txm-" + name + ".sock") if legacy else box.root / "runtime/txm/v1" / ("s-" + instance + ".sock")
    child_pid = None
    try:
        wait_for(lambda: (sock.exists() and pidfile.exists() and pidfile.stat().st_size) or proc.poll() is not None)
        if proc.poll() is not None:
            raise RuntimeError("native fixture server failed")
        child_pid = int(pidfile.read_text())
        yield proc, sock, child_pid
    finally:
        # The server remains alive/unreaped, so its unreaped PTY child PID
        # cannot be reused. Check that relationship before sending a signal.
        if child_pid and proc.poll() is None:
            if sys.platform.startswith("linux"):
                status = Path(f"/proc/{child_pid}/status")
                if status.exists() and f"PPid:\t{proc.pid}\n" in status.read_text():
                    os.kill(child_pid, signal.SIGTERM)
            else:
                # Portable parent verification; ps is read-only and PID-scoped.
                parent = subprocess.run(["/bin/ps", "-o", "ppid=", "-p", str(child_pid)],
                                        capture_output=True, text=True, check=True)
                if parent.stdout.strip() == str(proc.pid):
                    os.kill(child_pid, signal.SIGTERM)
            proc.wait(timeout=5)
        diagnostic = server_log.read_text()
        if proc.returncode == 66 or 'WARNING: DATA RACE' in diagnostic:
            raise AssertionError('native server race detector failure: ' + diagnostic[:8192])
