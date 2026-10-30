#!/usr/bin/env python3
"""Explicit synthetic recording only. No recording path is used by txm itself."""
import argparse
import codecs
import json
from pathlib import Path
import threading
import time

import dashboard_checks as d
from wire import Peer


def main():
    p=argparse.ArgumentParser();p.add_argument('--binary',type=Path,required=True);p.add_argument('--output',type=Path,required=True);args=p.parse_args()
    d.BINARY=args.binary.resolve(strict=True);d.a.BINARY=d.BINARY
    events=[];lock=threading.Lock();size=0
    def add(when,kind,value):
        nonlocal size
        with lock:
            size+=len(value.encode())
            if size>2*1024*1024:raise RuntimeError('synthetic recording exceeded bound')
            events.append([round(when,6),kind,value])
    with d.fixture() as box,d.a.coordinator(box):
        first=d.a.start(box,'review-one',['/bin/sh','-c','printf "SYNTHETIC OUTPUT ONLY\\n";exec /bin/sleep 120'],extra=['--provider','generic'])
        release=box.root/'release';script=box.root/'synthetic.py'
        script.write_text(f'import time\nfrom pathlib import Path\nwhile not Path({str(release)!r}).exists():time.sleep(.02)\nraise SystemExit(42)\n')
        second=d.a.start(box,'review-two',[d.sys.executable,str(script)],extra=['--provider','generic'])
        with d.console(box,['--backend','native','--agents']) as ui:
            ui.search('review');ui.keys(b'\t\t');ui.wait('managed (supervised)')
            with Peer(ui.sock,'observer') as peer:
                start=time.monotonic();initial=peer.attach().decode('utf-8','replace');add(0,'o',initial)
                stopped=threading.Event();failures=[]
                def capture():
                    decoder=codecs.getincrementaldecoder('utf-8')('replace')
                    try:
                        while not stopped.is_set():
                            chunk=peer.recv_data()
                            if not chunk:break
                            value=decoder.decode(chunk)
                            if value:add(time.monotonic()-start,'o',value)
                    except (EOFError,ConnectionError,OSError):pass
                    except BaseException as exc:failures.append(str(exc))
                thread=threading.Thread(target=capture);thread.start()
                try:
                    time.sleep(.5)
                    d.signal_request(box,first,1,'working');ui.wait('working');time.sleep(.6)
                    d.signal_request(box,first,2,'needs-input');ui.search('review-one');ui.wait('hook:generic');ui.wait('needs_input');time.sleep(.8)
                    d.signal_request(box,first,3,'turn-complete');ui.wait('turn complete: 20');time.sleep(.8)
                    ui.search('review-two');release.touch();d.a.await_exit(box,second['id']);ui.wait('code=42');time.sleep(.8)
                    ui.keys(b'\t'*5);ui.search('review-one');ui.keys(b'p');ui.wait('SYNTHETIC OUTPUT ONLY');time.sleep(.8)
                    ui.keys(b'o');time.sleep(.5);ui.keys(b'\x1c');ui.wait('Detached; dashboard resumed');time.sleep(.5)
                    ui.keys(b'q');ui.exited()
                finally:
                    stopped.set();peer.socket.shutdown(2);thread.join(timeout=3)
                assert not thread.is_alive() and not failures,failures
    events.sort(key=lambda row:row[0]);assert events and events[-1][0]>5
    # Every label/output is fixture data; no user path, prompt, credential or provider input.
    encoded='\n'.join(json.dumps(row,ensure_ascii=False) for row in events)
    assert '/home/' not in encoded and 'token' not in encoded and 'PRIVATE_' not in encoded
    header={'version':2,'width':160,'height':50,'title':'txm Phase 6 — synthetic agents, truthful state and read-only inspection','env':{'TERM':'xterm-256color'},'duration':events[-1][0]}
    args.output.parent.mkdir(parents=True,exist_ok=True)
    args.output.write_text(json.dumps(header,ensure_ascii=False)+'\n'+encoded+'\n')
    print(json.dumps({'output':str(args.output),'seconds':events[-1][0],'events':len(events),'bytes':args.output.stat().st_size,'synthetic_only':True}))

if __name__=='__main__':main()
