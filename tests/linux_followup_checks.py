#!/usr/bin/env python3
"""Real Linux-only gates. Requires a disposable root container, never host root."""
import argparse
import base64
from contextlib import contextmanager
import fcntl
import json
import os
from pathlib import Path
import pty
import re
import secrets
import select
import shlex
import shutil
import signal
import socket
import stat
import struct
import subprocess
import sys
import termios
import time
import unittest

import agent_checks as a
from sandbox import Sandbox, native_server, wait_for
from wire import Peer, packet, HELLO, REQUEST
from native_checks import snapshot

BINARY = None
REPORT = None
ZELLIJ = None
OBSERVATIONS = {}


def envelope(box, args, code=0, timeout=20):
    p = box.run([BINARY, '--json', *args], check=False, timeout=timeout)
    assert p.returncode == code, (args[:5], p.returncode, p.stdout, p.stderr)
    assert not p.stderr, p.stderr
    return json.loads(p.stdout)


@contextmanager
def traversable_for_peer_test(box, sock):
    # Prove the SO_PEERCRED gate separately from ordinary filesystem denial.
    # Relax only this fixture's ancestors/socket temporarily; restore before
    # Sandbox.guard/teardown or any normal owner CLI operation.
    paths = []
    p = sock.parent
    while p != box.root.parent:
        paths.append(p)
        if p == box.root:
            break
        p = p.parent
    original = [(p, stat.S_IMODE(p.stat().st_mode)) for p in paths]
    mode = stat.S_IMODE(sock.stat().st_mode)
    try:
        for p, _ in original:
            p.chmod(0o711)
        sock.chmod(0o666)
        yield
    finally:
        sock.chmod(mode)
        for p, saved in original:
            p.chmod(saved)


def foreign_probe(path, payload):
    script = '''import base64,json,os,socket,sys
os.setgroups([]);os.setgid(65534);os.setuid(65534)
s=socket.socket(socket.AF_UNIX);s.settimeout(4)
result={"uid":os.geteuid(),"connected":False,"rejected":False}
try:
 s.connect(sys.argv[1]);result['connected']=True
 try:s.sendall(base64.b64decode(sys.argv[2]));data=s.recv(4096)
 except (BrokenPipeError,ConnectionResetError):data=b''
 result['rejected']=not data
except PermissionError:result['filesystem_denied']=True
finally:s.close()
print(json.dumps(result))
'''
    p = subprocess.run([sys.executable, '-c', script, str(path), base64.b64encode(payload).decode()],
                       cwd='/tmp', env={'PATH': '/usr/bin:/bin', 'LANG': 'C'},
                       capture_output=True, text=True, timeout=6)
    assert p.returncode == 0, p.stderr
    return json.loads(p.stdout)


class ForeignUIDChecks(unittest.TestCase):
    def test_native_filesystem_and_actual_foreign_peer_rejection(self):
        with a.fixture() as box, native_server(box, BINARY, name='foreign_guard') as (server, sock, _):
            data = packet(HELLO, 1, {'role': 'control'})+packet(REQUEST, 2, {'op': 'kill'})
            denied = foreign_probe(sock, data)
            self.assertEqual(denied['uid'], 65534);self.assertTrue(denied['filesystem_denied'])
            with traversable_for_peer_test(box, sock):
                peer = foreign_probe(sock, data)
                self.assertTrue(peer['connected']);self.assertTrue(peer['rejected'])
            with Peer(sock, 'control') as owner:
                self.assertEqual(owner.identity['name'], 'foreign_guard')
                self.assertEqual(owner.request('status')['process_state'], 'running')
            self.assertIsNone(server.poll())
            OBSERVATIONS['foreign_native'] = {'filesystem_denied': True, 'connected_uid': peer['uid'],
                                               'peer_rejected': True, 'owner_and_child_survived': True}

    def test_coordinator_foreign_peer_cannot_stop_or_mutate_registry(self):
        with a.fixture() as box, a.coordinator(box) as service:
            envelope(box, ['agent', 'list'])
            registry = a.state(box)/'registry.json';before = registry.read_bytes()
            metadata = json.loads((a.runtime(box)/'coordinator.json').read_text())
            path = a.runtime(box)/('s-'+metadata['epoch']+'.sock')
            host = json.loads((a.state(box).parent/'host.json').read_text())['host_id']
            body = json.dumps({'schema_version': 1, 'epoch': metadata['epoch'], 'host_id': host,
                               'state_root': str(a.state(box)), 'op': 'stop'}).encode()
            framed = struct.pack('!I', len(body))+body
            self.assertTrue(foreign_probe(path, framed)['filesystem_denied'])
            with traversable_for_peer_test(box, path):
                result = foreign_probe(path, framed)
                self.assertTrue(result['connected']);self.assertTrue(result['rejected'])
            self.assertIsNone(service.poll());self.assertEqual(registry.read_bytes(), before)
            self.assertEqual(envelope(box, ['agent', 'list'])['data']['agents'], [])
            OBSERVATIONS['foreign_coordinator'] = {'connected_uid': result['uid'], 'peer_rejected': True,
                                                    'service_survived': True, 'registry_unchanged': True}


