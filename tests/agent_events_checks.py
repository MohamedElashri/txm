#!/usr/bin/env python3
"""private fixtures: signals, streams, waits and explicit actions."""
import argparse
from contextlib import contextmanager
from datetime import datetime, timezone, timedelta
import json
from pathlib import Path
import secrets
import shutil
import shlex
import signal
import subprocess
import sys
import time
import unittest

import agent_checks as a
from sandbox import native_server, wait_for

BINARY = None
CLAUDE = None


def stamp(offset=0):
    return (datetime.now(timezone.utc) + timedelta(seconds=offset)).isoformat().replace('+00:00', 'Z')


def cred(box, v):
    path = a.state(box) / ('h-' + v['hooks']['generation'] + '.json')
    if not path.exists():
        path = a.state(box) / ('h2-' + v['hooks']['generation'] + '.json')
    return json.loads(path.read_text())


def signal_request(box, v, sequence, wanted_state, **changes):
    c = cred(box, v)
    body = dict(instance_id=c['instance_id'], generation=c['generation'], token=c['token'],
                sequence=sequence, time=stamp(), state=wanted_state, session_id='')
    body.update(changes)
    return a.rpc(box, {'op': 'signal', 'id': v['id'], 'signal': body})


@contextmanager
def events(box, ident=None):
    path = box.root / ('events-' + secrets.token_hex(4))
    with path.open('wb') as output:
        proc = box.start([BINARY, 'events', '--jsonl', *(['--id', ident] if ident else [])],
                         stdout=output, stderr=subprocess.PIPE)
    def rows():
        return [json.loads(line) for line in path.read_text().splitlines() if line.endswith('}')]
    try:
        wait_for(lambda: bool(rows()) or proc.poll() is not None)
        if proc.poll() is not None:
            raise AssertionError(proc.communicate(timeout=3))
        yield proc, rows
    finally:
        if proc.poll() is None: proc.terminate()
        _, stderr = proc.communicate(timeout=5)
        if proc.returncode != 130 or b'DATA RACE' in stderr:
            raise AssertionError((proc.returncode, stderr))


