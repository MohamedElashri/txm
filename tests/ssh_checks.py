#!/usr/bin/env python3
"""Real OpenSSH authentication, identity, transport and PTY regression checks.

macOS uses an unprivileged high-port loopback sshd and fresh fixture keys. Linux
execution is restricted to the explicitly isolated root container. No host SSH
configuration, accounts, login services or provider credentials are changed.
"""
import argparse
from contextlib import contextmanager
import fcntl
import json
import os
from pathlib import Path
import pty
import pwd
import re
import select
import shlex
import shutil
import socket
import struct
import subprocess
import sys
import tempfile
import termios
import time
import unittest

import agent_checks as a
from sandbox import native_server, wait_for
from wire import Peer
from native_checks import snapshot
from tty_state import attributes as tty_attributes, flags as tty_flags

BINARY = None
OBSERVATIONS = {}


def envelope(box, args, code=0, timeout=20):
    p = box.run([BINARY, '--json', *args], check=False, timeout=timeout)
    assert p.returncode == code, (args[:5], p.returncode, p.stdout, p.stderr)
    assert not p.stderr, p.stderr
    return json.loads(p.stdout)


def terminal_exit(proc, master, timeout=8):
    # A real terminal consumes output. Drain the PTY while waiting so Darwin's
    # small output queue cannot block the remote writer during detach.
    def exited():
        while select.select([master], [], [], 0)[0]:
            try:
                if not os.read(master, 65536):
                    break
            except OSError:
                break
        return proc.poll() is not None
    wait_for(exited, timeout=timeout)
    return proc.communicate(timeout=2)


@contextmanager
def loopback(local, remote):
    # StrictModes checks every ancestor. Darwin's per-user TMPDIR is private;
    # Linux's root-only path is used exclusively in the disposable container.
    parent = os.environ.get('TMPDIR') if sys.platform == 'darwin' else '/root'
    if not parent:
        raise RuntimeError('a private macOS TMPDIR is required')
    with tempfile.TemporaryDirectory(prefix='txm-ssh-auth-', dir=parent) as keys:
        yield from _loopback(local, remote, Path(keys).resolve())