@contextmanager
def loopback(local, remote):
    local.guard();remote.guard()
    keys = local.root/'keys';keys.mkdir(mode=0o700)
    for name in ['host', 'client', 'wrong']:
        local.run(['/usr/bin/ssh-keygen', '-q', '-t', 'ed25519', '-N', '', '-f', keys/name])
    authdir = Path('/root/.ssh');authdir.mkdir(mode=0o700,exist_ok=True)
    authorized = authdir/('txm-'+local.token);authorized.write_bytes((keys/'client.pub').read_bytes());authorized.chmod(0o600)
    with socket.socket() as reserve:
        reserve.bind(('127.0.0.1', 0));port = reserve.getsockname()[1]
    remote_exe = remote.root/'bin/txm';remote_exe.symlink_to(BINARY)
    slow = remote.executable('txm-slow', '#!/bin/sh\n/bin/sleep 3\nexec '+shlex.quote(str(BINARY))+' "$@"\n')
    v2 = remote.executable('txm-v2', '#!/bin/sh\nprintf \'%s\\n\' \'{"schema_version":1,"data":{"protocol_version":2,"host_id":"'+'a'*32+'","sessions":[]},"errors":[],"partial":false}\'\n')
    launcher = remote.root/'ssh-endpoint.py'
    launcher.write_text(f'''import os,shlex
command=os.environ.get('SSH_ORIGINAL_COMMAND','')
parts=shlex.split(command)
assert len(parts)==5 and parts[0] in {[str(remote_exe),str(slow),str(v2)]!r}
assert parts[1:4]==['internal','remote','--request']
assert all(c in 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-' for c in parts[4])
os.execve('/bin/sh',['/bin/sh','-c',command],{remote.env!r})
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
ForceCommand /usr/bin/python3 {launcher}
''')
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
 User root
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
                master,slave = pty.openpty();before = termios.tcgetattr(slave)
                flags = [fcntl.fcntl(fd, fcntl.F_GETFL) for fd in (slave,)]
                try:
                    proc = local.start(args, stdin=slave, stdout=slave, stderr=subprocess.PIPE)
                    def attached():
                        with Peer(sock) as peer:return peer.request('status')['attached_clients'] > 0
                    wait_for(attached,timeout=8)
                    os.write(master,b'LOOPBACK_READONLY_SENTINEL\n');time.sleep(.2)
                    self.assertNotIn(b'LOOPBACK_READONLY_SENTINEL',snapshot(sock))
                    proc.terminate();_,err = proc.communicate(timeout=8);self.assertEqual(proc.returncode,130,err)
                    self.assertEqual(termios.tcgetattr(slave),before);self.assertEqual(fcntl.fcntl(slave,fcntl.F_GETFL),flags[0])
                    wait_for(lambda:not attached(),timeout=6)
                    normal = args[:-1]
                    proc = local.start(normal,stdin=slave,stdout=slave,stderr=subprocess.PIPE);wait_for(attached,timeout=8)
                    os.write(master,b'LOOPBACK_CONTROL_SENTINEL\n');wait_for(lambda:b'LOOPBACK_CONTROL_SENTINEL' in snapshot(sock),timeout=5)
                    os.write(master,b'\x1c');_,err = proc.communicate(timeout=8);self.assertEqual(proc.returncode,0,err)
                    self.assertEqual(termios.tcgetattr(slave),before)
                    marker = remote.root/'injection-must-not-run';evil = args[:];evil[evil.index('--name')+1]='name; touch '+str(marker)+'; $(id)'
                    proc = local.start(evil,stdin=slave,stdout=slave,stderr=subprocess.PIPE);_,err=proc.communicate(timeout=8)
                    self.assertEqual(proc.returncode,1,err);self.assertFalse(marker.exists())
                finally:os.close(master);os.close(slave)
            OBSERVATIONS['ssh_native_attach'] = {'readonly_enforced':True,'control_input':True,'detach':True,
                                                  'cancelled':True,'tty_and_flags_restored':True,'encoded_name_safe':True}


    def test_real_tmux_attach_with_literal_name_readonly_and_detach(self):
        with a.fixture() as local,a.fixture() as remote,loopback(local,remote) as f:
            host=allow_pin(local,f);remote.start_tmux('/usr/bin/tmux')
            marker=remote.root/'tmux-input';injection=remote.root/'must-not-execute-name'
            receiver=remote.executable('receiver','#!/bin/sh\nwhile IFS= read -r value; do [ "$value" = "REAL_SSH_TMUX_INPUT" ] && printf received > '+shlex.quote(str(marker))+'; done\n')
            remote.run([BINARY,'--backend','tmux','create','ssh_tmux',str(receiver)])
            row=next(s for s in envelope(local,['--backend','tmux','remote','list','fixture'])['data'][0]['sessions'] if s['locator']['name']=='ssh_tmux')
            name='literal; touch '+str(injection)
            remote.run(['tmux','rename-session','-t',row['backend_id'],name])
            args=[BINARY,'--backend','tmux','remote','attach','fixture','--host-id',host,'--name',name,'--session-id',row['backend_id']]
            def attached():return int(remote.run(['tmux','display-message','-p','-t',row['backend_id'],'#{session_attached}']).stdout)>0
            master,slave=pty.openpty();before=termios.tcgetattr(slave)
            try:
                fcntl.ioctl(slave,termios.TIOCSWINSZ,struct.pack('HHHH',30,120,0,0))
                proc=local.start([*args,'--read-only'],stdin=slave,stdout=slave,stderr=subprocess.PIPE);wait_for(attached,timeout=8)
                os.write(master,b'REAL_SSH_TMUX_INPUT\n');time.sleep(.2);self.assertFalse(marker.exists());self.assertFalse(injection.exists())
                proc.terminate();_,err=proc.communicate(timeout=8);self.assertEqual(proc.returncode,130,err);self.assertEqual(termios.tcgetattr(slave),before)
                wait_for(lambda:not attached(),timeout=6)
                proc=local.start(args,stdin=slave,stdout=slave,stderr=subprocess.PIPE);wait_for(attached,timeout=8)
                os.write(master,b'REAL_SSH_TMUX_INPUT\n');wait_for(marker.exists,timeout=5)
                os.write(master,b'\x02d');_,err=proc.communicate(timeout=8);self.assertEqual(proc.returncode,0,err)
                self.assertEqual(termios.tcgetattr(slave),before);self.assertFalse(injection.exists());remote.verify_tmux()
            finally:os.close(master);os.close(slave)
            OBSERVATIONS['ssh_tmux_attach']={'real_pty':True,'literal_shell_metacharacter_name':True,'readonly':True,'control_input':True,'detach_and_tty_restore':True}


