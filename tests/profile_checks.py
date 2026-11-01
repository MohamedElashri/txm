#!/usr/bin/env python3
"""profile preflight, exact placement, ownership and compatibility."""
import argparse
import fcntl
import termios
from tty_state import attributes as tty_attributes
import struct
import json
import os
from pathlib import Path
import shlex
import shutil
import signal
import subprocess
import sys
import time
import unittest
from sandbox import Sandbox,native_server,wait_for
from structured_checks import fake,calls,tmux
from wire import Peer

BINARY=None

def write_profile(box,text,name='profile.toml'):
    path=box.root/name;path.write_text(text);return path

def pane(name,dir,command,split=''):
    result='[[window.pane]]\n'
    for key,value in [('dir',str(dir)),('command',command),('split',split)]:
        if value:result+=key+'='+json.dumps(value)+'\n'
    return result

def inject_wrapper(box,intercept):
    guarded=box.root/'guarded-tmux'
    (box.root/'bin/tmux').rename(guarded)
    log=box.root/'intercepts.jsonl'
    box.executable('tmux',f'''#!{sys.executable}
import json,os,subprocess,sys,time
from pathlib import Path
args=sys.argv[1:]
guarded={str(guarded)!r}
with open({str(log)!r},'a') as f:f.write(json.dumps(args)+'\\n')
{intercept}
os.execv(guarded,[guarded,*args])
''')
    return log

