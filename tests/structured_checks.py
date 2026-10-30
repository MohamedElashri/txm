#!/usr/bin/env python3
"""Phase 2 contracts, isolated real tmux/native and bounded fake queries."""
import argparse
import base64
import concurrent.futures
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import time
import unittest
from sandbox import Sandbox, native_server, wait_for

BINARY = None

def fake(box, backend, body, version=True):
    if backend=='tmux' and version:
        body="if args[0]=='-V': print('tmux 3.4');sys.exit(0)\n"+body
    log=box.root/(backend+'-calls.jsonl')
    box.executable(backend, f'''#!{sys.executable}
import json,sys,time,os
from pathlib import Path
args=sys.argv[1:]
with open({str(log)!r},'a') as f:f.write(json.dumps(args)+'\\n')
{body}
''')
    return log

def fake_tmux(box, rows='$1|sample|0'):
    return fake(box,'tmux',f'''if args[0]=='list-sessions': print({rows!r})
elif args[0]=='-V': print('tmux 3.4')
''')

def calls(log):
    return [json.loads(line) for line in log.read_text().splitlines()] if log.exists() else []

def tmux(box,*args):
    return box.run([box.tmux,'-S',box.socket,*args]).stdout.strip()

class StructuredChecks(unittest.TestCase):
    def envelope(self, result, code=0):
        self.assertEqual(result.returncode,code,(result.stdout,result.stderr))
        data=json.loads(result.stdout)
        self.assertEqual(set(data),{'schema_version','data','errors','partial'})
        self.assertEqual(data['schema_version'],1)
        self.assertIsInstance(data['errors'],list)
        self.assertIsInstance(data['partial'],bool)
        for error in data['errors']:
            self.assertEqual(set(error),{'code','message','backend','locator','retryable'})
            self.assertIsInstance(error['retryable'],bool)
        return data

    def test_empty_identity_and_argument_error_goldens(self):
        with Sandbox() as box:
            for argv in (['session','inspect','--json'],['session','output','sample','--lines','0','--json'],
                         ['list','--backend','bad','--json'],['list','--wat','--json'],
                         ['list','--all-backends','--backend','native','--json'],['backend','--json'],['session','--json'],['window','--json']):
                d=self.envelope(box.run([BINARY,*argv],check=False),2)
                self.assertIsNone(d['data']);self.assertFalse(d['partial'])
                self.assertEqual(d['errors'][0]['code'],'invalid_usage')
                self.assertFalse((box.root/'state/txm').exists(),'invalid input wrote identity')
            empty=self.envelope(box.run([BINARY,'list','--json']))
            identity=empty['data']['host_id'];self.assertRegex(identity,r'^[a-f0-9]{32}$')
            self.assertEqual(empty['data']['sessions'],[])
            self.assertEqual(empty['errors'],[])
            again=self.envelope(box.run([BINARY,'session','list','--json']))
            self.assertEqual(again['data']['host_id'],identity)
            path=box.root/'state/txm/v1/host.json';self.assertEqual(path.stat().st_mode&0o777,0o600)
            path.write_text('{broken')
            d=self.envelope(box.run([BINARY,'list','--json'],check=False),1)
            self.assertEqual(d['errors'][0]['code'],'invalid_config');self.assertEqual(path.read_text(),'{broken')

    def test_concurrent_identity_initialization(self):
        with Sandbox() as box, concurrent.futures.ThreadPoolExecutor(max_workers=8) as pool:
            results=list(pool.map(lambda _:box.run([BINARY,'list','--json']),range(16)))
            ids={self.envelope(r)['data']['host_id'] for r in results}
            self.assertEqual(len(ids),1)
            self.assertEqual(list((box.root/'state/txm/v1').glob('.host-*')),[])

    def test_strict_selection_and_qualified_prefix(self):
        with Sandbox() as box, native_server(box,BINARY,name='pre_sample') as (server,_,_):
            box.env['TXM_SESSION_PREFIX']='pre_'
            for target in ('sample','native/pre_sample'):
                d=self.envelope(box.run([BINARY,'session','inspect',target,'--json']))
                s=d['data']['session'];self.assertEqual(s['locator']['name'],'pre_sample')
                self.assertEqual(s['locator']['host_id'],d['data']['host_id'])
                self.assertIsNone(s['managed_id']);self.assertRegex(s['instance_id'],r'^[a-f0-9]{32}$')
                self.assertEqual(s['process_state'],'running');self.assertEqual(s['attached_clients'],0)
            d=self.envelope(box.run([BINARY,'session','inspect','native/pre_sample','--backend','tmux','--json'],check=False),2)
            self.assertEqual(d['errors'][0]['code'],'invalid_target')
            (box.root/'bin/screen').unlink()
            d=self.envelope(box.run([BINARY,'list','--backend','screen','--json'],check=False),7)
            self.assertEqual(d['errors'][0]['code'],'backend_unavailable')
            self.assertIsNone(server.poll())
            self.assertFalse((box.root/'state/txm/v1/host.json').is_symlink())

    def test_partial_aggregation_and_error_precedence(self):
        with Sandbox() as box, native_server(box,BINARY) as (_,_,_):
            fake_tmux(box)
            result=self.envelope(box.run([BINARY,'list','--all-backends','--json'],check=False),7)
            self.assertTrue(result['partial'])
            ss=result['data']['sessions'];self.assertEqual([(s['locator']['backend'],s['locator']['name']) for s in ss],[('native','sample'),('tmux','sample')])
            self.assertEqual([e['backend'] for e in result['errors']],['screen','zellij'])
            caps=self.envelope(box.run([BINARY,'backend','capabilities','--all-backends','--json'],check=False),7)
            self.assertTrue(caps['partial']);self.assertEqual(len(caps['data']['backends']),4)
            for c in caps['data']['backends']:
                if c['version'] is None:self.assertTrue(all(v['level']=='unsupported' for v in c['capabilities'].values()))
        with Sandbox() as box:
            (box.root/'runtime').chmod(0o755)
            fake(box,'tmux',"print('permission denied',file=sys.stderr);sys.exit(1)")
            (box.root/'bin/screen').unlink()
            d=self.envelope(box.run([BINARY,'list','--all-backends','--json'],check=False),8)
            self.assertIsNone(d['data']);self.assertFalse(d['partial']);self.assertEqual(len(d['errors']),4)
            (box.root/'runtime').chmod(0o700)

    def test_capabilities_and_version_failure(self):
        with Sandbox() as box:
            fake_tmux(box)
            for backend in ('tmux','native'):
                d=self.envelope(box.run([BINARY,'backend','capabilities','--backend',backend,'--json']))
                b=d['data']['backends'][0];self.assertTrue(b['available']);self.assertEqual(b['backend'],backend)
                self.assertEqual(b['capabilities']['managed_launch']['level'],'supported')
                self.assertEqual(b['capabilities']['read_only_attach']['level'],'supported')
                self.assertIsNotNone(b['version'])
            fake(box,'tmux',"print('tmux 999.0')",version=False)
            d=self.envelope(box.run([BINARY,'backend','capabilities','--backend','tmux','--json'],check=False),4)
            self.assertEqual(d['errors'][0]['code'],'unsupported_version')
            for backend,version in (('screen','Screen version 4.9.1'),('zellij','zellij 0.43.1')):
                fake(box,backend,f"print({version!r})")
                d=self.envelope(box.run([BINARY,'backend','capabilities','--backend',backend,'--json']))
                c=d['data']['backends'][0]['capabilities']
                self.assertEqual(c['session_lifecycle']['level'],'degraded')
                for key in ('input_injection','read_only_attach','exact_pane_targeting','output_snapshot'):self.assertEqual(c[key]['level'],'unsupported')

    def test_screen_exact_full_identity_and_duplicate_names(self):
        with Sandbox() as box:
            log=fake(box,'screen',"if args[0]=='-ls': print('  123.foobar (Detached)')")
            out=box.run([BINARY,'delete','screen/foo'],check=False);self.assertEqual(out.returncode,3)
            self.assertTrue(all(c[0]=='-ls' for c in calls(log)))
            log=fake(box,'screen',"if args[0]=='-ls': print('  123.foo (Detached)\\n  124.foo (Attached)')")
            out=box.run([BINARY,'delete','screen/foo'],check=False);self.assertEqual(out.returncode,5)
            self.assertTrue(all(c[0]=='-ls' for c in calls(log)))
            d=self.envelope(box.run([BINARY,'session','inspect','screen/124.foo','--json']))
            self.assertEqual(d['data']['session']['backend_id'],'124.foo')
            self.assertIsNone(d['data']['session']['attached_clients'])
            box.run([BINARY,'delete','screen/123.foo'])
            self.assertEqual(calls(log)[-1],['-S','123.foo','-X','quit'])

    def test_zellij_exact_lookup_and_no_force_retry(self):
        with Sandbox() as box:
            log=fake(box,'zellij',"if args[0]=='list-sessions': print('foobar')\nelif args[0]=='kill-session': sys.exit(1)")
            self.assertEqual(box.run([BINARY,'delete','zellij/foo'],check=False).returncode,3)
            self.assertTrue(all(c[0] in ('-V','list-sessions') for c in calls(log)))
            self.assertEqual(box.run([BINARY,'delete','zellij/foobar'],check=False).returncode,1)
            mutations=[c for c in calls(log) if c[0]!='list-sessions']
            self.assertEqual(mutations,[['kill-session','foobar']])
            self.assertTrue(all(c==['list-sessions','--short','--no-formatting'] for c in calls(log) if c[0]=='list-sessions'))

    def test_unverified_exact_operations_make_zero_backend_calls(self):
        for backend in ('screen','zellij'):
            with Sandbox() as box:
                log=fake(box,backend,"raise AssertionError('no backend command should run')")
                for args in (['window','kill','foo','bar'],['window','rename','foo','bar','baz'],['window','split','foo','bar','h'],['pane','kill','foo','bar','1'],['exec','foo','bar','1','sentinel']):
                    out=box.run([BINARY,'--backend',backend,*args],check=False)
                    self.assertEqual(out.returncode,4,(args,out.stderr))
                d=self.envelope(box.run([BINARY,'--backend',backend,'session','output','foo','--json'],check=False),4)
                self.assertEqual(d['errors'][0]['code'],'unsupported');self.assertEqual(calls(log),[])

    def test_dynamic_backend_and_qualified_completion(self):
        with Sandbox() as box, native_server(box,BINARY) as (_,_,_):
            completed=box.run([BINARY,'__complete','session','inspect','native/s'])
            self.assertIn('native/sample',completed.stdout)
            bad=box.root/'home/.txm/config';bad.mkdir(parents=True)
            completed=box.run([BINARY,'__complete','list','--backend',''])
            for name in ('native','screen','tmux','zellij'):self.assertIn(name,completed.stdout)
        with Sandbox() as box:
            fake(box,'zellij',"print('No active zellij sessions found.');sys.exit(1)")
            d=self.envelope(box.run([BINARY,'list','--backend','zellij','--json']))
            self.assertEqual(d['data']['sessions'],[])

    def test_malformed_discovery_cannot_create(self):
        with Sandbox() as box:
            log=fake(box,'tmux',"if args[0]=='list-sessions': print('invalid metadata')")
            d=self.envelope(box.run([BINARY,'list','--backend','tmux','--json'],check=False),1)
            self.assertEqual(d['errors'][0]['code'],'protocol_error')
            self.assertEqual(box.run([BINARY,'attach','--backend','tmux','foo'],check=False).returncode,1)
            self.assertTrue(all(c[0] in ('-V','list-sessions') for c in calls(log)))

    def test_query_timeout_and_cancellation_emit_valid_json(self):
        with Sandbox() as box:
            fake(box,'tmux',"os.execv('/bin/sleep',['sleep','120'])")
            start=time.monotonic()
            d=self.envelope(box.run([BINARY,'list','--backend','tmux','--json'],check=False,timeout=8),6)
            self.assertLess(time.monotonic()-start,7);self.assertEqual(d['errors'][0]['code'],'timeout')
            proc=box.start([BINARY,'list','--backend','tmux','--json'],stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
            time.sleep(.15);proc.send_signal(signal.SIGTERM);out,err=proc.communicate(timeout=3)
            class Result: pass
            r=Result();r.returncode=proc.returncode;r.stdout=out;r.stderr=err
            d=self.envelope(r,130);self.assertEqual(d['errors'][0]['code'],'cancelled')

    def test_capture_collection_is_bounded(self):
        with Sandbox() as box:
            fake(box,'tmux',"""if args[0]=='list-sessions': print('$1|sample|0')
elif args[0]=='display-message': print('%1|24|100')
elif args[0]=='capture-pane':
 while True: os.write(1,('λ'*8192).encode())
""")
            start=time.monotonic()
            d=self.envelope(box.run([BINARY,'session','output','tmux/sample','--bytes','101','--json'],timeout=5))
            snap=d['data'];self.assertLess(time.monotonic()-start,3);self.assertTrue(snap['truncated']);self.assertLessEqual(snap['bytes'],101)
            self.assertEqual(len(snap['content'].encode()),snap['bytes']);self.assertNotIn('\ufffd',snap['content'])

    def test_native_bounded_output_and_history_rejection(self):
        with Sandbox() as box:
            script=box.root/'output.py';script.write_text("import sys,time\nsys.stdout.write('\\x1b[31mSAFE_λ_雪\\x1b[0m\\x1b]52;c;secret\\x07\\n');sys.stdout.flush();time.sleep(120)\n")
            with native_server(box,BINARY,argv=[sys.executable,script]):
                def content():return self.envelope(box.run([BINARY,'session','output','sample','--json']))['data']['content']
                wait_for(lambda:'SAFE_' in content())
                d=self.envelope(box.run([BINARY,'session','output','sample','--lines','2','--bytes','9','--json']))
                self.assertLessEqual(d['data']['bytes'],9);self.assertTrue(d['data']['truncated']);self.assertNotIn('\x1b',d['data']['content'])
                raw=self.envelope(box.run([BINARY,'session','output','sample','--raw','--json']))['data']
                self.assertEqual(raw['encoding'],'base64');self.assertIn(b'\x1b',base64.b64decode(raw['content']))
                self.envelope(box.run([BINARY,'session','output','sample','--history','--json'],check=False),4)
                self.assertNotIn('\x1b',box.run([BINARY,'dump','sample']).stdout)

    @unittest.skipUnless(shutil.which('tmux'),'tmux not installed')
    def test_real_tmux_exact_targets_and_literal_input(self):
        with Sandbox() as box:
            box.start_tmux(shutil.which('tmux'))
            tmux(box,'set-option','-g','base-index','1');tmux(box,'set-window-option','-g','pane-base-index','1')
            recorded=box.root/'input.json';ready=box.root/'reader-ready'
            reader=box.root/'reader.py';reader.write_text(f'''import os,tty,json,time
from pathlib import Path
tty.setraw(0)
Path({str(ready)!r}).touch()
data=b'';lines=[]
while True:
 data+=os.read(0,4096)
 while b'\\r' in data:
  line,data=data.split(b'\\r',1);lines.append(line.decode());p=Path({str(recorded)!r});tmp=p.with_suffix('.tmp');tmp.write_text(json.dumps(lines));tmp.replace(p)
''')
            box.run([BINARY,'create','sample',sys.executable,reader]);wait_for(ready.exists)
            box.run([BINARY,'create','sample_extra','/bin/sleep','120'])
            row=tmux(box,'list-panes','-t','=sample','-F','#{pane_id}|#{window_id}|#{pane_index}|#{window_index}').split('|')
            self.assertEqual(len(row),4,(row,tmux(box,'list-sessions','-F','#{session_id}:#{session_name}'),tmux(box,'list-windows','-t','=sample','-F','#{window_id}:#{window_name}')))
            pane,window,index,winindex=row;self.assertEqual((index,winindex),('1','1'))
            other=tmux(box,'split-window','-d','-P','-F','#{pane_id}','-t',pane,'/bin/sleep 120')
            tmux(box,'select-pane','-t',other)
            commands=['Enter','C-c','-dash','spaces with text','λ雪',';','end;','--json','--','$(synthetic)']
            for command in commands:box.run([BINARY,'exec','sample',window,pane,command])
            wait_for(lambda:recorded.exists() and len(json.loads(recorded.read_text()))==len(commands))
            self.assertEqual(json.loads(recorded.read_text()),commands)
            self.assertEqual(box.run([BINARY,'delete','sampl'],check=False).returncode,3)
            wrong=tmux(box,'list-panes','-t','=sample_extra','-F','#{pane_id}')
            self.assertEqual(box.run([BINARY,'exec','sample',window,wrong,'DO_NOT_SEND'],check=False).returncode,3)
            self.assertEqual(box.run([BINARY,'exec','sample',window,'','DO_NOT_SEND'],check=False).returncode,5)
            win2=tmux(box,'new-window','-P','-F','#{window_id}','-t','=sample:','-n','editor','/bin/sleep 120')
            tmux(box,'new-window','-t','=sample:','-n','editor','/bin/sleep 120')
            self.assertEqual(box.run([BINARY,'window','kill','sample','ed'],check=False).returncode,3)
            self.assertEqual(box.run([BINARY,'window','kill','sample','editor'],check=False).returncode,5)
            box.run([BINARY,'window','rename','sample',win2,'renamed'])
            self.assertEqual(tmux(box,'display-message','-p','-t',win2,'#{window_name}'),'renamed')
            box.run([BINARY,'pane','kill','sample',window,pane])
            self.assertEqual(tmux(box,'list-panes','-t',window,'-F','#{pane_id}'),other)
            self.assertEqual(json.loads(recorded.read_text()),commands)

    @unittest.skipUnless(shutil.which('tmux'),'tmux not installed')
    def test_real_tmux_snapshot_and_explicit_history(self):
        with Sandbox() as box:
            box.start_tmux(shutil.which('tmux'))
            emitted=box.root/'emitted';script=box.root/'emitter.py'
            script.write_text(f"import sys,time\nfrom pathlib import Path\nprint('OLD_LINE')\nfor i in range(100):print('ROW-%03d-λ-雪'%i)\nprint('\\x1b[31mVISIBLE_END\\x1b[0m\\x1b]52;c;synthetic\\x07');sys.stdout.flush();Path({str(emitted)!r}).touch();time.sleep(120)\n")
            box.run([BINARY,'create','sample',sys.executable,script]);wait_for(emitted.exists)
            d=self.envelope(box.run([BINARY,'session','output','sample','--json']))['data']
            self.assertEqual(d['type'],'screen_snapshot');self.assertNotIn('OLD_LINE',d['content']);self.assertIn('VISIBLE_END',d['content']);self.assertNotIn('\x1b',d['content'])
            history=self.envelope(box.run([BINARY,'session','output','sample','--history','--lines','1000','--json']))['data']
            self.assertEqual(history['type'],'history');self.assertIn('OLD_LINE',history['content'])
            short=self.envelope(box.run([BINARY,'session','output','sample','--history','--lines','3','--bytes','13','--json']))['data']
            self.assertTrue(short['truncated']);self.assertLessEqual(short['lines'],3);self.assertLessEqual(short['bytes'],13)
            raw=self.envelope(box.run([BINARY,'session','output','sample','--raw','--json']))['data']
            self.assertEqual(raw['format'],'raw');self.assertIn(b'\x1b',base64.b64decode(raw['content']))
            box.verify_tmux()

if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--binary',type=Path,required=True);parser.add_argument('tests',nargs='*');args=parser.parse_args();BINARY=args.binary.resolve(strict=True)
    unittest.main(argv=[sys.argv[0],*args.tests])