@contextmanager
def real_backend(kind):
    with Sandbox() as box:
        for name in ['cache','data','sockets']:(box.root/name).mkdir(mode=0o700)
        box.env['XDG_CACHE_HOME']=str(box.root/'cache');box.env['XDG_DATA_HOME']=str(box.root/'data')
        box.env['SYSTEM_SCREENRC']='/dev/null';box.env['SCREENRC']='/dev/null'
        box.env['ZELLIJ_SOCKET_DIR']=str(box.root/'sockets')
        cfg=box.root/'zellij.kdl';cfg.write_text('default_shell "/bin/sh"\nsession_serialization false\nshow_startup_tips false\nshow_release_notes false\n')
        exe='/usr/bin/screen' if kind=='screen' else str(ZELLIJ)
        prefix=[exe,'-c','/dev/null'] if kind=='screen' else [exe,'--config',str(cfg)]
        box.executable(kind,f'''#!{sys.executable}
import os,sys
from pathlib import Path
root=Path({str(box.root)!r})
assert root.stat().st_uid==os.geteuid() and root.stat().st_mode&0o777==0o700
assert (root/'.owner').read_text()=={box.token!r}
assert os.environ.get({'SCREENDIR' if kind=='screen' else 'ZELLIJ_SOCKET_DIR'!r})=={str(box.root/('screen' if kind=='screen' else 'sockets'))!r}
os.execv({exe!r},{prefix!r}+sys.argv[1:])
''')
        owned=[]
        def sessions():return envelope(box,['--backend',kind,'list'])['data']['sessions']
        def create(name,command=None):
            p=box.run([BINARY,'--backend',kind,'create',name,*(command or ['/bin/sleep','120'])],check=False,timeout=12)
            assert p.returncode==0,(kind,name,p.stdout,p.stderr)
            values=[]
            def visible():
                values[:]=[s for s in sessions() if s['locator']['name']==name]
                return len(values)==1
            wait_for(visible,timeout=8)
            row=values[0]
            if kind=='screen':
                pid=int(row['backend_id'].split('.',1)[0]);entry=box.root/'screen'/row['backend_id']
                assert b'SCREENDIR='+str(box.root/'screen').encode()+b'\0' in Path(f'/proc/{pid}/environ').read_bytes()
            else:
                candidates=list((box.root/'sockets').rglob(name));assert len(candidates)==1,candidates
                entry=candidates[0]
                assert stat.S_ISSOCK(entry.lstat().st_mode) and not entry.is_symlink()
                with socket.socket(socket.AF_UNIX) as c:
                    c.settimeout(2);c.connect(str(entry));pid,uid,_=struct.unpack('3i',c.getsockopt(socket.SOL_SOCKET,socket.SO_PEERCRED,12))
                    assert uid==os.geteuid()
                assert Path(f'/proc/{pid}/exe').resolve()==Path(exe).resolve()
            fd=os.pidfd_open(pid);owned.append((row,fd,entry));return row
        try:
            yield box,create,sessions
        finally:
            for row,fd,entry in reversed(owned):
                poll=select.poll();poll.register(fd,select.POLLIN)
                if not poll.poll(0):
                    box.run([BINARY,'--backend',kind,'delete',row['backend_id']],check=False,timeout=8)
                    try:wait_for(lambda:bool(poll.poll(0)),timeout=5)
                    except TimeoutError:
                        signal.pidfd_send_signal(fd,signal.SIGTERM)
                        wait_for(lambda:bool(poll.poll(0)),timeout=5)
                os.close(fd)


