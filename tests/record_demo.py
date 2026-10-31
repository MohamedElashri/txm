#!/usr/bin/env python3
"""Record actual generic CLI/PTY output in a private disposable fixture."""
import argparse
import fcntl
import hashlib
import json
import os
from pathlib import Path
import select
import shlex
import shutil
import struct
import termios
import time

import agent_checks as a
import extension_checks as x
from tty_state import attributes


def record(binary, destination):
    destination.mkdir(mode=0o700, parents=True, exist_ok=True)
    saved_binary=destination/"txm-demo"
    shutil.copy2(binary,saved_binary)
    binary_hash=hashlib.sha256(saved_binary.read_bytes()).hexdigest()
    events = []
    started = time.monotonic()
    def emit(text):
        events.append([round(time.monotonic()-started, 6), 'o', text])
    def at(seconds):
        remaining = seconds-(time.monotonic()-started)
        if remaining > 0:
            time.sleep(remaining)
    with a.fixture() as box:
        installed = box.root / 'bin/txm'
        shutil.copy2(saved_binary, installed); installed.chmod(0o700)
        a.BINARY = installed; x.BINARY = installed
        source = x.repo(box)
        worker = box.root / 'generic-worker'
        worker.write_text('#!/bin/sh\nset -eu\n' + '\n'.join([
            shlex.quote(str(installed))+' agent signal --from-env --state working >/dev/null',
            '/usr/bin/sleep 2',
            shlex.quote(str(installed))+' agent signal --from-env --state needs-input >/dev/null',
            '/usr/bin/sleep 18',
            shlex.quote(str(installed))+' agent signal --from-env --state working >/dev/null',
            '/usr/bin/sleep 2',
            shlex.quote(str(installed))+' agent signal --from-env --state turn-complete >/dev/null',
            '/usr/bin/sleep 2'])+'\n')
        worker.chmod(0o700)
        def run(second, args, structured=False):
            at(second); emit('\r\n$ '+shlex.join(['txm', *map(str,args)])+'\r\n')
            p = box.run([installed, *args], check=False, timeout=35)
            emit(p.stdout.replace('\n','\r\n')); emit(p.stderr.replace('\n','\r\n'))
            if p.returncode:
                raise AssertionError((args, p.returncode, p.stdout, p.stderr))
            return json.loads(p.stdout)['data'] if structured else p.stdout
        with a.coordinator(box):
            started = time.monotonic()
            emit('Persistent terminals. Supervised jobs. One CLI.\r\nDevelopment build; synthetic generic fixture; no provider account.\r\n')
            run(0, ['version'])
            run(3, ['--backend','native','session','create','demo-terminal','/usr/bin/sleep','70'])
            run(5, ['--backend','native','list','--json'])
            job = run(7, ['--backend','native','agent','start','demo-worker','--provider','generic',
                '--cwd',source,'--json','--',worker], True)['agent']
            at(12); emit('\r\n$ txm dashboard --backend native --agents\r\n')
            master, slave = os.openpty()
            fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH',40,140,0,0))
            original = attributes(slave)
            original_flags = fcntl.fcntl(slave, fcntl.F_GETFL)
            proc = box.start([installed,'dashboard','--backend','native','--agents'],stdin=slave,stdout=slave,stderr=slave,preexec_fn=lambda: fcntl.ioctl(0,termios.TIOCSCTTY,0))
            dashboard = bytearray()
            schedule = [(15,b']'),(17,b'g'),(19,b'g'),(23,b'q')]
            try:
                while proc.poll() is None:
                    elapsed = time.monotonic()-started
                    if schedule and elapsed >= schedule[0][0]:
                        _, key = schedule.pop(0)
                        if key == b'q':
                            (destination/'demo-dashboard.ansi').write_bytes(dashboard)
                        os.write(master,key)
                    ready,_,_ = select.select([master],[],[],0.05)
                    if ready:
                        chunk = os.read(master,65536); dashboard.extend(chunk); emit(chunk.decode('utf-8','replace'))
                    if elapsed > 30:
                        raise AssertionError('dashboard exit deadline')
                if proc.wait(timeout=3) != 0 or attributes(slave) != original or fcntl.fcntl(slave, fcntl.F_GETFL) != original_flags:
                    raise AssertionError(('dashboard failed or did not restore TTY',proc.returncode,repr(original),repr(attributes(slave)),original_flags,fcntl.fcntl(slave,fcntl.F_GETFL),dashboard[-1024:].decode('utf-8','replace')))
            finally:
                if proc.poll() is None:
                    proc.terminate(); proc.wait(timeout=6)
                os.close(master); os.close(slave)
            run(25, ['agent','wait',job['id'],'--until','exited','--timeout','10s','--json'])
            run(34, ['agent','status',job['id'],'--existing','--backend','native','--json'])
            task = run(38, ['--backend','native','task','start','demo-task','--repo',source,'--json','--','/usr/bin/true'], True)['task']
            run(40, ['agent','wait',task['id'],'--until','exited','--timeout','5s','--json'])
            run(43, ['task','review',task['id'],'--target','refs/heads/main','--json'])
            run(48, ['task','cleanup',task['id'],'--dry-run','--json'])
            run(52, ['task','cleanup',task['id'],'--json'])
            run(56, ['--backend','native','session','delete','demo-terminal'])
            at(59); emit('\r\nGeneric exit receipts verified. Owned task cleaned; branch retained.\r\nLive provider gates, ref-only merge and macOS metrics validation remain pending.\r\n')
            at(60)
    header = {'version':2,'width':140,'height':40,'timestamp':int(time.time()),
              'title':'txm development: generic supervised work','env':{'TERM':'xterm-256color'}}
    recording = destination/'generic-demo.cast'
    with recording.open('w') as output:
        output.write(json.dumps(header)+'\n')
        for event in events:
            output.write(json.dumps(event,ensure_ascii=False)+'\n')
    content = recording.read_text()
    for forbidden in ('PRIVATE_ENV_VALUE','PRIVATE_ARG_VALUE','Authorization:','api_key','access_token'):
        if forbidden in content:
            raise AssertionError('recording privacy check failed')
    report = {'duration_seconds':round(time.monotonic()-started,3),'events':len(events),
              'tty_restored':True,'fd_flags_restored':True,'privacy_check':True,'fixture_cleanup':'owned harness completed',
              'recording':str(recording),'provider_accounts':False,'binary_sha256':binary_hash,'binary':str(saved_binary)}
    (destination/'demo-report.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps(report))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(); parser.add_argument('--binary',type=Path,required=True)
    parser.add_argument('--destination',type=Path,required=True)
    args = parser.parse_args(); record(args.binary.resolve(strict=True),args.destination.resolve())