class ProfileChecks(unittest.TestCase):
    def test_strict_schema_limits_and_no_secret_diagnostics(self):
        with Sandbox() as box:
            log=fake(box,'tmux',"raise AssertionError('no backend command expected')")
            texts=['name="s"\nbackend="tmux"\nunknown="PRIVATE_SYNTHETIC"',
                   'name="s"\nbackend="tmux"\n[[window]]\nunknown="PRIVATE_SYNTHETIC"',
                   'name="s"\nbackend="tmux"\n[[window.pane]]\nargv=["PRIVATE_SYNTHETIC"]',
                   'name="s"\nbackend="tmux"\n[[window.pane]]\ndir="missing"',
                   'name="s"\nbackend="tmux"\n[[window]]\nlayout="invalid"',
                   'name="s"\nbackend="tmux"\n[[window.pane]]\nsplit="h"',
                   'name="s"\nbackend="tmux"\n[[window]]\nname="same"\n[[window]]\nname="same"',
                   'name="bad name"\nbackend="tmux"',
                   'name="s"\nbackend="bogus"',
                   'x'*(1048577)]
            for text in texts:
                out=box.run([BINARY,'profile','--detach',write_profile(box,text)],check=False)
                self.assertEqual(out.returncode,2,out.stderr);self.assertEqual(out.stdout,'');self.assertNotIn('PRIVATE_SYNTHETIC',out.stderr)
            fifo=box.root/'profile.fifo';os.mkfifo(fifo)
            self.assertEqual(box.run([BINARY,'profile',fifo],check=False,timeout=2).returncode,2)
            self.assertEqual(calls(log),[])
            self.assertFalse((box.root/'runtime/txm').exists());self.assertFalse((box.root/'state/txm').exists())

    def test_unsupported_shapes_and_backend_conflict_have_zero_commands(self):
        with Sandbox() as box:
            logs=[fake(box,b,"raise AssertionError('no backend command expected')") for b in ('tmux','screen','zellij')]
            for text in ['name="s"\nbackend="screen"','name="s"\nbackend="zellij"',
                         'name="s"\nbackend="native"\n[[window]]\nname="editor"',
                         'name="s"\nbackend="native"\n[[window]]\nlayout="tiled"',
                         'name="s"\nbackend="native"\n[[window.pane]]\n[[window.pane]]']:
                out=box.run([BINARY,'profile','--detach',write_profile(box,text)],check=False)
                self.assertEqual(out.returncode,4,out.stderr)
            text='name="s"\nbackend="native"'
            self.assertEqual(box.run([BINARY,'--backend','tmux','profile','--detach',write_profile(box,text)],check=False).returncode,2)
            self.assertTrue(all(not calls(log) for log in logs));self.assertFalse((box.root/'runtime/txm').exists())

    def test_older_tmux_profile_version_fails_before_mutation(self):
        with Sandbox() as box:
            log=fake(box,'tmux',"if args[0]=='-V':print('tmux 3.3')\nelse:raise AssertionError('no mutation expected')",version=False)
            profile=write_profile(box,'name="s"\nbackend="tmux"')
            self.assertEqual(box.run([BINARY,'profile','--detach',profile],check=False).returncode,4)
            self.assertEqual(calls(log),[['-V']])

    def test_dry_run_is_truthful_private_and_redacted(self):
        with Sandbox() as box:
            box.env['TXM_SESSION_PREFIX']='pre_'
            for backend in ('native','tmux'):
                log=fake(box,'tmux',"raise AssertionError('only version probe allowed')")
                profile=write_profile(box,'name="s"\nbackend='+json.dumps(backend)+'\n[[window.pane]]\ncommand="PRIVATE_SYNTHETIC"')
                out=box.run([BINARY,'profile','--dry-run',profile])
                self.assertIn(backend+'/pre_s',out.stdout);self.assertIn(str(box.root/'repo'),out.stdout)
                self.assertIn('terminal input (omitted)',out.stdout);self.assertNotIn('PRIVATE_SYNTHETIC',out.stdout+out.stderr)
                self.assertTrue(all(c==['-V'] for c in calls(log)))
                self.assertFalse((box.root/'runtime/txm').exists());self.assertFalse((box.root/'state/txm').exists())

    def test_missing_read_only_never_applies(self):
        with Sandbox() as box:
            profile=write_profile(box,'name="s"\nbackend="native"\n[[window.pane]]\ncommand="PRIVATE_SYNTHETIC"')
            self.assertEqual(box.run([BINARY,'profile','-r',profile],check=False).returncode,2)
            out=box.run([BINARY,'profile','--attach-existing','-r',profile],check=False)
            self.assertEqual(out.returncode,3);self.assertFalse(list((box.root/'runtime').rglob('s-*.sock')))
            box.env['TXM_READ_ONLY']='1'
            self.assertEqual(box.run([BINARY,'profile',profile],check=False).returncode,2)
            self.assertFalse((box.root/'state/txm').exists())

    def test_existing_native_is_conflict_or_unchanged_observer(self):
        with Sandbox() as box,native_server(box,BINARY) as (_,sock,_):
            profile=write_profile(box,'name="sample"\nbackend="native"\n[[window.pane]]\ncommand="PRIVATE_SYNTHETIC"')
            self.assertEqual(box.run([BINARY,'profile','--detach',profile],check=False).returncode,5)
            box.run([BINARY,'profile','--detach','--attach-existing',profile])
            master,slave=os.openpty();proc=None
            try:
                proc=box.start([BINARY,'profile','--attach-existing','-r',profile],stdin=slave,stdout=slave,stderr=slave)
                def attached():
                    with Peer(sock) as p:return p.request('status')['attached_clients']==1
                wait_for(attached);os.write(master,b'\x1c')
                wait_for(lambda:proc.poll() is not None,timeout=4)
                self.assertEqual(proc.returncode,0)
                with Peer(sock) as p:
                    import base64
                    content=base64.b64decode(p.request('snapshot',format='plain',lines=100,max_bytes=1048576)['content'])
                    self.assertNotIn(b'PRIVATE_SYNTHETIC',content)
            finally:
                if proc and proc.poll() is None:
                    proc.terminate()
                    try:wait_for(lambda:proc.poll() is not None,timeout=4)
                    except TimeoutError:proc.kill();proc.wait(timeout=4)
                os.close(master);os.close(slave)

    def test_native_single_terminal_cwd_prefix_and_no_host_state(self):
        with Sandbox() as box:
            cwd=box.root/'cwd with spaces';cwd.mkdir();marker=box.root/'native-result'
            box.env['TXM_SESSION_PREFIX']='pre_'
            text='name="native"\nbackend="native"\n'+pane('',cwd,'pwd -P > '+shlex.quote(str(marker))+'; exec /bin/sleep 30')
            profile=write_profile(box,text)
            try:
                result=box.run([BINARY,'profile','--detach',profile],timeout=8,check=False)
                self.assertEqual(result.returncode,0,(result.stdout[-2000:],result.stderr[-2000:]))
                wait_for(marker.exists)
                self.assertEqual(marker.read_text().strip(),str(cwd));self.assertFalse((box.root/'state/txm').exists())
                self.assertIn('pre_native',box.run([BINARY,'list']).stdout)
                self.assertEqual(box.run([BINARY,'profile','--detach',profile],check=False).returncode,5)
            finally:
                box.run([BINARY,'delete','native/pre_native'],check=False,timeout=8)

    def test_picker_enter_attach_detach_restores_tty(self):
        with Sandbox() as box,native_server(box,BINARY) as (_,sock,_):
            master,slave=os.openpty();fcntl.ioctl(slave,termios.TIOCSWINSZ,struct.pack('HHHH',24,80,0,0));old=tty_attributes(slave)
            # A real terminal's shell remains its session leader while txm exits.
            # Darwin revokes the PTY if the session leader exits, so hold an owned
            # wrapper until configuration has been checked and drain output.
            done=box.root/'picker-done';release=box.root/'picker-release';stop=box.root/'picker-stop'
            wrapper=box.root/'picker-wrapper.py'
            wrapper.write_text(f"import subprocess,time\nfrom pathlib import Path\np=subprocess.Popen({[str(BINARY)]!r})\nwhile p.poll() is None:\n if Path({str(stop)!r}).exists():p.kill()\n time.sleep(.01)\npath=Path({str(done)!r});tmp=path.with_suffix('.tmp');tmp.write_text(str(p.wait()));tmp.replace(path)\nwhile not Path({str(release)!r}).exists():time.sleep(.01)\n")
            proc=box.start([sys.executable,wrapper],stdin=slave,stdout=slave,stderr=slave,preexec_fn=lambda:fcntl.ioctl(0,termios.TIOCSCTTY,0))
            os.set_blocking(master,False);rendered=bytearray()
            def visible():
                try:rendered.extend(os.read(master,65536))
                except BlockingIOError:pass
                return b'sample' in rendered
            try:
                wait_for(visible);os.write(master,b'\r')
                def attached():
                    visible()
                    with Peer(sock) as peer:return peer.request('status')['attached_clients']==1
                wait_for(attached);os.write(master,b'\x1c')
                wait_for(lambda:(visible(),done.exists())[1],timeout=4)
                self.assertEqual(int(done.read_text()),0)
                self.assertEqual(tty_attributes(slave),old)
            finally:
                if not done.exists():
                    stop.touch();wait_for(lambda:(visible(),done.exists())[1],timeout=4)
                release.touch();proc.wait(timeout=4)
                os.close(master);os.close(slave)

    @unittest.skipUnless(shutil.which('tmux'),'tmux unavailable')
    def test_real_tmux_exact_ids_cwd_layout_and_focus(self):
        for base in (0,1):
            with self.subTest(base=base),Sandbox() as box:
                box.start_tmux(shutil.which('tmux'));tmux(box,'set-option','-g','base-index',str(base));tmux(box,'set-window-option','-g','pane-base-index',str(base))
                guardpane=tmux(box,'list-panes','-t','='+box.guard_name,'-F','#{pane_id}')
                tmux(box,'set-hook','-g','after-new-window','select-pane -t '+guardpane)
                tmux(box,'set-hook','-g','after-split-window','select-pane -t '+guardpane)
                tmux(box,'set-window-option','-g','synchronize-panes','on')
                tmux(box,'set-option','-g','default-command','exec /bin/sleep 120')
                box.env['TXM_SESSION_PREFIX']='pre_'
                dirs=[box.root/"dir space #{HOME} $x 'quote';",box.root/'dir second',box.root/'dir third;']
                for d in dirs:d.mkdir()
                text='name="profile"\nbackend="tmux"\n[[window]]\nname="editor"\nlayout="even-horizontal"\n'
                markers=[]
                for i,d in enumerate(dirs):
                    if i==2:text+='[[window]]\nname="server"\nlayout="tiled"\n'
                    marker=box.root/f'pane-{i}.cwd';markers.append(marker)
                    command=f"printf 'PROFILE_SENTINEL_{i}\\n'; pwd -P > "+shlex.quote(str(marker))
                    text+=pane('',d,command,'v' if i==1 else '')
                profile=write_profile(box,text)
                box.run([BINARY,'profile','--detach',profile],timeout=20)
                for marker,d in zip(markers,dirs):wait_for(marker.exists);self.assertEqual(marker.read_text().strip(),str(d))
                windows=tmux(box,'list-windows','-t','=pre_profile','-F','#{window_index}|#{window_name}|#{window_id}').splitlines()
                self.assertEqual([int(w.split('|')[0]) for w in windows],[base,base+1])
                self.assertEqual([w.split('|')[1] for w in windows],['editor','server'])
                for wi,w in enumerate(windows):
                    if wi==0:
                        heights=tmux(box,'list-panes','-t',w.split('|')[2],'-F','#{pane_height}').splitlines()
                        self.assertEqual(len(set(heights)),1,'even-horizontal layout was not applied')
                    rows=tmux(box,'list-panes','-t',w.split('|')[2],'-F','#{pane_id}|#{pane_index}|#{synchronize-panes}').splitlines()
                    self.assertEqual([int(r.split('|')[1]) for r in rows],list(range(base,base+len(rows))))
                    for pi,row in enumerate(rows):
                        pid,index,sync=row.split('|');self.assertEqual(sync,'0')
                        number=pi if wi==0 else 2
                        content=tmux(box,'capture-pane','-p','-t',pid)
                        self.assertIn('PROFILE_SENTINEL_'+str(number),content)
                        for other in set(range(3))-{number}:self.assertNotIn('PROFILE_SENTINEL_'+str(other),content)
                guard=tmux(box,'capture-pane','-p','-t',guardpane)
                self.assertNotIn('PROFILE_SENTINEL',guard)
                self.assertFalse((box.root/'state/txm').exists());box.verify_tmux()

    @unittest.skipUnless(shutil.which('tmux'),'tmux unavailable')
    def test_tmux_default_attach_and_existing_policy(self):
        with Sandbox() as box:
            box.start_tmux(shutil.which('tmux'))
            profile=write_profile(box,'name="profile"\nbackend="tmux"')
            master,slave=os.openpty();fcntl.ioctl(slave,termios.TIOCSWINSZ,struct.pack('HHHH',24,80,0,0));old=tty_attributes(slave)
            proc=None
            try:
                proc=box.start([BINARY,'profile',profile],stdin=slave,stdout=slave,stderr=slave)
                wait_for(lambda:tmux(box,'list-clients','-F','#{session_name}')=='profile')
                os.write(master,b'\x02d');self.assertEqual(proc.wait(timeout=5),0);self.assertEqual(tty_attributes(slave),old)
            finally:
                if proc and proc.poll() is None:proc.terminate();proc.wait(timeout=5)
                os.close(master);os.close(slave)
            self.assertEqual(box.run([BINARY,'profile','--detach',profile],check=False).returncode,5)
            original=tmux(box,'list-panes','-t','=profile','-F','#{pane_id}')
            marker=box.root/'not-reapplied'
            profile.write_text('name="profile"\nbackend="tmux"\n'+pane('',box.root/'repo','touch '+shlex.quote(str(marker))))
            box.run([BINARY,'profile','--detach','--attach-existing',profile])
            self.assertEqual(tmux(box,'list-panes','-t','=profile','-F','#{pane_id}'),original)
            self.assertFalse(marker.exists())
            box.verify_tmux()

    @unittest.skipUnless(shutil.which('tmux'),'tmux unavailable')
    def test_exact_input_refuses_synchronization(self):
        with Sandbox() as box:
            box.start_tmux(shutil.which('tmux'));tmux(box,'new-session','-d','-s','input','exec /bin/sleep 120')
            ids=tmux(box,'list-panes','-t','=input','-F','#{window_id}|#{pane_id}').split('|');tmux(box,'set-option','-p','-t',ids[1],'synchronize-panes','on')
            out=box.run([BINARY,'exec','input',*ids,'PRIVATE_SYNTHETIC'],check=False)
            self.assertEqual(out.returncode,4);self.assertNotIn('PRIVATE_SYNTHETIC',tmux(box,'capture-pane','-p','-t',ids[1]))

    @unittest.skipUnless(shutil.which('tmux'),'tmux unavailable')
    def test_input_rechecks_synchronization_in_server_queue(self):
        for profile_mode in (False,True):
            with Sandbox() as box:
                box.start_tmux(shutil.which('tmux'))
                if not profile_mode:
                    tmux(box,'new-session','-d','-s','input','/bin/sh','-i')
                    ids=tmux(box,'list-panes','-t','=input','-F','#{window_id}|#{pane_id}').split('|')
                marker=box.root/'must-not-run'
                command='printf unsafe > '+shlex.quote(str(marker))
                inject_wrapper(box,"""if args[0]=='if-shell' and 'send-keys' in args[-2]:
 subprocess.check_call([guarded,'set-option','-p','-t',args[3],'synchronize-panes','on'])
""")
                if profile_mode:
                    spec=write_profile(box,'name="profile"\nbackend="tmux"\n'+pane('',box.root/'repo',command))
                    out=box.run([BINARY,'profile','--detach',spec],check=False)
                    self.assertIn('no owned session survives',out.stderr)
                else:
                    out=box.run([BINARY,'exec','input',*ids,command],check=False)
                    self.assertNotIn('unsafe',tmux(box,'capture-pane','-p','-t',ids[1]))
                self.assertNotEqual(out.returncode,0,out.stderr)
                self.assertFalse(marker.exists());box.verify_tmux()

    @unittest.skipUnless(shutil.which('tmux'),'tmux unavailable')
    def test_failed_layout_rolls_back_only_owned_session_before_input(self):
        with Sandbox() as box:
            box.start_tmux(shutil.which('tmux'));tmux(box,'new-session','-d','-s','profile_extra','exec /bin/sleep 120')
            marker=box.root/'forbidden'
            profile=write_profile(box,'name="profile"\nbackend="tmux"\n[[window]]\nlayout="tiled"\n'+pane('',box.root/'repo','touch '+shlex.quote(str(marker))))
            inject_wrapper(box,"if args[0]=='if-shell' and 'select-layout' in args[-2]: sys.exit(1)")
            out=box.run([BINARY,'profile','--detach',profile],check=False,timeout=15)
            self.assertEqual(out.returncode,1,out.stderr);self.assertIn('no owned session survives',out.stderr)
            self.assertNotIn('profile\n',tmux(box,'list-sessions','-F','#{session_name}')+'\n')
            self.assertFalse(marker.exists());self.assertIn('profile_extra',tmux(box,'list-sessions','-F','#{session_name}'));box.verify_tmux()

    @unittest.skipUnless(shutil.which('tmux'),'tmux unavailable')
    def test_lost_ownership_refuses_rollback_and_reports_survivor(self):
        with Sandbox() as box:
            box.start_tmux(shutil.which('tmux'))
            profile=write_profile(box,'name="profile"\nbackend="tmux"\n[[window]]\nlayout="tiled"')
            inject_wrapper(box,"""if args[0]=='if-shell' and 'select-layout' in args[-2]:
 target=args[args.index('-t')+1].split(':')[0]
 subprocess.run([guarded,'set-environment','-t',target,'TXM_PROFILE_OWNER','replaced'],check=True)
 sys.exit(1)
""")
            out=box.run([BINARY,'profile','--detach',profile],check=False,timeout=15)
            self.assertEqual(out.returncode,7,out.stderr);self.assertIn('tmux/profile',out.stderr)
            self.assertIn('profile',tmux(box,'list-sessions','-F','#{session_name}'));box.verify_tmux()

    @unittest.skipUnless(shutil.which('tmux'),'tmux unavailable')
    def test_malformed_creation_receipt_never_deletes_by_name(self):
        with Sandbox() as box:
            box.start_tmux(shutil.which('tmux'))
            profile=write_profile(box,'name="profile"\nbackend="tmux"')
            log=inject_wrapper(box,"""if args[0]=='new-session':
 subprocess.run([guarded,*args],check=True,stdout=subprocess.DEVNULL)
 print('malformed receipt');sys.exit(0)
""")
            out=box.run([BINARY,'profile','--detach',profile],check=False)
            self.assertEqual(out.returncode,7);self.assertIn('profile',tmux(box,'list-sessions','-F','#{session_name}'))
            self.assertFalse(any('kill-session' in ' '.join(c) for c in calls(log)));box.verify_tmux()

    @unittest.skipUnless(shutil.which('tmux'),'tmux unavailable')
    def test_cancellation_rolls_back_with_fresh_cleanup_context(self):
        with Sandbox() as box:
            box.start_tmux(shutil.which('tmux'));started=box.root/'layout-started'
            profile=write_profile(box,'name="profile"\nbackend="tmux"\n[[window]]\nlayout="tiled"')
            inject_wrapper(box,f"""if args[0]=='if-shell' and 'select-layout' in args[-2]:
 Path({str(started)!r}).touch();os.execv('/bin/sleep',['sleep','120'])
""")
            proc=box.start([BINARY,'profile','--detach',profile],stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
            wait_for(started.exists);proc.send_signal(signal.SIGTERM);stdout,stderr=proc.communicate(timeout=15)
            self.assertEqual(proc.returncode,130,(stdout,stderr));self.assertIn('no owned session survives',stderr)
            self.assertNotIn('profile\n',tmux(box,'list-sessions','-F','#{session_name}')+'\n');box.verify_tmux()

if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--binary',type=Path,required=True);p.add_argument('tests',nargs='*');args=p.parse_args();BINARY=args.binary.resolve(strict=True)
    unittest.main(argv=[sys.argv[0],*args.tests])
