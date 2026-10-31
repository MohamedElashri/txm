#!/usr/bin/env python3
"""Explicit, bounded Codex lifecycle fixture; no live calls during unit discovery."""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import shlex
import stat
import struct
import subprocess
import sys
import time

import agent_checks as a
from sandbox import wait_for
from wire import Peer, RESIZE, packet, read

CONTROLLER = r'''import json,os,subprocess,sys,time
from pathlib import Path
root=Path(sys.argv[1]);seen=set();active=None
try:
 while not (root/'controller-stop').exists():
  path=root/'launch.json'
  if not path.exists():time.sleep(.02);continue
  request=json.loads(path.read_text());ident=request['id']
  if ident in seen:time.sleep(.02);continue
  if len(seen)>=8:raise RuntimeError('provider launch capacity')
  seen.add(ident)
  # Reserve before Popen. A repeated launch ID never executes twice.
  (root/('launch-'+ident+'.reserved')).touch(exist_ok=False)
  active=subprocess.Popen(request['argv'],cwd=root/'repo',env=dict(os.environ))
  while active.poll() is None:
   if (root/'controller-stop').exists():
    active.terminate()
    try:active.wait(timeout=5)
    except subprocess.TimeoutExpired:active.kill();active.wait(timeout=5)
    break
   time.sleep(.02)
  temp=root/'exit.tmp';temp.write_text(json.dumps({'id':ident,'exit_code':active.returncode}));temp.replace(root/('exit-'+ident+'.json'))
  active=None
finally:
 if active is not None and active.poll() is None:
  active.terminate()
  try:active.wait(timeout=5)
  except subprocess.TimeoutExpired:active.kill();active.wait(timeout=5)
'''


def save(path, value):
    temp = path.with_suffix('.tmp')
    temp.write_text(json.dumps(value))
    temp.chmod(0o600)
    temp.replace(path)


class PromptBudget:
    """Single controller, durable reservations, no retry after uncertain input."""
    def __init__(self, path, limit=4):
        self.path, self.limit = path, limit
        if not path.exists():
            save(path, [])

    def reserve(self, ident, text):
        rows = json.loads(self.path.read_text())
        if any(row['id'] == ident for row in rows):
            raise ValueError('prompt ID already reserved; inspect, never replay')
        if len(rows) >= self.limit:
            raise ValueError('live prompt allowance exhausted')
        rows.append({'id': ident, 'sha256': hashlib.sha256(text.encode()).hexdigest(),
                     'state': 'reserved_before_input', 'time': time.time()})
        save(self.path, rows)


def codex_argv(binary, session=None):
    # Resume owns its JSON/config flags. Do not reuse exec-only parent flags.
    argv = [str(binary), '--no-daemon', '--no-alt-screen']
    if session:
        argv += ['resume', session]
    return argv


def private_auth(source, destination):
    info = source.lstat()
    if not stat.S_ISREG(info.st_mode) or info.st_uid != os.getuid() or stat.S_IMODE(info.st_mode) != 0o600:
        raise ValueError('auth source must be an owned mode0600 regular file')
    value = json.loads(source.read_text())
    if value.get('auth_mode') != 'chatgpt' or value.get('OPENAI_API_KEY'):
        raise ValueError('only existing ChatGPT authentication is authorized')
    fd = os.open(destination, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, 'w') as output:
        json.dump(value, output)


def screen_lines(text):
    # Never publish a full provider screen, account banner or raw transcript.
    needles = ('trust', 'hook', 'review', 'sessionstart', 'sessionend', 'permissionrequest',
               'userpromptsubmit', 'posttooluse', 'stop', 'interrupt', 'txm_codex_',
               'continue', 'cancel', 'approve', 'allow', 'deny', 'reject', 'quit',
               'working', '/usr/bin/touch', 'read-only', 'needs', 'enable', 'disable')
    lines = []
    for line in text.splitlines():
        if any(word in line.lower() for word in needles) or re.match(r'^\s*[›>○●✓✕□☐☑]?[\s]*\d+[.)]', line):
            line = re.sub(r'[\w.+-]+@[\w.-]+', '<account>', line)
            line = re.sub(r'/tmp/txm-test-[^/\s]+', '<fixture>', line)
            lines.append(line[:300])
    return lines[-45:]


