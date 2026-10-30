#!/usr/bin/env python3
"""Real Linux peer credential and installed backend regression checks. Requires a disposable root container, never host root."""
import argparse
import base64
from contextlib import contextmanager
import fcntl
import json
import os
from pathlib import Path
import pty
import select
import shlex
import signal
import socket
import stat
import struct
import subprocess
import sys
import termios
import time
import unittest

import ssh_checks
from ssh_checks import LoopbackSSHChecks
import agent_checks as a
from sandbox import Sandbox, native_server, wait_for
from wire import Peer, packet, HELLO, REQUEST

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
    BINARY=args.binary.resolve(strict=True);a.BINARY=BINARY;ssh_checks.BINARY=BINARY;REPORT=args.report
    ZELLIJ=args.zellij.resolve(strict=True) if args.zellij else None
    program=unittest.main(argv=[sys.argv[0],*remaining],exit=False)
    versions={}
    for tool,argv in [('sshd',['/usr/sbin/sshd','-V']),('ssh',['/usr/bin/ssh','-V']),('screen',['/usr/bin/screen','-v']),('tmux',['/usr/bin/tmux','-V'])]:
        p=subprocess.run(argv,capture_output=True,text=True,timeout=5);versions[tool]=(p.stdout+p.stderr).strip()
    REPORT.write_text(json.dumps({'schema_version':1,'passed':program.result.wasSuccessful(),
         'tests_run':program.result.testsRun,'skipped':program.result.skipped,'versions':versions,
         'observations':{**OBSERVATIONS, **ssh_checks.OBSERVATIONS},'scope':'Disposable Linux amd64 root container, network none/loopback only, readonly host source/binaries; no host accounts or real sessions/SSH config.'},indent=2)+'\n')
    return 0 if program.result.wasSuccessful() else 1


if __name__=='__main__':
    raise SystemExit(main())