def _loopback(local, remote, keys):
    local.guard();remote.guard()
    for name in ['host', 'client', 'wrong']:
        local.run(['/usr/bin/ssh-keygen', '-q', '-t', 'ed25519', '-N', '', '-f', keys/name])
    authorized = keys/'authorized_keys';authorized.write_bytes((keys/'client.pub').read_bytes());authorized.chmod(0o600)
    with socket.socket() as reserve:
        reserve.bind(('127.0.0.1', 0));port = reserve.getsockname()[1]
    remote_exe = remote.root/'bin/txm';remote_exe.symlink_to(BINARY)
    slow = remote.executable('txm-slow', '#!/bin/sh\n/bin/sleep 3\nexec '+shlex.quote(str(BINARY))+' "$@"\n')
    v2 = remote.executable('txm-v2', '#!/bin/sh\nprintf \'%s\\n\' \'{"schema_version":1,"data":{"protocol_version":2,"host_id":"'+'a'*32+'","sessions":[]},"errors":[],"partial":false}\'\n')
    launcher = remote.root/'ssh-endpoint.py'
    launcher.write_text(f'''import os,shlex
assert os.environ['HOME']=={remote.env['HOME']!r}
assert os.environ['ZDOTDIR']=={remote.env['HOME']!r}
command=os.environ.get('SSH_ORIGINAL_COMMAND','')
parts=shlex.split(command)
assert len(parts)==5 and parts[0] in {[str(remote_exe),str(slow),str(v2)]!r}
assert parts[1:4]==['internal','remote','--request']
assert all(c in 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-' for c in parts[4])
os.chdir({str(remote.root)!r})
os.execve(parts[0],parts,{remote.env!r})
''')
    cfg = local.root/'sshd.conf'
    cfg.write_text(f'''ListenAddress 127.0.0.1
Port {port}
HostKey {keys/'host'}
PidFile {local.root/'sshd.pid'}
AuthorizedKeysFile {authorized}
PermitRootLogin prohibit-password
PubkeyAuthentication yes
PasswordAuthentication no
KbdInteractiveAuthentication no
UsePAM no
PermitUserRC no
PermitUserEnvironment no
AllowTcpForwarding no
AllowAgentForwarding no
X11Forwarding no
UseDNS no
PrintMotd no
LogLevel ERROR
SetEnv HOME={remote.env['HOME']} ZDOTDIR={remote.env['HOME']} ENV=/dev/null BASH_ENV=/dev/null
ForceCommand {shlex.quote(sys.executable)} -I {shlex.quote(str(launcher))}
''')
    if sys.platform.startswith('linux'):
        Path('/run/sshd').mkdir(mode=0o755, exist_ok=True)
    local.run(['/usr/sbin/sshd', '-t', '-f', cfg])
    server_log = local.root/'sshd.stderr'
    with server_log.open('wb') as output:
        server = local.start(['/usr/sbin/sshd', '-D', '-e', '-f', cfg], stdout=subprocess.DEVNULL, stderr=output)
    def ready():
        if server.poll() is not None:
            raise AssertionError(server_log.read_text())
        try:
            with socket.create_connection(('127.0.0.1', port), timeout=.1):return True
        except OSError:return False
    wait_for(ready)
    known = local.root/'known_hosts'
    key = (keys/'host.pub').read_text().split()
    known.write_text(f'[127.0.0.1]:{port} {key[0]} {key[1]}\n')
    wrong_known = local.root/'wrong_known_hosts';wrong_key = (keys/'wrong.pub').read_text().split()
    wrong_known.write_text(f'[127.0.0.1]:{port} {wrong_key[0]} {wrong_key[1]}\n')
    client = local.root/'ssh.conf'
    client.write_text(f'''Host badkey
 UserKnownHostsFile {wrong_known}
 ControlMaster no
 ControlPath none
Host badauth
 IdentityFile {keys/'wrong'}
 ControlMaster no
 ControlPath none
Host dead
 Port 1
 ControlMaster no
 ControlPath none
Host fixture badkey dead
 IdentityFile {keys/'client'}
Host fixture badkey badauth dead
 HostName 127.0.0.1
 Port {port}
 User {pwd.getpwuid(os.geteuid()).pw_name}
 IdentitiesOnly yes
 IdentityAgent none
 UserKnownHostsFile {known}
 GlobalKnownHostsFile /dev/null
 ControlMaster auto
 ControlPersist 10
 ControlPath {local.root}/control-%C
 LogLevel ERROR
''')
    # OpenSSH selects its user's config through passwd, not a fixture HOME.
    # Supply a private -F while preserving every product argv/transport option.
    local.executable('ssh', f'''#!{sys.executable}
import os,sys
assert sys.argv[-2] in ('fixture','badkey','badauth','dead')
os.execv('/usr/bin/ssh',['/usr/bin/ssh','-F',{str(client)!r},*sys.argv[1:]])
''')
    def control(op):
        local.guard()
        return local.run(['/usr/bin/ssh', '-F', client, '-O', op, 'fixture'], check=False, timeout=5)
    state = {'port': port, 'executable': str(remote_exe), 'slow': str(slow), 'v2': str(v2),
             'config': client, 'server': server, 'control': control, 'known': known}
    try:
        yield state
    finally:
        control('exit')
        if server.poll() is None:
            server.terminate();server.wait(timeout=5)
        authorized.unlink()
        local.guard();remote.guard()


def allow_pin(local, fixture, executable=None):
    envelope(local, ['remote', 'allow', 'fixture', '--executable', executable or fixture['executable']])
    ident = envelope(local, ['remote', 'probe', 'fixture'])['data']['host_id']
    envelope(local, ['remote', 'pin', 'fixture', ident])
    return ident