class ExternalBackendChecks(unittest.TestCase):
    def exercise(self,kind):
        with real_backend(kind) as (protected,guard_create,guard_sessions),real_backend(kind) as (box,create,sessions):
            guard=guard_create('guard_session');long=create('foobar')
            self.assertEqual([s['locator']['name'] for s in sessions()],['foobar'])
            missing=box.run([BINARY,'--backend',kind,'delete','foo'],check=False)
            self.assertEqual(missing.returncode,3,(missing.stdout,missing.stderr))
            self.assertEqual(sessions()[0]['backend_id'],long['backend_id'])
            short=create('foo');cap=envelope(box,['--backend',kind,'backend','capabilities'])['data']['backends'][0]
            self.assertTrue(cap['available']);self.assertEqual(cap['capabilities']['session_lifecycle']['level'],'degraded')
            for argv in [['attach','--read-only','foo'],['exec','foo','0','0','MUST_NOT_SEND'],['session','output','foo']]:
                p=box.run([BINARY,'--backend',kind,*argv],check=False)
                self.assertEqual(p.returncode,4,(kind,argv,p.returncode,p.stdout,p.stderr))
            box.run([BINARY,'--backend',kind,'delete',short['backend_id']])
            wait_for(lambda:[s['locator']['name'] for s in sessions()]==['foobar'])
            # The adapter's nuke must stay inside this isolated backend namespace.
            box.run([BINARY,'--backend',kind,'nuke'])
            wait_for(lambda:not sessions())
            self.assertEqual(guard_sessions()[0]['backend_id'],guard['backend_id'])
            OBSERVATIONS.setdefault(kind,{}).update({'version':cap['version'],'empty_listing':True,'create_list_inspect_delete':True,
                'exact_name_collision':True,'typed_identity':True,'unsupported_refusals':True,'nuke_isolation':True,
                'protected_second_namespace_survived':True,'scope':'installed-version basic lifecycle; no pane/layout/read-only capability expansion'})

    def test_actual_attach_input_detach_and_tty_restore(self):
        for kind in ['screen','zellij']:
            if kind=='zellij' and ZELLIJ is None:continue
            with self.subTest(backend=kind),real_backend(kind) as (box,create,sessions):
                marker=box.root/'received';script=box.executable('receiver','#!/bin/sh\nIFS= read -r value\n[ "$value" = "REAL_BACKEND_INPUT" ] && printf received > '+shlex.quote(str(marker))+'\nexec /bin/sleep 120\n')
                row=create('attach_test',[str(script)])
                master,slave=pty.openpty();before=termios.tcgetattr(slave)
                try:
                    fcntl.ioctl(slave,termios.TIOCSWINSZ,struct.pack('HHHH',30,120,0,0))
                    proc=box.start([BINARY,'--backend',kind,'attach',row['backend_id']],stdin=slave,stdout=slave,stderr=subprocess.PIPE)
                    wait_for(lambda: not (termios.tcgetattr(slave)[3]&termios.ICANON),timeout=8)
                    os.write(master,b'REAL_BACKEND_INPUT\n');wait_for(marker.exists,timeout=6)
                    os.write(master,b'\x01d' if kind=='screen' else b'\x0fd')
                    _,err=proc.communicate(timeout=8);self.assertEqual(proc.returncode,0,err)
                    self.assertEqual(termios.tcgetattr(slave),before)
                    self.assertTrue(sessions())
                    OBSERVATIONS.setdefault(kind,{})['real_attach_input_detach_tty']=True
                finally:os.close(master);os.close(slave)

    def test_screen_real_lifecycle_targets_and_namespace_isolation(self):self.exercise('screen')

    def test_zellij_real_lifecycle_targets_and_namespace_isolation(self):
        if ZELLIJ is None:self.skipTest('pinned Zellij executable not provided')
        self.exercise('zellij')


