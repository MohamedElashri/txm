#!/usr/bin/env python3
"""Disposable Git repositories and fake SSH only; no user resources."""
import argparse
import base64
import json
import fcntl
import hashlib
import os
from pathlib import Path
import pty
import secrets
import shlex
import shutil
import signal
import subprocess
import sys
import termios
from tty_state import attributes as tty_attributes
import time
import unittest
from concurrent.futures import ThreadPoolExecutor
import agent_checks as a
from sandbox import Sandbox, native_server, wait_for
BINARY = None
GIT = shutil.which('git', path='/usr/bin:/bin')

def command(box, args, code=0, timeout=35):
    p=box.run([BINARY,'--json',*args],check=False,timeout=timeout)
    assert p.returncode==code,(args[:4],p.returncode,p.stdout,p.stderr)
    assert not p.stderr,p.stderr
    return json.loads(p.stdout)

def git(box,*args,repo=None):
    return box.run([GIT,'-C',str(repo or box.root/'repo'),*args]).stdout.strip()

def repo(box):
    (box.root/'bin/git').symlink_to(GIT)
    git(box,'init','-b','main')
    git(box,'config','user.name','fixture')
    git(box,'config','user.email','fixture@example.invalid')
    (box.root/'repo/tracked').write_text('committed\n')
    (box.root/'repo/.gitignore').write_text('ignored\n')
    git(box,'add','tracked','.gitignore');git(box,'commit','-m','fixture')
    return box.root/'repo'

def start(box,name='task',argv=None,extra=None,backend='native'):
    return command(box,['--backend',backend,'task','start',name,'--repo',str(box.root/'repo'),*(extra or []),'--',*(argv or ['/bin/sleep','90'])])['data']

def remote_req(op='probe',**fields):
    q=dict(schema_version=1,protocol_version=1,op=op,host_id='',backend='',name='',backend_id='',instance_id='',read_only=False);q.update(fields)
    return base64.urlsafe_b64encode(json.dumps(q).encode()).decode().rstrip('=')

def fake_ssh(local,remote):
    log=local.root/'ssh-args.jsonl'
    local.executable('ssh',rf'''#!{sys.executable}
import json,os,shlex,sys,time
from pathlib import Path
args=sys.argv[1:]
with open({str(log)!r},'a') as f:f.write(json.dumps(args)+'\n')
assert args[-3]=='--' and args[-2] in ('fixture','slow','bad','wrong','version','collision','garbage')
assert args[0] in ('-T','-tt')
for option in ('BatchMode=yes','StrictHostKeyChecking=yes','ConnectTimeout=5','ConnectionAttempts=1','ServerAliveInterval=5','ServerAliveCountMax=1'):assert option in args
parts=shlex.split(args[-1]);assert parts[:4]==['txm','internal','remote','--request'] and len(parts)==5
assert all(c in 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-' for c in parts[4])
if args[-2]=='slow':time.sleep(20)
if args[-2]=='bad':sys.exit(255)
if args[-2] in ('wrong','version','collision','garbage'):
 if args[-2]=='garbage':print('PRIVATE_DIAGNOSTIC');sys.exit(0)
 host='f'*32 if args[-2]=='wrong' else ({{'version':'4','collision':'5'}}[args[-2]]*32)
 session=dict(locator=dict(host_id=host,backend='tmux',name='one'),backend_id='$1',managed_id=None,instance_id=None,process_state='unknown',client_state='unknown',attached_clients=None,observed_at='2026-10-30T00:00:00Z',source='fixture',capabilities={{}})
 data=dict(protocol_version=2 if args[-2]=='version' else 1,host_id=host,sessions=[session,session] if args[-2]=='collision' else [])
 print(json.dumps(dict(schema_version=1,data=data,errors=[],partial=False)));sys.exit(0)
os.execve({str(BINARY)!r},[{str(BINARY)!r},*parts[1:]],{remote.env!r})
''')
    return log