class LoopbackSSHChecks(unittest.TestCase):
    def test_real_auth_hostkeys_hostpins_and_partial_failure(self):
        with a.fixture() as local, a.fixture() as remote, loopback(local, remote) as f:
            host = allow_pin(local, f)
            with native_server(remote, BINARY, name='remote_real'):
                data = envelope(local, ['remote', 'list', 'fixture'])['data'][0]
                self.assertEqual(data['sessions'][0]['locator']['name'], 'remote_real')
                self.assertEqual(data['host_id'], host)
                envelope(local, ['remote', 'pin', 'fixture', 'b'*32])
                envelope(local, ['remote', 'list', 'fixture'], 5)
                envelope(local, ['remote', 'pin', 'fixture', host])
                for alias in ['badkey', 'badauth']:
                    envelope(local, ['remote', 'allow', alias, '--executable', f['executable']])
                    envelope(local, ['remote', 'probe', alias], 7)
                envelope(local, ['remote', 'allow', 'dead', '--host-id', 'd'*32, '--executable', f['executable']])
                result = envelope(local, ['remote', 'list', 'fixture', 'dead'], 7)
                self.assertTrue(result['partial']);self.assertEqual(result['data'][0]['sessions'][0]['locator']['name'], 'remote_real')
                self.assertEqual(result['data'][1]['error']['code'], 'backend_unavailable')
            OBSERVATIONS['ssh_auth'] = {'actual_loopback': True, 'public_key_auth': True, 'bad_host_key_rejected': True,
                                         'bad_auth_key_rejected': True, 'host_pin_rejected': True, 'partial_failure': True}

    def test_controlmaster_reuse_reconnect_version_timeout_and_cancellation(self):
        with a.fixture() as local, a.fixture() as remote, loopback(local, remote) as f:
            host = allow_pin(local, f)
            def master():
                p = f['control']('check');self.assertEqual(p.returncode, 0, p.stderr)
                return int(re.search(r'pid=(\d+)', p.stderr).group(1))
            first = master();envelope(local, ['remote', 'list', 'fixture']);self.assertEqual(master(), first)
            self.assertEqual(f['control']('exit').returncode, 0)
            envelope(local, ['remote', 'probe', 'fixture']);second = master();self.assertNotEqual(first, second)
            envelope(local, ['remote', 'forget', 'fixture'])
            envelope(local, ['remote', 'allow', 'fixture', '--host-id', host, '--executable', f['v2']])
            envelope(local, ['remote', 'list', 'fixture'], 4)
            envelope(local, ['remote', 'forget', 'fixture'])
            envelope(local, ['remote', 'allow', 'fixture', '--host-id', host, '--executable', f['slow']])
            envelope(local, ['remote', 'list', 'fixture', '--timeout', '1s'], 6)
            proc = local.start([BINARY, '--json', 'remote', 'list', 'fixture'], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            time.sleep(.25);proc.terminate();out,err = proc.communicate(timeout=5)
            self.assertEqual(proc.returncode, 130);self.assertEqual(json.loads(out)['errors'][0]['code'], 'cancelled');self.assertFalse(err)
            OBSERVATIONS['ssh_controlmaster'] = {'same_master_reused': True, 'new_master_after_exit': True,
                                                'version_mismatch': True, 'timeout': True, 'cancelled': True}

    def test_actual_native_attach_readonly_cancel_tty_and_quoted_name(self):
        with a.fixture() as local, a.fixture() as remote, loopback(local, remote) as f:
            host = allow_pin(local, f)
            with native_server(remote, BINARY, name='remote_cat', argv=['/bin/cat']) as (_,sock,_):
                session = envelope(local, ['remote', 'list', 'fixture'])['data'][0]['sessions'][0]
                args = [BINARY,'--backend','native','remote','attach','fixture','--host-id',host,'--name','remote_cat',
                        '--session-id',session['backend_id'],'--instance',session['instance_id'],'--read-only']
                master,slave = pty.openpty();before = tty_attributes(slave)
                flags = [tty_flags(fd) for fd in (slave,)]
                try:
                    proc = local.start(args, stdin=slave, stdout=slave, stderr=subprocess.PIPE)
                    def attached():
                        with Peer(sock) as peer:return peer.request('status')['attached_clients'] > 0
                    wait_for(attached,timeout=8)
                    os.write(master,b'LOOPBACK_READONLY_SENTINEL\n');time.sleep(.2)
                    self.assertNotIn(b'LOOPBACK_READONLY_SENTINEL',snapshot(sock))
                    proc.terminate();_,err = terminal_exit(proc, master);self.assertEqual(proc.returncode,130,err)
                    self.assertEqual(tty_attributes(slave),before);self.assertEqual(tty_flags(slave),flags[0])
                    wait_for(lambda:not attached(),timeout=6)
                    normal = args[:-1]
                    proc = local.start(normal,stdin=slave,stdout=slave,stderr=subprocess.PIPE);wait_for(attached,timeout=8)
                    os.write(master,b'LOOPBACK_CONTROL_SENTINEL\n');wait_for(lambda:b'LOOPBACK_CONTROL_SENTINEL' in snapshot(sock),timeout=5)
                    os.write(master,b'\x1c');_,err = terminal_exit(proc, master);self.assertEqual(proc.returncode,0,err)
                    self.assertEqual(tty_attributes(slave),before)
                    marker = remote.root/'injection-must-not-run';evil = args[:];evil[evil.index('--name')+1]='name; touch '+str(marker)+'; $(id)'
                    proc = local.start(evil,stdin=slave,stdout=slave,stderr=subprocess.PIPE);_,err=terminal_exit(proc, master)
                    self.assertEqual(proc.returncode,1,err);self.assertFalse(marker.exists())
                finally:os.close(master);os.close(slave)
            OBSERVATIONS['ssh_native_attach'] = {'readonly_enforced':True,'control_input':True,'detach':True,
                                                  'cancelled':True,'tty_and_flags_restored':True,'encoded_name_safe':True}


    def test_real_tmux_attach_with_literal_name_readonly_and_detach(self):
        with a.fixture() as local,a.fixture() as remote,loopback(local,remote) as f:
            host=allow_pin(local,f);remote.start_tmux(shutil.which('tmux'))
            marker=remote.root/'tmux-input';injection=remote.root/'must-not-execute-name'
            receiver=remote.executable('receiver','#!/bin/sh\nwhile IFS= read -r value; do [ "$value" = "REAL_SSH_TMUX_INPUT" ] && printf received > '+shlex.quote(str(marker))+'; done\n')
            remote.run([BINARY,'--backend','tmux','create','ssh_tmux',str(receiver)])
            row=next(s for s in envelope(local,['--backend','tmux','remote','list','fixture'])['data'][0]['sessions'] if s['locator']['name']=='ssh_tmux')
            name='literal; touch '+str(injection)
            remote.run(['tmux','rename-session','-t',row['backend_id'],name])
            args=[BINARY,'--backend','tmux','remote','attach','fixture','--host-id',host,'--name',name,'--session-id',row['backend_id']]
            def attached():return int(remote.run(['tmux','display-message','-p','-t',row['backend_id'],'#{session_attached}']).stdout)>0
            master,slave=pty.openpty();before=tty_attributes(slave)
            try:
                fcntl.ioctl(slave,termios.TIOCSWINSZ,struct.pack('HHHH',30,120,0,0))
                proc=local.start([*args,'--read-only'],stdin=slave,stdout=slave,stderr=subprocess.PIPE);wait_for(attached,timeout=8)
                os.write(master,b'REAL_SSH_TMUX_INPUT\n');time.sleep(.2);self.assertFalse(marker.exists());self.assertFalse(injection.exists())
                proc.terminate();_,err=terminal_exit(proc, master);self.assertEqual(proc.returncode,130,err);self.assertEqual(tty_attributes(slave),before)
                wait_for(lambda:not attached(),timeout=6)
                proc=local.start(args,stdin=slave,stdout=slave,stderr=subprocess.PIPE);wait_for(attached,timeout=8)
                os.write(master,b'REAL_SSH_TMUX_INPUT\n');wait_for(marker.exists,timeout=5)
                os.write(master,b'\x02d');_,err=terminal_exit(proc, master);self.assertEqual(proc.returncode,0,err)
                self.assertEqual(tty_attributes(slave),before);self.assertFalse(injection.exists());remote.verify_tmux()
            finally:os.close(master);os.close(slave)
            OBSERVATIONS['ssh_tmux_attach']={'real_pty':True,'literal_shell_metacharacter_name':True,'readonly':True,'control_input':True,'detach_and_tty_restore':True}


def main():
    global BINARY
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', required=True, type=Path)
    parser.add_argument('--report', required=True, type=Path)
    args, remaining = parser.parse_known_args()
    linux_fixture = (sys.platform.startswith('linux') and os.geteuid() == 0
                     and Path('/.dockerenv').exists() and os.environ.get('TXM_LINUX_FIXTURE') == '1')
    if not (sys.platform == 'darwin' and os.geteuid() != 0 or linux_fixture):
        parser.error('requires unprivileged macOS or the isolated disposable Linux root container')
    for tool in ('/usr/sbin/sshd', '/usr/bin/ssh', '/usr/bin/ssh-keygen'):
        if not os.access(tool, os.X_OK):
            parser.error(tool+' is required')
    if shutil.which('tmux') is None:
        parser.error('tmux is required')
    BINARY = args.binary.resolve(strict=True)
    a.BINARY = BINARY
    program = unittest.main(argv=[sys.argv[0], *remaining], exit=False)
    version = subprocess.run(['/usr/bin/ssh', '-V'], capture_output=True, text=True, timeout=5)
    args.report.write_text(json.dumps({'schema_version': 1, 'passed': program.result.wasSuccessful(),
        'tests_run': program.result.testsRun, 'skipped': program.result.skipped,
        'ssh_version': (version.stdout+version.stderr).strip(), 'observations': OBSERVATIONS,
        'scope': sys.platform+' private loopback sshd; strict host keys, fixture keys and forced endpoint; no host configuration changes'}, indent=2)+'\n')
    return 0 if program.result.wasSuccessful() else 1


if __name__ == '__main__':
    raise SystemExit(main())