def main():
    global BINARY,REPORT,ZELLIJ
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary',required=True,type=Path);parser.add_argument('--report',required=True,type=Path)
    parser.add_argument('--zellij',type=Path)
    args,remaining=parser.parse_known_args()
    if not (sys.platform.startswith('linux') and os.geteuid()==0 and Path('/.dockerenv').exists()
            and os.environ.get('TXM_LINUX_FIXTURE')=='1'):
        parser.error('requires the explicitly isolated disposable root container; never run on the host')
    BINARY=args.binary.resolve(strict=True);a.BINARY=BINARY;REPORT=args.report
    ZELLIJ=args.zellij.resolve(strict=True) if args.zellij else None
    program=unittest.main(argv=[sys.argv[0],*remaining],exit=False)
    versions={}
    for tool,argv in [('sshd',['/usr/sbin/sshd','-V']),('ssh',['/usr/bin/ssh','-V']),('screen',['/usr/bin/screen','-v']),('tmux',['/usr/bin/tmux','-V'])]:
        p=subprocess.run(argv,capture_output=True,text=True,timeout=5);versions[tool]=(p.stdout+p.stderr).strip()
    REPORT.write_text(json.dumps({'schema_version':1,'passed':program.result.wasSuccessful(),
         'tests_run':program.result.testsRun,'skipped':program.result.skipped,'versions':versions,
         'observations':OBSERVATIONS,'scope':'Disposable Linux amd64 root container, network none/loopback only, readonly host source/binaries; no host accounts or real sessions/SSH config.'},indent=2)+'\n')
    return 0 if program.result.wasSuccessful() else 1


if __name__=='__main__':
    raise SystemExit(main())