class TaskChecks(unittest.TestCase):
    def test_dry_run_and_branch_filter_boundaries(self):
        with a.fixture() as box:
            source=repo(box);before=git(box,'worktree','list','--porcelain');head=git(box,'rev-parse','HEAD')
            hook=source/'.git/hooks/post-checkout';marker=box.root/'hook-ran'
            hook.write_text('#!/bin/sh\ntouch '+shlex.quote(str(marker))+'\n');hook.chmod(0o700)
            box.env['NOT_ALLOWED']='PRIVATE_ENV_VALUE'
            out=command(box,['task','start','dry','--repo',str(source),'--dry-run','--','/bin/echo','PRIVATE_ARG_VALUE'])
            self.assertTrue(out['data']['dry_run']);self.assertNotIn('PRIVATE_',json.dumps(out))
            self.assertFalse((box.root/'state/txm').exists());self.assertFalse((box.root/'runtime/txm').exists())
            self.assertEqual(git(box,'worktree','list','--porcelain'),before);self.assertEqual(git(box,'rev-parse','HEAD'),head)
            command(box,['task','start','dry','--repo',str(source),'--branch','main','--dry-run','--','/usr/bin/true'],5)
            git(box,'config','filter.fixture.smudge','touch '+str(marker))
            command(box,['task','start','filter','--repo',str(source),'--dry-run','--','/usr/bin/true'],8)
            command(box,['task','start','filter','--repo',str(source),'--allow-repo-commands','--dry-run','--','/usr/bin/true'])
            self.assertFalse(marker.exists());self.assertFalse((box.root/'state/txm').exists())

    def test_detached_head_hooks_dirty_and_forced_cleanup(self):
        with a.fixture() as box, a.coordinator(box):
            source=repo(box);git(box,'checkout','--detach');base=git(box,'rev-parse','HEAD')
            (source/'tracked').write_text('source dirty\n');(source/'untracked').write_text('source private')
            marker=box.root/'hook-ran';hook=source/'.git/hooks/post-checkout';hook.write_text('#!/bin/sh\ntouch '+shlex.quote(str(marker))+'\n');hook.chmod(0o700)
            v=start(box);r=v['task'];ident=r['id'];work=Path(r['worktree'])
            self.assertEqual(v['agent']['managed_id'],ident);self.assertEqual(r['base_commit'],base)
            self.assertEqual((work/'tracked').read_text(),'committed\n');self.assertFalse((work/'untracked').exists());self.assertFalse(marker.exists())
            command(box,['task','cleanup',ident],7)
            command(box,['task','stop',ident]);a.await_exit(box,ident)
            for name in ['tracked','untracked','ignored']:
                p=work/name;old=p.read_bytes() if p.exists() else None;p.write_text('preserve fixture')
                command(box,['task','cleanup',ident],7);self.assertTrue(p.exists())
                if old is None:p.unlink()
                else:p.write_bytes(old)
            (work/'untracked').write_text('fixture only')
            command(box,['task','cleanup',ident,'--force'],2)
            command(box,['task','cleanup',ident,'--force','--confirm','0'*32],2)
            command(box,['task','cleanup',ident,'--force','--confirm',ident,'--dry-run']);self.assertTrue(work.exists())
            command(box,['task','cleanup',ident,'--force','--confirm',ident]);self.assertFalse(work.exists())
            self.assertEqual(git(box,'rev-parse',r['branch']),base);self.assertEqual((source/'tracked').read_text(),'source dirty\n');self.assertTrue((source/'untracked').exists())
            command(box,['task','cleanup',ident])

    def test_restart_orphan_and_no_reexecution(self):
        with a.fixture() as box:
            repo(box)
            with a.coordinator(box):
                v=start(box,argv=['/usr/bin/true']);r=v['task'];a.await_exit(box,r['id'])
            with a.coordinator(box):
                got=command(box,['task','inspect',r['id']])['data'];self.assertEqual(got['agent']['lifecycle'],'exited')
                response=a.rpc(box,dict(op='start',id=r['id'],backend='native',name='must-not-replay',cwd=r['worktree'],argv=['/usr/bin/true'],env_names=['HOME','PATH','TERM']))
                self.assertEqual(response['error']['code'],'conflict')
                command(box,['task','cleanup',r['id']])
                # Existing unmanaged name causes a known prelaunch refusal. Its
                # resource survives and preflight creates no new worktree.
                with native_server(box,BINARY,name='collision'):
                    before=git(box,'worktree','list','--porcelain')
                    command(box,['task','start','collision','--repo',str(box.root/'repo'),'--','/usr/bin/true'],5)
                    self.assertEqual(git(box,'worktree','list','--porcelain'),before)
                    self.assertIn('collision',box.run([BINARY,'list']).stdout)

    def test_identity_symlink_and_state_copy_refusal(self):
        with a.fixture() as box,a.coordinator(box):
            source=repo(box);v=start(box,argv=['/usr/bin/true']);r=v['task'];ident=r['id'];a.await_exit(box,ident);work=Path(r['worktree']);saved=work.with_name('saved')
            work.rename(saved);work.symlink_to(saved,target_is_directory=True)
            command(box,['task','cleanup',ident,'--force','--confirm',ident],7);self.assertTrue(saved.exists())
            work.unlink();saved.rename(work)
            original=(work/'.git').read_bytes()
            # Keep the replacement inode alive to make the mismatch deterministic.
            replacement=work/'.git';pin=work/'.replacement';replacement.rename(pin);replacement.write_bytes(original)
            command(box,['task','cleanup',ident,'--force','--confirm',ident],7);replacement.unlink();pin.rename(replacement)
            other=box.root/'copied-state';shutil.copytree(box.root/'state',other)
            env=dict(box.env,XDG_STATE_HOME=str(other));p=box.run([BINARY,'--json','task','inspect',ident],env=env,check=False);self.assertEqual(p.returncode,1);self.assertTrue(work.exists())
            link=box.root/'repo-link';link.symlink_to(source,target_is_directory=True)
            command(box,['task','start','symlink','--repo',str(link),'--dry-run','--','/usr/bin/true'],5)

    @unittest.skipUnless(shutil.which('tmux'),'tmux unavailable')
    def test_tmux_rename_and_owner_loss(self):
        with a.fixture() as box:
            repo(box);box.start_tmux(shutil.which('tmux'))
            with a.coordinator(box):
                v=start(box,backend='tmux');r=v['task'];ident=r['id'];binding=json.loads((a.state(box)/'registry.json').read_text())['agents'][0]['binding'];sid=binding['backend_id']
                box.run(['tmux','rename-session','-t',sid,'renamed'])
                box.run(['tmux','set-environment','-t',sid,'TXM_MANAGED_INSTANCE','0'*32])
                command(box,['task','stop',ident],7);self.assertEqual(box.run(['tmux','display-message','-p','-t',sid,'#{session_name}']).stdout.strip(),'renamed')
                box.run(['tmux','set-environment','-t',sid,'TXM_MANAGED_INSTANCE',binding['owner']])
                command(box,['task','stop',ident]);a.await_exit(box,ident);command(box,['task','cleanup',ident]);box.verify_tmux()

    def test_conditional_filters_cannot_run_during_new_worktree_checkout(self):
        with a.fixture() as box:
            source=repo(box);marker=box.root/'conditional-filter-ran';config=box.root/'filters.conf'
            (source/'.gitattributes').write_text('*.txt filter=fixture\n');(source/'data.txt').write_text('fixture data\n');git(box,'add','.gitattributes','data.txt');git(box,'commit','-m','attributes')
            config.write_text('[filter "fixture"]\n smudge = /bin/sh -c "touch '+str(marker)+'; cat"\n required = true\n')
            key='includeIf.gitdir:'+str(source/'.git/worktrees')+'/.path';git(box,'config',key,str(config))
            result=command(box,['task','start','conditional','--repo',str(source),'--','/usr/bin/true'],7)['data'];r=result['task']
            self.assertTrue(r['created']);self.assertFalse(r['launch_requested']);self.assertFalse(marker.exists())
            self.assertFalse((box.root/'runtime/txm/v1/agents').exists());git(box,'config','--unset-all',key)
            command(box,['task','cleanup',r['id'],'--force','--confirm',r['id']]);self.assertFalse(marker.exists())

    def test_native_name_reuse_and_changed_branch_never_target_other_resource(self):
        with a.fixture() as box,a.coordinator(box):
            repo(box);v=start(box,argv=['/usr/bin/true']);r=v['task'];ident=r['id'];a.await_exit(box,ident)
            wait_for(lambda:r['name'] not in box.run([BINARY,'list']).stdout.splitlines())
            lock=box.root/'runtime/txm/v1'/('n-'+hashlib.sha256(r['name'].encode()).hexdigest()+'.lock')
            def released():
                with lock.open('r+') as f:
                    try:fcntl.flock(f,fcntl.LOCK_EX|fcntl.LOCK_NB)
                    except BlockingIOError:return False
                    fcntl.flock(f,fcntl.LOCK_UN);return True
            wait_for(released)
            with native_server(box,BINARY,name=r['name']):
                command(box,['--backend','tmux','task','stop',ident],2)
                command(box,['task','stop',ident]);self.assertIn(r['name'],box.run([BINARY,'list']).stdout)
                git(box,'switch','-c','different-branch',repo=Path(r['worktree']))
                command(box,['task','cleanup',ident,'--force','--confirm',ident],7)
                git(box,'switch',r['branch'],repo=Path(r['worktree']))
                command(box,['task','cleanup',ident]);self.assertIn(r['name'],box.run([BINARY,'list']).stdout)

    def test_registry_loss_requires_pinned_exit_receipt(self):
        with a.fixture() as box:
            repo(box)
            with a.coordinator(box):
                r=start(box)['task'];ident=r['id']
                index=box.root/'state/txm/v1/tasks/index.json';saved=index.read_bytes()
                changed=json.loads(saved);changed['tasks'][0]['instance_id']=secrets.token_hex(16)
                index.write_text(json.dumps(changed))
                command(box,['task','stop',ident],7)
                command(box,['task','cleanup',ident,'--force','--confirm',ident],7)
                self.assertEqual(a.status(box,ident)['lifecycle'],'running')
                index.write_bytes(saved)
            registry=a.state(box)/'registry.json';original=registry.read_bytes();registry.unlink()
            # Losing the coordinator registry must not turn a live task into a
            # never-launched orphan, including with explicit dirty-file force.
            with a.coordinator(box):
                command(box,['task','cleanup',ident],7)
                command(box,['task','cleanup',ident,'--force','--confirm',ident],7)
                self.assertTrue(Path(r['worktree']).exists())
            registry.write_bytes(original);registry.chmod(0o600)
            with a.coordinator(box):
                command(box,['task','stop',ident]);a.await_exit(box,ident)
            registry.unlink()
            # A pinned instance plus its validated immutable receipt still proves
            # completion if the registry is lost after the child has exited.
            with a.coordinator(box):
                view=command(box,['task','inspect',ident])['data']
                self.assertEqual(view['agent']['lifecycle'],'exited')
                self.assertEqual(view['agent']['instance_id'],r['instance_id'])
                receipt=a.state(box)/('r-'+r['instance_id']+'.json');data=receipt.read_bytes()
                receipt.write_text('{"schema_version":99}')
                command(box,['task','cleanup',ident,'--force','--confirm',ident],7)
                self.assertTrue(Path(r['worktree']).exists());receipt.write_bytes(data)
                index=box.root/'state/txm/v1/tasks/index.json';metadata=index.read_bytes()
                lost=json.loads(metadata);lost['tasks'][0].pop('instance_id');index.write_text(json.dumps(lost))
                command(box,['task','cleanup',ident,'--force','--confirm',ident],7)
                index.write_bytes(metadata)
                command(box,['task','cleanup',ident]);self.assertFalse(Path(r['worktree']).exists())
                self.assertTrue(git(box,'rev-parse',r['branch']))

    def test_concurrent_tasks_commits_and_unknown_launch_intent(self):
        with a.fixture() as box:
            repo(box)
            with a.coordinator(box):
                with ThreadPoolExecutor(max_workers=3) as pool:values=list(pool.map(lambda n:start(box,name='task'+str(n),argv=['/usr/bin/true']),range(3)))
                self.assertEqual(len({v['task']['id'] for v in values}),3)
                for v in values:a.await_exit(box,v['task']['id'])
                self.assertEqual(len(command(box,['task','list'])['data']),3)
                r=values[0]['task'];work=Path(r['worktree']);(work/'tracked').write_text('task commit\n');git(box,'add','tracked',repo=work);git(box,'commit','-m','task commit',repo=work);commit=git(box,'rev-parse','HEAD',repo=work)
                command(box,['task','cleanup',r['id']]);self.assertEqual(git(box,'rev-parse',r['branch']),commit)
            # Simulate a durable prelaunch intent with neither handshake nor receipt.
            registry=a.state(box)/'registry.json';data=json.loads(registry.read_text());r=values[1]['task'];agent=next(x for x in data['agents'] if x['id']==r['id']);agent['binding']=dict(backend_id='',native_instance='',owner='')
            receipt=a.state(box)/('r-'+agent['instance_id']+'.json');receipt.unlink();registry.write_text(json.dumps(data))
            with a.coordinator(box):
                view=command(box,['task','inspect',r['id']])['data'];self.assertEqual(view['agent']['lifecycle'],'unknown')
                command(box,['task','cleanup',r['id'],'--force','--confirm',r['id']],7);self.assertTrue(Path(r['worktree']).exists())

