#!/usr/bin/env python3
"""Opt-in subscription-only Claude lifecycle controller; never runs on import."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import shlex
import stat
import struct
import sys

import agent_checks as a
from live_codex_checks import CONTROLLER, Fixture as NativeFixture, PromptBudget, save
from sandbox import wait_for
from wire import Peer, RESIZE, packet, read

VERSION = '2.1.309 (Claude Code)'
MARKERS = ('TXM_CLAUDE_ONE', 'TXM_CLAUDE_TWO', 'TXM_CLAUDE_RESUMED', 'TXM_CLAUDE_RECOVERED')
OBSERVER = '''import json,os,subprocess,sys,time
from pathlib import Path
root=Path(__file__).parent
try:
 raw=sys.stdin.buffer.read(65537);d=json.loads(raw);c=json.loads((root/'context.json').read_text())
 body={k:d[k] for k in ['hook_event_name','session_id','turn_id','prompt_id','source','stop_hook_active','agent_id','agent_type'] if k in d}
 row={k:v for k,v in body.items() if isinstance(v,(str,bool))}
 row.update(payload_keys=sorted(d),armed=c.get('armed',False),at=time.time())
 if isinstance(d.get('model'),str) and d['model'].startswith('claude-') and len(d['model'])<=128:row['model']=d['model']
 if d.get('permission_mode') in ['default','plan','acceptEdits','auto','dontAsk','bypassPermissions']:row['permission_mode']=d['permission_mode']
 if d.get('error') in ['rate_limit','overloaded','authentication_failed','invalid_request','model_not_found','server_error','unknown']:
  row['error_kind']=d['error']
 if row['armed']:
  env=dict(os.environ);env.update(c['environment'])
  p=subprocess.run([c['binary'],'agent','hook',c['agent_id'],'--generation',c['generation'],'--provider','claude'],input=json.dumps(body).encode(),env=env,capture_output=True,timeout=2)
  row.update(accepted=p.returncode==0 and not p.stderr,neutral=p.stdout.strip()==b'{}')
 with (root/'hooks.jsonl').open('a') as f:f.write(json.dumps(row)+'\\n')
except Exception:
 with (root/'hooks.jsonl').open('a') as f:f.write(json.dumps({'bridge_error':True,'at':time.time()})+'\\n')
print('{}')
'''


def owned_secret(path):
    info = path.lstat()
    if not stat.S_ISREG(info.st_mode) or info.st_uid != os.getuid() or stat.S_IMODE(info.st_mode) != 0o600:
        raise ValueError('authentication requires an owned mode0600 regular file')
    return path.read_bytes()


def ui_result(screen):
    """Keep categorical evidence, not prompts/transcripts or account banners."""
    return {
        'screen_sha256': hashlib.sha256(screen.encode()).hexdigest(),
        'trust_prompt': 'trust this folder' in screen.lower() or 'trust the files' in screen.lower(),
        'permission_prompt': 'Do you want to proceed?' in screen,
        'interrupted': 'Interrupted' in screen or 'interrupted' in screen,
        'api_error': 'API Error' in screen or 'Invalid model' in screen,
        'fixed_replies': [marker for marker in MARKERS
                          if re.search(r'^\s*[⏺●]\s*'+marker+r'\s*$', screen, re.M)],
    }


def claude_argv(binary, settings, session=None, model='sonnet'):
    argv = [str(binary), '--restricted', '--setting-sources', '', '--strict-mcp-config',
            '--tools', 'Read,Bash', '--settings', str(settings), '--model', model,
            '--effort', 'low', '--permission-mode', 'default', '--no-chrome']
    if session:
        argv += ['--resume', session]
    return argv


class Fixture(NativeFixture):
    def __init__(self, box, binary, claude, work):
        self.box, self.binary, self.claude, self.work, self.trust = box, binary, claude, work, True
        self.budget = PromptBudget(work/'prompt-budget.json', limit=6)
        self.launches = []
        directory = box.root/'home/.claude'
        directory.mkdir(mode=0o700)
        self.auth_source = Path.home()/'.claude/.credentials.json'
        self.auth_original = owned_secret(self.auth_source)
        auth = json.loads(self.auth_original)
        if auth.get('claudeAiOauth', {}).get('subscriptionType') != 'team':
            raise ValueError('only the previously authorized Claude team subscription is allowed')
        save(directory/'.credentials.json', auth)
        profile = json.loads((Path.home()/'.claude.json').read_text())
        save(box.root/'home/.claude.json', {k: profile[k] for k in
             ['oauthAccount', 'hasCompletedOnboarding', 'lastOnboardingVersion', 'hasAvailableSubscription'] if k in profile})
        box.env.update(CLAUDE_CONFIG_DIR=str(directory), CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC='1',
                       DISABLE_AUTOUPDATER='1', DISABLE_TELEMETRY='1')
        (box.root/'bin/claude').symlink_to(claude)
        if box.run([claude, '--version']).stdout.strip() != VERSION:
            raise ValueError('installed Claude version differs from reviewed fixture')
        box.run(['/usr/bin/git', 'init', '-q'])
        (box.root/'repo/known.txt').write_text('fixture\n')
        (box.root/'controller.py').write_text(CONTROLLER)
        self.job = a.start(box, 'claude-lifecycle', [sys.executable, str(box.root/'controller.py'), str(box.root)],
                           extra=sum((['--env', k] for k in [
                               'CLAUDE_CONFIG_DIR', 'CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC',
                               'DISABLE_AUTOUPDATER', 'DISABLE_TELEMETRY',
                               'XDG_STATE_HOME', 'XDG_RUNTIME_DIR', 'TMPDIR']), []))
        enabled = a.envelope(box, ['agent','hooks','enable', self.job['id'], '--provider','claude'])['data']['agent']
        self.generation = enabled['hooks']['generation']
        generated = json.loads(box.run([binary,'agent','hooks','config',self.job['id']]).stdout)
        (work/'observer.py').write_text(OBSERVER)
        command = shlex.join([sys.executable, str(work/'observer.py')])
        for groups in generated['hooks'].values():
            for group in groups:
                for hook in group['hooks']:
                    hook['command'] = command
        self.settings = box.root/'hooks.json'
        save(self.settings, generated)
        save(work/'review-hooks.json', generated)
        self.context = {'binary':str(binary), 'agent_id':self.job['id'], 'generation':self.generation,
                        'armed':True, 'environment':{k:box.env[k] for k in ['XDG_STATE_HOME','XDG_RUNTIME_DIR','TMPDIR']}}
        save(work/'context.json', self.context)
        self.socket = None
        def locate():
            for path in (box.root/'runtime/txm/v1').glob('s-*.sock'):
                try:
                    with Peer(path) as peer:
                        if peer.identity.get('name') == self.job['locator']['name']:
                            self.socket = path
                            return True
                except (OSError, EOFError):
                    pass
            return False
        wait_for(locate)
        with Peer(self.socket,'control') as peer:
            peer.socket.sendall(packet(RESIZE,2,struct.pack('!HH',140,45)))
            assert json.loads(read(peer.socket)[2])['error'] is None

    def launch(self, session=None, model='sonnet'):
        if self.launches and not (self.box.root/('exit-'+self.launches[-1]+'.json')).exists():
            raise ValueError('previous provider child is still running')
        ident = secrets.token_hex(8)
        save(self.box.root/'launch.json', {'id':ident, 'argv':claude_argv(self.claude,self.settings,session,model)})
        self.launches.append(ident)
        wait_for(lambda:(self.box.root/('launch-'+ident+'.reserved')).exists())
        return {'launch_id':ident}

    def command(self, value):
        if value['op'] == 'launch':
            return self.launch(value.get('session_id'), value.get('model','sonnet'))
        if value['op'] == 'snapshot':
            screen = self.snapshot()
            # Transient UI review is limited to synthetic fixture choices.
            lines = [line[:300] for line in screen.splitlines() if
                     any(word in line.lower() for word in ['trust', 'continue', 'proceed', 'yes', 'no,',
                     'permission', 'interrupted', 'api error', 'txm_claude_', 'esc', 'enter', 'theme'])]
            lines = [re.sub(r'[\w.+-]+@[\w.-]+', '<account>', line) for line in lines]
            return {'ui':ui_result(screen), 'review_lines':lines[-35:]}
        return super().command(value)

    def finish(self):
        result = super().finish()
        current = owned_secret(self.box.root/'home/.claude/.credentials.json')
        changed = json.loads(current) != json.loads(self.auth_original)
        result['auth_refreshed'] = changed
        result['original_auth_unchanged'] = owned_secret(self.auth_source) == self.auth_original
        if changed:
            # Never silently discard a rotated refresh token. Preserve privately
            # until a guarded compare-and-replace can maintain login continuity.
            path = self.work/'refreshed-auth.secret'
            fd = os.open(path, os.O_WRONLY|os.O_CREAT|os.O_EXCL, 0o600)
            with os.fdopen(fd,'wb') as output:
                output.write(current)
        save(self.work/'context.json', {'armed':False, 'retired':True})
        return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', required=True, type=Path)
    parser.add_argument('--claude', required=True, type=Path)
    parser.add_argument('--work-dir', required=True, type=Path)
    args = parser.parse_args()
    work = args.work_dir.resolve(strict=True)
    if work.stat().st_uid != os.getuid() or stat.S_IMODE(work.stat().st_mode) != 0o700 or (work/'prompt-budget.json').exists():
        parser.error('fresh owned mode0700 work directory and unused prompt budget required')
    a.BINARY = args.binary.resolve(strict=True)
    report = {'schema_version':1, 'scope':'authorized Claude subscription-only disposable lifecycle',
              'prompt_limit':6, 'normal_settings_changed':False, 'commands':[]}
    with a.fixture() as box, a.coordinator(box):
        f = Fixture(box, a.BINARY, args.claude.resolve(strict=True), work)
        f.launch()
        print(json.dumps({'ready':True}),flush=True)
        try:
            for line in sys.stdin:
                request = json.loads(line)
                if request['op'] == 'finish':
                    break
                try:
                    output = f.command(request)
                    report['commands'].append({'op':request['op'], 'result':
                        output['ui'] if request['op']=='snapshot' else output})
                    print(json.dumps(output),flush=True)
                except Exception as error:
                    print(json.dumps({'error':str(error)}),flush=True)
                save(work/'report.json',report)
        finally:
            report['cleanup'] = f.finish()
            save(work/'report.json',report)
    print(json.dumps({'finished':True}),flush=True)


if __name__ == '__main__':
    main()