class AgentEventsChecks(unittest.TestCase):
    def test_authenticated_signals_replay_freshness_rotation_and_privacy(self):
        with a.fixture() as box, a.coordinator(box):
            v = a.start(box, 'signals', ['/bin/sleep', '60'], extra=['--provider', 'generic'])
            self.assertEqual(v['attention'], 'unknown')
            with events(box, v['id']) as (_, rows):
                for change, code in [({'token': '0'*64}, 'denied'), ({'instance_id': '0'*32}, 'denied'),
                                     ({'generation': '0'*32}, 'denied'), ({'time': stamp(-121)}, 'invalid_usage'),
                                     ({'time': stamp(10)}, 'invalid_usage'), ({'state': 'done'}, 'invalid_usage')]:
                    self.assertEqual(signal_request(box, v, 1, 'working', **change)['error']['code'], code)
                self.assertIsNone(signal_request(box, v, 1, 'working')['error'])
                self.assertEqual(signal_request(box, v, 1, 'idle')['error']['code'], 'conflict')
                view = signal_request(box, v, 2, 'needs-input')['agents'][0]
                self.assertEqual(view['attention'], 'needs_input'); self.assertEqual(view['lifecycle'], 'running')
                self.assertEqual(view['attention_source'], 'hook:generic')
                wait_for(lambda: any(x['kind'] == 'attention_changed' for x in rows()))
                token = cred(box, v)['token']
                for data in [(a.state(box)/'registry.json').read_text(), json.dumps(rows()), json.dumps(view)]:
                    self.assertNotIn(token, data); self.assertNotIn('prompt', data)
                complete = signal_request(box, v, 3, 'turn-complete')['agents'][0]
                self.assertIsNotNone(complete['turn_complete_at']); self.assertEqual(complete['lifecycle'], 'running')
                old = cred(box, v)
                rotated = a.envelope(box, ['agent', 'hooks', 'enable', v['id']])['data']['agent']
                self.assertNotEqual(rotated['hooks']['generation'], old['generation'])
                self.assertFalse((a.state(box)/('h-'+old['generation']+'.json')).exists())
                stale = dict(instance_id=old['instance_id'], generation=old['generation'], token=old['token'], sequence=4, time=stamp(), state='idle')
                self.assertEqual(a.rpc(box, {'op':'signal','id':v['id'],'signal':stale})['error']['code'], 'denied')
                for path in a.state(box).iterdir(): self.assertEqual(path.stat().st_mode & 0o777, 0o600)

    def test_generic_managed_context_self_signals_and_environment_opt_in(self):
        with a.fixture() as box, a.coordinator(box):
            script = box.root/'send_hook.py'; ready=box.root/'ready'; release=box.root/'release'
            script.write_text(f'import os,subprocess,time,json\nfrom pathlib import Path\nassert "TXM_HOOK_TOKEN" not in os.environ\np=subprocess.run([{str(BINARY)!r},"agent","signal","--from-env","--state","needs-input"],capture_output=True,text=True)\nPath({str(ready)!r}+".debug").write_text(str(p.returncode)+p.stdout+p.stderr)\nassert p.returncode==0\nPath({str(ready)!r}).touch()\nwhile not Path({str(release)!r}).exists():time.sleep(.02)\n')
            v=a.start(box,'self',[sys.executable,str(script)],extra=['--provider','generic'])
            try: wait_for(ready.exists)
            except Exception:
                raise AssertionError((box.root/'ready.debug').read_text())
            result=a.envelope(box,['agent','wait',v['id'],'--until','needs-input','--timeout','2s'])
            self.assertEqual(result['data']['agent']['attention'],'needs_input')
            release.touch();a.envelope(box,['agent','wait',v['id'],'--timeout','3s'])
            output=box.root/'env';v=a.start(box,'plain',[sys.executable,'-c',f'import os;from pathlib import Path;Path({str(output)!r}).write_text(str([k for k in os.environ if k.startswith("TXM_AGENT") or k.startswith("TXM_HOOK")]))'])
            a.await_exit(box,v['id']);self.assertEqual(output.read_text(),'[]')

    def test_wait_exit_codes_timeout_cancellation_and_missing_hooks(self):
        with a.fixture() as box,a.coordinator(box):
            for label,argv in [('zero',['/usr/bin/true']),('nonzero',['/bin/sh','-c','exit 42']),('signal',[sys.executable,'-c','import os,signal;os.kill(os.getpid(),signal.SIGUSR1)'])]:
                v=a.start(box,label,argv);a.await_exit(box,v['id'])
                a.envelope(box,['agent','wait',v['id'],'--timeout','2s'])
                a.envelope(box,['agent','wait',v['id'],'--timeout','2s','--fail-on-nonzero'],0 if label=='zero' else 1)
            v=a.start(box,'live',['/bin/sleep','60'])
            a.envelope(box,['agent','wait',v['id'],'--timeout','100ms'],6)
            a.envelope(box,['agent','wait',v['id'],'--until','idle','--timeout','1s'],4)
            proc=box.start([BINARY,'--json','agent','wait',v['id']],stdout=subprocess.PIPE,stderr=subprocess.PIPE)
            time.sleep(.2);proc.terminate();out,err=proc.communicate(timeout=3)
            self.assertEqual(proc.returncode,130);self.assertEqual(json.loads(out)['errors'][0]['code'],'cancelled');self.assertFalse(err)
            for duration in ('0s','25h'):
                a.envelope(box,['agent','wait',v['id'],'--timeout',duration],2)
            with native_server(box,BINARY,name='unmanaged'):
                u=a.envelope(box,['agent','register','native/unmanaged'])['data']['agent']
                a.envelope(box,['agent','wait',u['id'],'--timeout','1s'],4)

    def test_atomic_initial_snapshot_and_future_transition(self):
        with a.fixture() as box,a.coordinator(box):
            v=a.start(box,'live',['/bin/sleep','60'],extra=['--provider','generic'])
            signal_request(box,v,1,'idle')
            a.envelope(box,['agent','wait',v['id'],'--until','idle','--timeout','1s'])
            proc=box.start([BINARY,'--json','agent','wait',v['id'],'--until','needs-input','--timeout','3s'],stdout=subprocess.PIPE,stderr=subprocess.PIPE)
            signal_request(box,v,2,'needs-input')
            out,err=proc.communicate(timeout=5);self.assertEqual(proc.returncode,0,(out,err))
            with events(box,v['id']) as (_,rows):
                first=rows()[0];self.assertEqual(first['kind'],'resync_required')
                self.assertEqual(first['metadata']['agents'][0]['attention'],'needs_input')
                signal_request(box,v,3,'working');signal_request(box,v,4,'idle')
                wait_for(lambda:any(x.get('metadata',{}).get('agent',{}).get('attention')=='idle' for x in rows()))
                seq=[x['sequence'] for x in rows()];self.assertEqual(seq,sorted(set(seq)))
                self.assertTrue(all(x['schema_version']==1 and x['coordinator_epoch']==first['coordinator_epoch'] for x in rows()))

    def test_missing_receipt_is_unknown_and_wait_fails(self):
        with a.fixture() as box,a.coordinator(box):
            v=a.start(box,'once',['/usr/bin/true']);a.await_exit(box,v['id'])
            (a.state(box)/('r-'+v['instance_id']+'.json')).unlink()
            result=a.envelope(box,['agent','wait',v['id'],'--timeout','4s'],1)
            self.assertEqual(result['data']['agent']['lifecycle'],'unknown');self.assertIsNone(result['data']['agent']['receipt'])

    def test_stale_attention_is_unknown_after_restart(self):
        with a.fixture() as box:
            with a.coordinator(box):
                v=a.start(box,'live',['/bin/sleep','60'],extra=['--provider','generic']);signal_request(box,v,1,'idle')
            registry=a.state(box)/'registry.json';data=json.loads(registry.read_text());data['agents'][0]['hook']['observed_at']=stamp(-121);registry.write_text(json.dumps(data))
            with a.coordinator(box):
                self.assertEqual(a.status(box,v['id'])['attention'],'unknown')
                a.envelope(box,['agent','wait',v['id'],'--until','idle','--timeout','100ms'],6)

    def test_reconnect_epoch_resync_never_reexecutes(self):
        with a.fixture() as box:
            try:
                with a.coordinator(box) as service:
                    count=box.root/'count';script=box.root/'once.py'
                    script.write_text(f'import time\nfrom pathlib import Path\np=Path({str(count)!r});p.write_text(p.read_text()+"x" if p.exists() else "x")\ntime.sleep(20)\n')
                    v=a.start(box,'live',[sys.executable,str(script)],extra=['--provider','generic']);wait_for(count.exists)
                    with events(box,v['id']) as (_,rows):
                        old=rows()[0]['coordinator_epoch'];service.kill();service.wait(timeout=4)
                        wait_for(lambda:any(x['coordinator_epoch']!=old and x['kind']=='resync_required' for x in rows()),timeout=10)
                        self.assertEqual(count.read_text(),'x');self.assertEqual(a.status(box,v['id'])['lifecycle'],'running')
            finally:
                a.envelope(box,['agent','stop-coordinator'])

    def test_subscriber_limit_and_cancellation_release(self):
        from contextlib import ExitStack
        with a.fixture() as box,a.coordinator(box):
            with ExitStack() as stack:
                for _ in range(8):stack.enter_context(events(box))
                out=box.run([BINARY,'events','--jsonl','--timeout','1s'],check=False)
                self.assertEqual(out.returncode,5);self.assertEqual(out.stdout,'')
                # Separate short-request budget remains usable.
                a.envelope(box,['agent','list'])
            with events(box):pass

    def test_slow_notification_action_deadline_and_cancellation(self):
        with a.fixture() as box,a.coordinator(box):
            one=a.start(box,'one',['/bin/sleep','60'],extra=['--provider','generic'])
            two=a.start(box,'two',['/bin/sleep','60'],extra=['--provider','generic'])
            output=box.root/'starts';script=box.root/'slow_action.py'
            script.write_text(f'import sys,json,time\nfrom pathlib import Path\np=Path({str(output)!r});p.open("a").write(json.loads(sys.stdin.read())["agent_id"]+"\\n")\ntime.sleep(10)\n')
            proc=box.start([BINARY,'agent','notify','--',sys.executable,str(script)],stdout=subprocess.PIPE,stderr=subprocess.PIPE)
            try:
                time.sleep(.3);signal_request(box,one,1,'needs-input');wait_for(output.exists)
                signal_request(box,two,1,'needs-input')
                wait_for(lambda:len(output.read_text().splitlines())==2,timeout=4)
                then=time.monotonic();proc.terminate();out,err=proc.communicate(timeout=3)
                self.assertLess(time.monotonic()-then,3);self.assertEqual(proc.returncode,130,(out,err))
            finally:
                if proc.poll() is None:proc.terminate();proc.communicate(timeout=4)

    @unittest.skipUnless(shutil.which('tmux'),'tmux unavailable')
    def test_tmux_generic_hook_context_wait_and_exit(self):
        with a.fixture() as box:
            box.start_tmux(shutil.which('tmux'))
            with a.coordinator(box):
                script=box.root/'tmux_hook.py';release=box.root/'release'
                script.write_text(f'import subprocess,time\nfrom pathlib import Path\nsubprocess.run([{str(BINARY)!r},"agent","signal","--from-env","--state","idle"],check=True)\nwhile not Path({str(release)!r}).exists():time.sleep(.02)\nraise SystemExit(42)\n')
                v=a.start(box,'hooked',[sys.executable,str(script)],backend='tmux',extra=['--provider','generic'])
                a.envelope(box,['agent','wait',v['id'],'--until','idle','--timeout','3s'])
                release.touch();a.envelope(box,['agent','wait',v['id'],'--timeout','3s'])
                self.assertEqual(a.status(box,v['id'])['receipt']['exit_code'],42);box.verify_tmux()

    def test_provider_bridge_session_binding_and_conservative_stop(self):
        with a.fixture() as box,a.coordinator(box):
            # Version probes are fixtures; this is schema coverage, not real provider execution.
            for provider,version in [('claude','2.1.308 (Claude Code)'),('claude','2.1.309 (Claude Code)'),('codex','codex-cli 0.165.0')]:
                box.executable(provider,'#!/bin/sh\nprintf "%s\\n" '+repr(version)+'\n')
                v=a.start(box,provider+('-new' if '309' in version else ''),['/bin/sleep','60'])
                v=a.envelope(box,['agent','hooks','enable',v['id'],'--provider',provider])['data']['agent']
                config=box.run([BINARY,'agent','hooks','config',v['id']]).stdout
                self.assertNotIn(cred(box,v)['token'],config);self.assertIn('SessionStart',json.loads(config)['hooks'])
                command=[BINARY,'agent','hook',v['id'],'--generation',v['hooks']['generation'],'--provider',provider]
                for event,expected,extra in [('SessionStart','unknown',{}),('UserPromptSubmit','working',{}),('PermissionRequest','unknown',{}),('PostToolUse','working',{}),('Stop','idle',{'stop_hook_active':False})]:
                    out=box.run(command,input=json.dumps(dict(session_id='session-A',hook_event_name=event,prompt='PRIVATE_PROMPT',transcript_path='/must/not/open',**extra)))
                    self.assertEqual(out.stdout,'{}\n');self.assertFalse(out.stderr)
                    current=a.status(box,v['id']);self.assertEqual(current['attention'],expected)
                    observation=current['provider_observation']
                    self.assertEqual(observation['identity']['generation'],v['hooks']['generation'])
                    self.assertEqual(observation['session_id'],'session-A')
                    self.assertEqual(observation['ordering'],'bridge_arrival')
                    self.assertEqual(current['lifecycle'],'running');self.assertIsNone(current['receipt'])
                    if event=='PermissionRequest':
                        self.assertEqual(observation['attention'],'permission_requested')
                    if event=='Stop':
                        self.assertIsNotNone(current['turn_complete_at'])
                        self.assertEqual(observation['turn_outcome'],'completed')
                        self.assertEqual(observation['local_turn'],1)
                    else:
                        self.assertIsNone(current['turn_complete_at'])
                before=a.status(box,v['id'])['hooks']['sequence']
                for body in [dict(session_id='session-A',hook_event_name='Stop',stop_hook_active=True),dict(session_id='session-B',hook_event_name='UserPromptSubmit'),None]:
                    out=box.run(command,input=json.dumps(body));self.assertEqual(out.stdout,'{}\n')
                self.assertEqual(a.status(box,v['id'])['hooks']['sequence'],before)
                self.assertNotIn('PRIVATE_PROMPT',(a.state(box)/'registry.json').read_text())
                self.assertNotIn('PRIVATE_PROMPT',(a.state(box)/('o-'+v['hooks']['generation']+'.json')).read_text())

    def test_doctor_defaults_to_discovery_without_state_or_provider_execution(self):
        with a.fixture() as box:
            marker=box.root/'probed'
            for provider in ('claude','codex','opencode'):
                box.executable(provider,'#!/bin/sh\n: > '+shlex.quote(str(marker))+'\necho future-version\n')
            report=a.envelope(box,['agent','doctor'])['data']
            self.assertEqual(report['coordinator'],'not_contacted');self.assertFalse(report['probed'])
            self.assertFalse(marker.exists())
            self.assertFalse((box.root/'state/txm').exists());self.assertFalse((box.root/'runtime/txm').exists())
            for provider in report['providers'][1:]:
                self.assertTrue(provider['available']);self.assertIsNone(provider['compatible'])
                self.assertFalse(provider['verified']);self.assertIsNone(provider['enabled'])
            report=a.envelope(box,['agent','doctor','--probe'])['data']
            self.assertTrue(marker.exists());self.assertTrue(report['probed'])
            for provider in report['providers'][1:]:
                self.assertFalse(provider['compatible']);self.assertFalse(provider['verified'])
            self.assertFalse((box.root/'state/txm').exists());self.assertFalse((box.root/'runtime/txm').exists())

    def test_claude_main_turn_survives_subagents_resume_failure_and_session_end(self):
        with a.fixture() as box,a.coordinator(box):
            box.executable('claude','#!/bin/sh\necho "2.1.308 (Claude Code)"\n')
            v=a.start(box,'claude-observer',['/bin/sleep','60'])
            v=a.envelope(box,['agent','hooks','enable',v['id'],'--provider','claude'])['data']['agent']
            config=json.loads(box.run([BINARY,'agent','hooks','config',v['id']]).stdout)
            self.assertEqual(set(config),{'hooks'})
            self.assertTrue({'PostToolUse','PostToolUseFailure','StopFailure','SessionEnd'}<=set(config['hooks']))
            for groups in config['hooks'].values():
                for group in groups:
                    for hook in group['hooks']:
                        self.assertEqual(hook['type'],'command');self.assertEqual(hook['timeout'],2)
                        self.assertNotIn(cred(box,v)['token'],hook['command'])
            command=[BINARY,'agent','hook',v['id'],'--generation',v['hooks']['generation'],'--provider','claude']
            def emit(event,**extra):
                body=dict(hook_event_name=event,session_id='main',**extra)
                result=box.run(command,input=json.dumps(body))
                self.assertEqual(result.stdout,'{}\n')
                return a.status(box,v['id'])
            emit('SessionStart',source='startup');current=emit('UserPromptSubmit')
            sequence=current['hooks']['sequence']
            current=emit('Stop',agent_id='subagent-1',stop_hook_active=False)
            self.assertEqual(current['hooks']['sequence'],sequence)
            self.assertEqual(current['attention'],'working')
            emit('PermissionRequest');emit('PostToolUse')
            current=emit('Stop',stop_hook_active=False)
            completed=current['turn_complete_at'];self.assertIsNotNone(completed)
            current=emit('SessionEnd',reason='other')
            self.assertEqual(current['turn_complete_at'],completed)
            self.assertEqual(current['lifecycle'],'running');self.assertIsNone(current['receipt'])
            emit('SessionStart',source='resume')
            current=emit('Stop',stop_hook_active=False)
            self.assertIsNone(current['turn_complete_at'])
            self.assertEqual(current['provider_observation']['turn_outcome'],'unknown')
            emit('UserPromptSubmit')
            current=emit('StopFailure',error='PRIVATE_ERROR')
            self.assertEqual(current['provider_observation']['turn_outcome'],'failed')
            self.assertIsNone(current['turn_complete_at'])
            self.assertNotIn('PRIVATE_ERROR',(a.state(box)/('o-'+v['hooks']['generation']+'.json')).read_text())

    def test_automatic_claude_new_version_uses_sidecar_and_preserves_explicit_settings(self):
        for version in ['2.1.308 (Claude Code)', '2.1.309 (Claude Code)']:
            with self.subTest(version=version), a.fixture() as box, a.coordinator(box):
                script = ('#!'+sys.executable+'\nimport json,subprocess,sys\nfrom pathlib import Path\n'
                          'if "--version" in sys.argv:print('+repr(version)+');raise SystemExit(0)\n'
                          'config=json.loads(Path(sys.argv[sys.argv.index("--settings")+1]).read_text())\n'
                          'for event,extra in [("SessionStart",{"source":"startup"}),("UserPromptSubmit",{"prompt_id":"p-one"}),("Stop",{"prompt_id":"p-one","stop_hook_active":False})]:\n'
                          ' command=config["hooks"][event][0]["hooks"][0]["command"]\n'
                          ' p=subprocess.run(command,shell=True,input=json.dumps(dict(hook_event_name=event,session_id="fixture",**extra)),capture_output=True,text=True)\n'
                          ' assert p.stdout=="{}\\n" and not p.stderr\n')
                claude=box.executable('claude',script)
                job=a.start(box,'automatic-claude',[str(claude),'--init-only'],extra=['--provider','claude'])
                current=a.await_exit(box,job['id'])
                self.assertEqual(current['receipt']['exit_code'],0)
                self.assertEqual(current['provider_observation']['turn_id'],'p-one')
                self.assertEqual(current['provider_observation']['turn_outcome'],'completed')
                record=json.loads((a.state(box)/'registry.json').read_text())['agents'][0]
                if '309' in version:
                    self.assertIsNone(record.get('hook'))
                    self.assertEqual(cred(box,current)['schema_version'],2)
                else:
                    self.assertEqual(record['hook']['version'],version)
        with a.fixture() as box:
            claude=box.executable('claude','#!/bin/sh\necho "2.1.309 (Claude Code)"\n')
            settings=box.root/'explicit.json';settings.write_text('{"hooks":{}}');before=settings.read_bytes()
            for args in [['--settings',str(settings)],['--settings={}']]:
                a.envelope(box,['--backend','native','agent','start','--provider','claude','refused','--',str(claude),*args],4)
            self.assertEqual(settings.read_bytes(),before)
            self.assertFalse((box.root/'state/txm').exists())
            self.assertFalse((box.root/'runtime/txm').exists())

    def test_codex_turn_compact_interrupt_resume_and_manual_trust_boundary(self):
        with a.fixture() as box:
            a.envelope(box,['--backend','native','agent','start','--provider','codex','auto','--','/usr/bin/true'],4)
            self.assertFalse((box.root/'state/txm').exists());self.assertFalse((box.root/'runtime/txm').exists())
        for version in ('codex-cli 0.165.0', 'codex-cli 0.166.0'):
            with self.subTest(version=version):
                with a.fixture() as box,a.coordinator(box):
                    box.executable('codex','#!/bin/sh\necho "' + version + '"\n')
                    v=a.start(box,'codex-observer',['/bin/sleep','60'])
                    v=a.envelope(box,['agent','hooks','enable',v['id'],'--provider','codex'])['data']['agent']
                    config=box.run([BINARY,'agent','hooks','config',v['id']]).stdout
                    self.assertIn('Interrupt',json.loads(config)['hooks'])
                    self.assertNotIn('PostToolUseFailure',json.loads(config)['hooks'])
                    self.assertNotIn('bypass',config);self.assertNotIn(cred(box,v)['token'],config)
                    self.assertFalse((box.root/'home/.codex').exists());self.assertFalse((box.root/'repo/.codex').exists())
                    command=[BINARY,'agent','hook',v['id'],'--generation',v['hooks']['generation'],'--provider','codex']
                    def emit(event,**extra):
                        result=box.run(command,input=json.dumps(dict(hook_event_name=event,session_id='thread',**extra)))
                        self.assertEqual(result.stdout,'{}\n')
                        return a.status(box,v['id'])
                    emit('SessionStart',source='startup');emit('UserPromptSubmit',turn_id='turn-1')
                    current=emit('SessionStart',source='compact')
                    self.assertEqual(current['provider_observation']['turn_id'],'turn-1')
                    self.assertTrue(current['provider_observation']['turn_active'])
                    current=emit('Stop',turn_id='turn-1',stop_hook_active=False)
                    self.assertEqual(current['provider_observation']['turn_outcome'],'completed')
                    current=emit('UserPromptSubmit',turn_id='turn-2')
                    sequence=current['hooks']['sequence']
                    current=emit('Stop',turn_id='turn-1',stop_hook_active=False)
                    self.assertEqual(current['hooks']['sequence'],sequence)
                    current=emit('Interrupt',turn_id='turn-2')
                    self.assertEqual(current['provider_observation']['turn_outcome'],'interrupted')
                    self.assertIsNone(current['turn_complete_at']);self.assertEqual(current['lifecycle'],'running')
                    emit('SessionStart',source='resume');current=emit('UserPromptSubmit',turn_id='turn-3')
                    sequence=current['hooks']['sequence']
                    current=emit('SessionStart',source='fork')
                    self.assertEqual(current['hooks']['sequence'],sequence)
                    current=emit('Stop',turn_id='turn-3',stop_hook_active=False)
                    completed=current['turn_complete_at'];self.assertIsNotNone(completed)
                    current=emit('SessionEnd',reason='other')
                    self.assertEqual(current['turn_complete_at'],completed)
                    self.assertIsNone(current['receipt']);self.assertEqual(current['lifecycle'],'running')

    def test_unsupported_providers_and_ordinary_independence(self):
        with a.fixture() as box:
            box.executable('opencode','#!/bin/sh\necho "opencode v2.2.1"\n')
            box.executable('claude','#!/bin/sh\necho "future unsupported"\n')
            value=a.envelope(box,['agent','providers'])
            self.assertFalse(next(p for p in value['data']['providers'] if p['name']=='opencode')['supported'])
            a.envelope(box,['agent','start','--provider','claude','bad','--','/usr/bin/true'],4)
            self.assertFalse(a.runtime(box).exists());self.assertFalse(a.state(box).exists())
            with native_server(box,BINARY):
                box.run([BINARY,'list','--json']);self.assertFalse(a.runtime(box).exists())

    def test_notifications_explicit_transcript_free_throttled_and_no_snapshot_replay(self):
        with a.fixture() as box,a.coordinator(box):
            v=a.start(box,'live',['/bin/sleep','60'],extra=['--provider','generic']);signal_request(box,v,1,'needs-input')
            output=box.root/'actions';script=box.root/'action.py'
            script.write_text(f'import sys\nfrom pathlib import Path\np=Path({str(output)!r});p.open("a").write(sys.stdin.read()+"\\n")\n')
            proc=box.start([BINARY,'agent','notify','--',sys.executable,str(script)],stdout=subprocess.PIPE,stderr=subprocess.PIPE)
            time.sleep(.4);self.assertFalse(output.exists())
            signal_request(box,v,2,'working');signal_request(box,v,3,'needs-input')
            wait_for(output.exists);signal_request(box,v,4,'turn-complete');time.sleep(.3)
            proc.terminate();out,err=proc.communicate(timeout=4);self.assertEqual(proc.returncode,130,(out,err))
            rows=[json.loads(x) for x in output.read_text().splitlines()];self.assertEqual(len(rows),1)
            self.assertEqual(set(rows[0]),{'schema_version','agent_id','managed_id','kind','time'})
            self.assertEqual(rows[0]['kind'],'needs_input')
            # Existing terminal state on a new subscription cannot repeat actions.
            proc=box.start([BINARY,'agent','notify','--',sys.executable,str(script)],stdout=subprocess.PIPE,stderr=subprocess.PIPE)
            time.sleep(.3);signal_request(box,v,5,'unknown');time.sleep(.2);proc.terminate();proc.communicate(timeout=3);self.assertEqual(len(output.read_text().splitlines()),1)

    def test_real_claude_init_only_sessionstart_no_model_or_auth(self):
        if not CLAUDE: self.skipTest("explicit real Claude path not supplied")
        with a.fixture() as box,a.coordinator(box):
            (box.root/'bin/claude').symlink_to(CLAUDE)
            box.env.update(CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC='1',DISABLE_AUTOUPDATER='1',DISABLE_TELEMETRY='1',CLAUDE_CONFIG_DIR=str(box.root/'home/.claude'))
            settings=[]
            for source,directory in [('user',box.root/'home/.claude'),('project',box.root/'repo/.claude')]:
                directory.mkdir()
                script=box.root/(source+'-hook.py')
                script.write_text('from pathlib import Path\nPath('+repr(str(box.root/(source+'-ran')))+').touch()\nprint("{}")\n')
                config=directory/'settings.json'
                config.write_text(json.dumps({'hooks':{'SessionStart':[{'hooks':[{'type':'command','command':shlex.join([sys.executable,str(script)]),'timeout':2}]}]}}))
                settings.append((config,config.read_bytes()))
            v=a.start(box,'real-claude',[str(box.root/'bin/claude'),'--init-only','--setting-sources','user,project','--permission-mode','default','--tools',''],extra=['--provider','claude',*sum((['--env',k] for k in ['CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC','DISABLE_AUTOUPDATER','DISABLE_TELEMETRY','CLAUDE_CONFIG_DIR']),[])])
            current=a.await_exit(box,v['id'],timeout=15)
            self.assertEqual(current['receipt']['exit_code'],0)
            self.assertGreaterEqual(current['hooks']['sequence'],1);self.assertTrue(current['hooks']['session_bound'])
            self.assertEqual(current['attention'],'unknown');self.assertIsNone(current['turn_complete_at'])
            self.assertFalse((box.root/'home/.claude/.credentials.json').exists())
            for source in ['user','project']:
                self.assertTrue((box.root/(source+'-ran')).exists())
            for config,before in settings:
                self.assertEqual(config.read_bytes(),before)

if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--binary',type=Path,required=True);parser.add_argument('--real-claude',type=Path)
    args,remaining=parser.parse_known_args();BINARY=args.binary.resolve(strict=True);a.BINARY=BINARY
    CLAUDE=args.real_claude.resolve(strict=True) if args.real_claude else None
    unittest.main(argv=[sys.argv[0],*remaining])
