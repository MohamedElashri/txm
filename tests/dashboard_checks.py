#!/usr/bin/env python3
"""Phase 6 real PTYs rendered by the existing native VT, never a new parser."""
import argparse
import base64
from contextlib import contextmanager
import json
import os
from pathlib import Path
import secrets
import shutil
import signal
import struct
import subprocess
import sys
import time
import unittest

import agent_checks as a
from agent_events_checks import signal_request
from sandbox import native_server,wait_for
from structured_checks import fake,calls,tmux
from wire import Peer,packet,read,RESIZE

BINARY=None
REPORT=None
METRICS={}

@contextmanager
def fixture():
    with a.fixture() as box:
        # Race instrumentation stays enabled; omit only Go's exit sleep.
        box.env['GORACE']+=' atexit_sleep_ms=0'
        yield box

class Console:
    def __init__(self,box,sock,result,ready,wrapper,server):
        self.box,self.sock,self.result,self.ready=box,sock,result,ready
        self.wrapper,self.server=wrapper,server
        self.last=''
    def snapshot(self):
        with Peer(self.sock) as peer:
            data=peer.request('snapshot',format='plain',lines=100,max_bytes=65536)
        self.last=base64.b64decode(data['content']).decode();return self.last
    def wait(self,text,timeout=6):
        try:wait_for(lambda:text in self.snapshot(),timeout=timeout)
        except Exception:raise AssertionError('missing '+repr(text)+' in '+repr(self.last))
        return self.last
    def keys(self,data):
        with Peer(self.sock,'control') as peer:peer.request('input',data=base64.b64encode(data).decode())
    def resize(self,cols,rows):
        with Peer(self.sock,'control') as peer:
            peer.socket.sendall(packet(RESIZE,2,struct.pack('!HH',cols,rows)))
            assert json.loads(read(peer.socket)[2])['error'] is None
    def search(self,query):
        self.keys(b'/\x15'+query.encode()+b'\r');self.wait('Search /'+query)
    def exited(self,code=0,timeout=6):
        wait_for(self.result.exists,timeout=timeout);result=json.loads(self.result.read_text())
        assert result['code']==code,result
        assert result['term_before']==result['term_after'],result
        assert result['flags_before']==result['flags_after'],result
        return result
    def send_signal(self,sig):
        wait_for(self.ready.exists);pid=json.loads(self.ready.read_text())['pid']
        assert f'PPid:\t{self.wrapper}\n' in Path(f'/proc/{pid}/status').read_text()
        assert b'dashboard\0' in Path(f'/proc/{pid}/cmdline').read_bytes()
        fd=os.pidfd_open(pid)
        try:signal.pidfd_send_signal(fd,sig)
        finally:os.close(fd)

@contextmanager
def console(box,args=()):
    token=secrets.token_hex(4);script=box.root/('console-'+token+'.py');result=box.root/('result-'+token+'.json');ready=box.root/('ready-'+token+'.json')
    script.write_text(f'''import fcntl,json,os,subprocess,termios
from pathlib import Path
before=termios.tcgetattr(0);flags=[fcntl.fcntl(fd,fcntl.F_GETFL) for fd in (0,1)]
p=subprocess.Popen({[str(BINARY),'dashboard',*map(str,args)]!r})
Path({str(ready)!r}).write_text(json.dumps({{"pid":p.pid}}))
code=p.wait()
value={{"code":code,"term_before":before,"term_after":termios.tcgetattr(0),"flags_before":flags,"flags_after":[fcntl.fcntl(fd,fcntl.F_GETFL) for fd in (0,1)]}}
p=Path({str(result)!r});tmp=p.with_suffix('.tmp');tmp.write_text(json.dumps(value,default=lambda b:b.hex()));tmp.replace(p)
''')
    with native_server(box,BINARY,name='console_'+token,argv=[sys.executable,str(script)]) as (server,sock,wrapper):
        ui=Console(box,sock,result,ready,wrapper,server)
        try:
            ui.resize(160,50);ui.wait('txm dashboard')
            yield ui
        finally:
            if not result.exists() and sock.exists():
                # A control-C is delivered only to this fixture's own terminal.
                try:ui.keys(b'\x03');ui.exited(130)
                except (FileNotFoundError,ConnectionRefusedError,EOFError):pass