class RemoteChecks(unittest.TestCase):
    def test_fake_transport_pins_partial_version_collision_and_timeout(self):
        with a.fixture() as local,a.fixture() as remote:
            host=command(remote,['internal','remote','--request',remote_req()])['data']['host_id']
            log=fake_ssh(local,remote)
            command(local,['remote','probe','fixture'],8);self.assertFalse(log.exists())
            command(local,['remote','allow','fixture']);self.assertEqual(command(local,['remote','probe','fixture'])['data']['host_id'],host)
            command(local,['remote','list','fixture'],8);command(local,['remote','pin','fixture',host])
            command(local,['remote','allow','duplicate','--host-id',host],5)
            with native_server(remote,BINARY,name='remote-session'):
                sessions=command(local,['remote','list','fixture'])['data'][0]['sessions'];self.assertEqual(sessions[0]['locator']['host_id'],host);self.assertEqual(sessions[0]['locator']['name'],'remote-session')
            # Distinct configured pins let failures be attributed per alias.
            for alias in ['bad','slow','wrong','version','collision','garbage']:
                pin=host if alias=='fixture' else (('1' if alias=='wrong' else {'bad':'2','slow':'3','version':'4','collision':'5','garbage':'6'}[alias])*32)
                command(local,['remote','allow',alias,'--host-id',pin])
            out=command(local,['remote','list','fixture','bad'],7);self.assertTrue(out['partial']);self.assertEqual(len(out['data']),2);self.assertEqual(out['data'][1]['error']['code'],'backend_unavailable')
            command(local,['remote','list','slow','--timeout','1s'],6)
            for alias,code in [('wrong',5),('version',4),('collision',1),('garbage',1)]:
                out=command(local,['--backend','tmux','remote','list',alias],code);self.assertNotIn('PRIVATE_DIAGNOSTIC',json.dumps(out))
            self.assertTrue(all(json.loads(line)[-3]=='--' for line in log.read_text().splitlines()))
            for alias in ['user@host','host:22','-oProxyCommand=x','space host']:
                command(local,['remote','allow',alias],2)

    def test_cancelled_ssh_and_normal_core_do_not_initialize_extensions(self):
        with a.fixture() as local,a.fixture() as remote:
            command(local,['--backend','native','list']);state=local.root/'state/txm/v1'
            for name in ['tasks','task-worktrees','remotes','agents']:self.assertFalse((state/name).exists())
            host=command(remote,['internal','remote','--request',remote_req()])['data']['host_id'];log=fake_ssh(local,remote)
            command(local,['remote','allow','slow','--host-id',host])
            proc=local.start([BINARY,'--json','remote','list','slow'],stdout=subprocess.PIPE,stderr=subprocess.PIPE)
            wait_for(log.exists);proc.terminate();out,err=proc.communicate(timeout=5);self.assertEqual(proc.returncode,130);self.assertEqual(json.loads(out)['errors'][0]['code'],'cancelled');self.assertFalse(err)
            self.assertFalse((state/'tasks').exists());self.assertFalse((state/'agents').exists())

    def test_endpoint_bad_protocol_and_host_refuse_before_backend(self):
        with a.fixture() as box:
            command(box,['internal','remote','--request',remote_req(protocol_version=2)],4)
            self.assertFalse((box.root/'state/txm').exists())
            host=command(box,['internal','remote','--request',remote_req()])['data']['host_id']
            calls=box.root/'backend-calls';box.executable('tmux','#!/bin/sh\ntouch '+shlex.quote(str(calls))+'\nexit 1\n')
            command(box,['internal','remote','--request',remote_req('list',host_id='0'*32,backend='tmux')],5)
            self.assertFalse(calls.exists())
            self.assertFalse((box.root/'state/txm/v1/agents').exists())

    def test_attach_encoding_pinned_instance_and_local_tty_restore(self):
        with a.fixture() as local,a.fixture() as remote:
            host=command(remote,['internal','remote','--request',remote_req()])['data']['host_id'];log=fake_ssh(local,remote)
            command(local,['remote','allow','fixture','--host-id',host])
            with native_server(remote,BINARY,name='remote-session',argv=['/bin/cat']) as (_,sock,_):
                from wire import Peer
                with Peer(sock,'observer') as peer:instance=peer.identity['instance_id']
                args=[BINARY,'--backend','native','remote','attach','fixture','--host-id',host,'--name','remote-session','--session-id',instance,'--instance',instance,'--read-only']
                p=local.run(args,check=False);self.assertEqual(p.returncode,2)
                master,slave=pty.openpty();before=tty_attributes(slave)
                try:
                    proc=local.start(args,stdin=slave,stdout=slave,stderr=subprocess.PIPE)
                    wait_for(lambda:log.exists());time.sleep(.2)
                    os.write(master,b'REMOTE_READONLY_SENTINEL\n');time.sleep(.1)
                    from native_checks import snapshot
                    self.assertNotIn(b'REMOTE_READONLY_SENTINEL',snapshot(sock))
                    proc.terminate();_,err=proc.communicate(timeout=8);self.assertEqual(proc.returncode,130,err);self.assertEqual(tty_attributes(slave),before)
                finally:os.close(master);os.close(slave)
                # A malicious name stays inside the encoded JSON token.
                marker=local.root/'shell-injection';evil='name; touch '+str(marker)+'; $(id) \n'
                master,slave=pty.openpty()
                try:
                    altered=args[:];altered[altered.index('--name')+1]=evil
                    proc=local.start(altered,stdin=slave,stdout=slave,stderr=subprocess.PIPE);proc.communicate(timeout=8);self.assertIn(proc.returncode,(1,2,3,5));self.assertFalse(marker.exists())
                finally:os.close(master);os.close(slave)
                raw=log.read_text();request=json.loads(base64.urlsafe_b64decode(json.loads(raw.splitlines()[0])[-1].split()[-1]+'==='));self.assertTrue(request['read_only']);self.assertNotIn('shell-injection',raw);self.assertNotIn('$(id)',raw)

if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--binary',type=Path,required=True);args,remaining=parser.parse_known_args();BINARY=args.binary.resolve(strict=True);a.BINARY=BINARY
    unittest.main(argv=[sys.argv[0],*remaining])