def report_output(op, output):
    if op != 'snapshot':
        return output
    text = '\n'.join(output['screen'])
    return {'screen_sha256': hashlib.sha256(text.encode()).hexdigest(),
            'line_count': len(output['screen']),
            'trust_prompt': 'Trust this folder?' in text,
            'hook_review': 'hooks need review' in text.lower(),
            'permission_prompt': 'Press enter to confirm or esc to cancel' in text,
            'interrupted': 'Conversation interrupted' in text,
            'fixed_replies': [x for x in ['TXM_CODEX_ONE','TXM_CODEX_TWO','TXM_CODEX_RESUMED']
                              if any(re.fullmatch(r'\s*•\s*'+re.escape(x)+r'\s*', line)
                                     for line in output['screen'])],
            'fixed_markers': [x for x in ['TXM_CODEX_ONE','TXM_CODEX_TWO','TXM_CODEX_RESUMED'] if x in text]}


class Fixture:
    def __init__(self, box, binary, codex, work, trust):
        self.box, self.binary, self.codex, self.work, self.trust = box, binary, codex, work, trust
        self.budget = PromptBudget(work/'prompt-budget.json')
        self.launches = []
        directory = box.root/'home/.codex'
        directory.mkdir(mode=0o700)
        private_auth(Path.home()/'.codex/auth.json', directory/'auth.json')
        box.env['CODEX_HOME'] = str(directory)
        (box.root/'bin/codex').symlink_to(codex)
        if box.run([codex,'--version']).stdout.strip() != 'codex-cli 0.166.0':
            raise ValueError('live fixture requires exact reviewed Codex0.166.0')
        config = ('forced_login_method = "chatgpt"\nsandbox_mode = "read-only"\n'
                  'approval_policy = "on-request"\nmodel_reasoning_effort = "low"\n')
        (directory/'config.toml').write_text(config)
        box.run(['/usr/bin/git', 'init', '-q'])
        (box.root/'controller.py').write_text(CONTROLLER)
        self.job = a.start(box, 'codex-lifecycle', [sys.executable, str(box.root/'controller.py'), str(box.root)],
                           extra=['--env','CODEX_HOME','--env','XDG_STATE_HOME','--env','XDG_RUNTIME_DIR','--env','TMPDIR'])
        enabled = a.envelope(box, ['agent','hooks','enable',self.job['id'],'--provider','codex'])['data']['agent']
        self.generation = enabled['hooks']['generation']
        generated = json.loads(box.run([binary,'agent','hooks','config',self.job['id']]).stdout)
        reviewed = json.loads((work/'review-hooks.json').read_text())
        if set(generated['hooks']) != set(reviewed['hooks']):
            raise ValueError('reviewed hooks differ from product event catalog')
        (directory/'hooks.json').write_text(json.dumps(reviewed))
        self.context = {'binary':str(binary),'agent_id':self.job['id'],'generation':self.generation,
                        'armed':False,'environment':{k:box.env[k] for k in ['XDG_STATE_HOME','XDG_RUNTIME_DIR','TMPDIR']}}
        save(work/'context.json',self.context)
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

    def launch(self, session=None):
        if self.launches and not (self.box.root/('exit-'+self.launches[-1]+'.json')).exists():
            raise ValueError('previous provider process is still running')
        ident = secrets.token_hex(8)
        save(self.box.root/'launch.json',{'id':ident,'argv':codex_argv(self.codex,session)})
        self.launches.append(ident)
        wait_for(lambda:(self.box.root/('launch-'+ident+'.reserved')).exists())
        return {'launch_id':ident}

    def snapshot(self):
        with Peer(self.socket) as peer:
            data = peer.request('snapshot',format='plain',lines=60,max_bytes=32768)
        return base64.b64decode(data['content']).decode(errors='replace')

    def input(self, data):
        with Peer(self.socket,'control') as peer:
            peer.request('input',data=base64.b64encode(data).decode())

    def command(self, value):
        op = value['op']
        if op == 'snapshot':
            return {'screen':screen_lines(self.snapshot())}
        if op == 'status':
            view = a.status(self.box,self.job['id'])
            return {k:view.get(k) for k in ['lifecycle','attention','turn_complete_at','provider_observation','observation_error','receipt']}
        if op == 'events':
            path = self.work/'hooks.jsonl'
            return {'events':[json.loads(x) for x in path.read_text().splitlines()] if path.exists() else []}
        if op == 'arm':
            self.context['armed'] = True
            save(self.work/'context.json',self.context)
            return {'armed':True}
        if op == 'launch':
            return self.launch(value.get('session_id'))
        if op == 'exits':
            return {'exits':[json.loads(x.read_text()) for x in self.box.root.glob('exit-*.json')]}
        if op == 'prompt':
            if not self.trust:
                raise ValueError('fixture trust grant required before model input')
            text = value['text']
            if not isinstance(text,str) or len(text)>600:
                raise ValueError('bounded synthetic prompt required')
            self.budget.reserve(value['id'],text)
            self.input(text.encode())
            # Codex's paste-burst protection can absorb an Enter delivered in
            # the same input frame. Submit separately without reserving again.
            time.sleep(.2)
            self.input(b'\r')
            return {'reserved':value['id']}
        if op == 'keys':
            # Escape/control cancellation never approves trust. Other keys need
            # the explicit fixture-only grant, including /hooks review input.
            data = base64.b64decode(value['data'],validate=True)
            if not self.trust and data not in [b'\x03',b'\x04',b'\x1b']:
                raise ValueError('fixture trust grant required for interactive choices')
            self.input(data)
            return {'sent_bytes':len(data)}
        raise ValueError('unknown fixture operation')

    def finish(self):
        (self.box.root/'controller-stop').touch()
        receipt = a.state(self.box)/('r-'+self.job['instance_id']+'.json')
        wait_for(receipt.exists,timeout=12)
        exits=[json.loads(x.read_text()) for x in self.box.root.glob('exit-*.json')]
        return {'managed_receipt':json.loads(receipt.read_text()),'provider_exits':exits,
                'all_launched_children_reaped':len(exits)==len(self.launches),
                'prompt_reservations':json.loads(self.budget.path.read_text())}


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary',required=True,type=Path)
    parser.add_argument('--codex',required=True,type=Path)
    parser.add_argument('--work-dir',required=True,type=Path)
    parser.add_argument('--allow-fixture-trust',action='store_true')
    args=parser.parse_args()
    work=args.work_dir.resolve(strict=True)
    if stat.S_IMODE(work.stat().st_mode)!=0o700 or work.stat().st_uid!=os.getuid():
        parser.error('private owned mode0700 work directory required')
    review=json.loads((work/'trust-review.json').read_text())
    for name,key in [('hook.py','observer_sha256'),('review-hooks.json','hooks_sha256')]:
        if hashlib.sha256((work/name).read_bytes()).hexdigest()!=review[key]:
            parser.error('reviewed hook bytes changed')
    if (work/'prompt-budget.json').exists():
        parser.error('new run directory/budget required; never reset a live allowance')
    a.BINARY=args.binary.resolve(strict=True)
    result={'schema_version':1,'scope':'disposable_codex_lifecycle','fixture_trust_granted':args.allow_fixture_trust,
            'normal_profile_modified':False,'commands':[]}
    with a.fixture() as box,a.coordinator(box):
        f=Fixture(box,a.BINARY,args.codex.resolve(strict=True),work,args.allow_fixture_trust)
        f.launch()
        print(json.dumps({'ready':True,'grant':args.allow_fixture_trust}),flush=True)
        try:
            for line in sys.stdin:
                value=json.loads(line)
                if value['op']=='finish':break
                try:
                    output=f.command(value)
                    result['commands'].append({'op':value['op'],'result':report_output(value['op'],output)})
                    print(json.dumps(output),flush=True)
                except Exception as error:
                    print(json.dumps({'error':str(error)}),flush=True)
                save(work/'report.json',result)
        finally:
            result['cleanup']=f.finish()
            save(work/'report.json',result)
    print(json.dumps({'finished':True,'report':str(work/'report.json')}),flush=True)


if __name__=='__main__':main()