class DashboardChecks(unittest.TestCase):
    def test_non_tty_and_invalid_options_never_access_backends_or_agents(self):
        with fixture() as box:
            logs=[fake(box,name,"raise AssertionError('must not query')",version=False) for name in ('tmux','screen','zellij')]
            for args in [[],['--agents'],['--preview'],['--refresh','0s'],['--backend','bad'],['--all-backends','--backend','native']]:
                p=box.run([BINARY,'dashboard',*args],check=False)
                self.assertEqual(p.returncode,2,(p.stdout,p.stderr));self.assertEqual(p.stdout,'')
            self.assertTrue(all(not calls(log) for log in logs))
            self.assertFalse((box.root/'state/txm').exists());self.assertFalse(a.runtime(box).exists())

    def test_native_refresh_search_preview_resize_and_quit_restore_tty(self):
        with fixture() as box,native_server(box,BINARY,name='work',argv=['/bin/sh','-c','printf "SAFE_PREVIEW\\n\\033]52;c;PRIVATE_ESCAPE\\007";exec /bin/sleep 120']):
            with console(box,['--backend','native']) as ui:
                ui.search('work');ui.wait('authenticated status');self.assertNotIn('SAFE_PREVIEW',ui.snapshot())
                ui.keys(b'p');view=ui.wait('SAFE_PREVIEW');self.assertNotIn('PRIVATE_ESCAPE',view)
                self.assertIn('screen_snapshot',view);self.assertIn('output_history=unsupported',view)
                for width,height in [(72,18),(24,6),(200,70),(160,50)]:
                    ui.resize(width,height);ui.wait('txm dashboard')
                ui.keys(b'r');ui.wait('Refresh requested')
                ui.keys(b'q');ui.exited()
            self.assertFalse(a.runtime(box).exists());self.assertFalse(a.state(box).exists())

    def test_native_readonly_and_control_attach_repeat_without_input_leaks(self):
        with fixture() as box:
            script=box.root/'reader.py';received=box.root/'received'
            script.write_text(f'import os\nfrom pathlib import Path\nwhile True:\n data=os.read(0,4096)\n if not data:break\n with Path({str(received)!r}).open("ab") as output:output.write(data)\n')
            with native_server(box,BINARY,name='work',argv=[sys.executable,str(script)]) as (_,sock,_),console(box,['--backend','native']) as ui:
                ui.search('work');ui.wait('running')
                def attached():
                    with Peer(sock) as peer:return peer.request('status')['attached_clients']==1
                for _ in range(3):
                    ui.keys(b'o');wait_for(attached);ui.keys(b'FORBIDDEN\r');time.sleep(.06)
                    self.assertFalse(received.exists());ui.keys(b'\x1c');ui.wait('Detached; dashboard resumed');ui.wait('running')
                ui.keys(b'\r');wait_for(attached);ui.keys(b'ALLOWED\r');wait_for(received.exists)
                self.assertIn(b'ALLOWED',received.read_bytes());ui.keys(b'\x1c');ui.wait('Detached; dashboard resumed');ui.keys(b'q');ui.exited()

    @unittest.skipUnless(shutil.which('tmux'),'tmux unavailable')
    def test_real_tmux_readonly_control_and_name_reuse(self):
        with fixture() as box:
            box.start_tmux(shutil.which('tmux'));tmux(box,'new-session','-d','-s','work','exec /bin/sleep 120')
            with console(box,['--backend','tmux']) as ui:
                ui.search('work');ui.wait('read_only_attach=supported')
                for key,readonly in [(b'o','1'),(b'\r','0')]:
                    ui.keys(key);wait_for(lambda:tmux(box,'list-clients','-F','#{client_readonly}')==readonly)
                    ui.wait('[work]');ui.keys(b'\x02d');ui.wait('Detached; dashboard resumed')
                tmux(box,'rename-session','-t','=work','renamed');ui.keys(b'\x1b');time.sleep(.08)
                # Leave search, then refresh using the new name, never resurrect work.
                ui.search('renamed');ui.keys(b'r');ui.wait('renamed');ui.keys(b'q');ui.exited()
                self.assertNotIn('work',tmux(box,'list-sessions','-F','#{session_name}'));box.verify_tmux()

    @unittest.skipUnless(sys.platform.startswith('linux') and hasattr(os,'pidfd_open'),'Linux fixture PID ownership')
    def test_cancellation_during_native_and_tmux_attach_restores_tty(self):
        with fixture() as box,native_server(box,BINARY,name='work') as (_,sock,_),console(box,['--backend','native']) as ui:
            ui.search('work');ui.wait('running');ui.keys(b'\r')
            def attached():
                with Peer(sock) as peer:return peer.request('status')['attached_clients']==1
            wait_for(attached);ui.send_signal(signal.SIGTERM);ui.exited(130)
            with Peer(sock) as peer:self.assertEqual(peer.request('status')['process_state'],'running')
        if shutil.which('tmux'):
            with fixture() as box:
                box.start_tmux(shutil.which('tmux'));tmux(box,'new-session','-d','-s','work','exec /bin/sleep 120')
                with console(box,['--backend','tmux']) as ui:
                    ui.search('work');ui.wait('read_only_attach=supported');ui.keys(b'\r');ui.wait('[work]')
                    ui.send_signal(signal.SIGTERM);ui.exited(130)
                    self.assertIn('work',tmux(box,'list-sessions','-F','#{session_name}'));box.verify_tmux()

    def test_slow_unavailable_backends_do_not_block_native_or_cancellation(self):
        with fixture() as box:
            tmux_log=fake(box,'tmux',"if args[0]=='list-sessions':time.sleep(10)")
            screen_log=fake(box,'screen',"sys.exit(1)",version=False)
            zellij_log=fake(box,'zellij',"print('fixture');sys.exit(0)",version=False)
            with console(box) as ui:
                ui.wait('console_');ui.wait('screen:operation_failed');ui.wait('tmux:timeout',timeout=5)
                started=time.monotonic();ui.keys(b'/does-not-exist\r');ui.wait('No matching observations.');elapsed=(time.monotonic()-started)*1000
                self.assertLess(elapsed,500);METRICS['slow_backend_search_ms']=elapsed
                started=time.monotonic();ui.keys(b'q');ui.exited();self.assertLess(time.monotonic()-started,3)
            before=len(calls(tmux_log));time.sleep(.55);self.assertEqual(len(calls(tmux_log)),before)
            self.assertTrue(calls(screen_log));self.assertTrue(calls(zellij_log));self.assertFalse(a.runtime(box).exists())

    def test_unsupported_preview_and_attach_are_ui_errors_without_side_effects(self):
        with fixture() as box:
            log=fake(box,'screen',"print('1234.sample (Detached)')",version=False)
            with console(box,['--backend','screen','--preview']) as ui:
                ui.wait('sample');ui.keys(b'o');ui.wait('dashboard actions are verified only for native/tmux')
                ui.keys(b'\r');ui.wait('Preview unavailable:');self.assertNotIn('screen_snapshot',ui.snapshot())
                ui.keys(b'q');ui.exited()
            self.assertTrue(all(call==['-ls'] for call in calls(log)),calls(log))

    def test_current_native_terminal_cannot_attach_or_preview_itself(self):
        with fixture() as box,console(box,['--backend','native']) as ui:
            ui.search('console_');ui.wait('read_only_attach=supported')
            ui.keys(b'\r');ui.wait('feedback loop')
            ui.keys(b'p');ui.wait('Preview unavailable: current native terminal')
            with Peer(ui.sock) as peer:self.assertEqual(peer.request('status')['attached_clients'],0)
            ui.keys(b'q');ui.exited()

    def test_stale_native_selection_cannot_attach_reused_name(self):
        with fixture() as box,native_server(box,BINARY,name='work') as (_,old_socket,_),console(box,['--backend','native','--refresh','1m']) as ui:
            ui.search('work');ui.wait('read_only_attach=supported')
            with Peer(old_socket,'control') as peer:peer.request('kill')
            wait_for(lambda:not old_socket.exists())
            with native_server(box,BINARY,name='work') as (_,new_socket,_):
                self.assertNotEqual(new_socket,old_socket)
                ui.keys(b'o');ui.wait('Attach failed: conflict')
                with Peer(new_socket) as peer:self.assertEqual(peer.request('status')['attached_clients'],0)
                ui.keys(b'q');ui.exited()

    def test_agent_subscription_shows_sources_and_separate_process_attention_turn(self):
        with fixture() as box,a.coordinator(box):
            live=a.start(box,'worker',['/bin/sleep','120'],extra=['--provider','generic'])
            done=a.start(box,'finished',['/bin/sh','-c','exit 42']);a.await_exit(box,done['id'])
            with console(box,['--backend','native','--agents']) as ui:
                ui.search('worker');ui.keys(b'\t\t');ui.wait('managed (supervised)')
                self.assertIsNone(signal_request(box,live,1,'needs-input')['error']);view=ui.wait('needs_input')
                self.assertIn('hook:generic',view);self.assertIn('running',view)
                signal_request(box,live,2,'turn-complete');ui.wait('turn complete: 20')
                ui.keys(b'\x1b');time.sleep(.1);ui.search('finished');ui.wait('code=42');self.assertIn('exited',ui.snapshot())
                ui.keys(b'q');ui.exited();self.assertEqual(a.status(box,live['id'])['lifecycle'],'running')

    def test_unchanged_running_agent_gets_fresh_typed_proof(self):
        with fixture() as box,a.coordinator(box):
            a.start(box,'worker',['/bin/sleep','120'],extra=['--provider','generic'])
            with console(box,['--backend','native','--agents']) as ui:
                ui.search('worker');ui.keys(b'\t\t');ui.wait('managed (supervised)')
                initial=ui.snapshot();stamp=next(line.split('observed ',1)[1].strip() for line in initial.splitlines() if line.startswith('Process:'))
                def refreshed():
                    value=ui.snapshot()
                    return 'Process: running' in value and 'unknown stale' not in value and any(line.startswith('Process:') and stamp not in line for line in value.splitlines())
                wait_for(refreshed,timeout=5)
                time.sleep(5.2);self.assertIn('Process: running',ui.snapshot());self.assertNotIn('unknown stale',ui.snapshot())
                ui.keys(b'q');ui.exited()

    def test_agent_disconnect_is_stale_then_epoch_resync_without_relaunch(self):
        with fixture() as box:
            with a.coordinator(box) as service:
                live=a.start(box,'worker',['/bin/sleep','120'],extra=['--provider','generic'])
                with console(box,['--backend','native','--agents']) as ui:
                    ui.search('worker');ui.keys(b'\t\t');ui.wait('managed (supervised)')
                    service.send_signal(signal.SIGSTOP)
                    try:ui.wait('agents stale / unavailable',timeout=8);self.assertIn('unknown stale',ui.snapshot())
                    finally:service.send_signal(signal.SIGCONT)
                    ui.wait('agents subscribed');ui.wait('running')
                    ui.keys(b'q');ui.exited()
            with a.coordinator(box):self.assertEqual(a.status(box,live['id'])['lifecycle'],'running')

    @unittest.skipUnless(sys.platform.startswith('linux') and hasattr(os,'pidfd_open'),'Linux fixture PID ownership')
    def test_sigint_sigterm_and_raw_ctrl_c_restore_tty(self):
        with fixture() as box:
            for sig in (signal.SIGINT,signal.SIGTERM,None):
                with console(box,['--backend','native']) as ui:
                    ui.wait('console_')
                    if sig is None:ui.keys(b'\x03')
                    else:ui.send_signal(sig)
                    ui.exited(130)

    def test_thousand_sessions_search_resize_and_quit_budget(self):
        with fixture() as box:
            rows='\n'.join(f'${i+1}|task-{i:04d}|0' for i in range(1000))
            fake(box,'tmux',f"if args[0]=='list-sessions':print({rows!r})")
            with console(box,['--backend','tmux']) as ui:
                ui.wait('tmux:1000');started=time.monotonic();ui.search('task-0999');ui.wait('1 rows');METRICS['search_1000_ms']=(time.monotonic()-started)*1000
                self.assertLess(METRICS['search_1000_ms'],500)
                started=time.monotonic();ui.resize(120,35);ui.wait('task-0999');METRICS['resize_1000_ms']=(time.monotonic()-started)*1000;self.assertLess(METRICS['resize_1000_ms'],500)
                if sys.platform.startswith('linux'):
                    pid=json.loads(ui.ready.read_text())['pid'];rss=next(int(line.split()[1]) for line in Path(f'/proc/{pid}/status').read_text().splitlines() if line.startswith('VmRSS:'));METRICS['rss_1000_kib']=rss
                    if 'race' not in str(BINARY):self.assertLess(rss,65536)
                started=time.monotonic();ui.keys(b'q');ui.exited();METRICS['quit_1000_ms']=(time.monotonic()-started)*1000;self.assertLess(METRICS['quit_1000_ms'],500)

if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--binary',type=Path,required=True);p.add_argument('--report',type=Path)
    args,remaining=p.parse_known_args();BINARY=args.binary.resolve(strict=True);a.BINARY=BINARY;REPORT=args.report
    try:result=unittest.main(argv=[sys.argv[0],*remaining],exit=False).result
    finally:
        if REPORT:REPORT.write_text(json.dumps({'scope':'guarded Linux native VT/PTY dashboard fixtures; race RSS is diagnostic only','budgets':{'input_resize_quit_ms':500,'rss_1000_kib':65536,'slow_cancel_s':3},'metrics':METRICS},indent=2)+'\n')
    raise SystemExit(0 if result.wasSuccessful() else 1)
