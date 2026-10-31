#!/usr/bin/env python3
"""Explicit metadata pruning never terminates sessions or deletes Git/exit data."""
import argparse
import json
from pathlib import Path
import select
import subprocess
import sys
import unittest
import agent_checks as a
import extension_checks as e
from sandbox import native_server,wait_for

BINARY=None

def cmd(box,args,code=0):return e.command(box,args,code)

def event(proc,timeout=6):
    ready,_,_=select.select([proc.stdout],[],[],timeout)
    if not ready:raise TimeoutError('event not received')
    line=proc.stdout.readline()
    if not line:raise AssertionError('event stream ended')
    return json.loads(line)


class PruneChecks(unittest.TestCase):
    def test_task_agent_pruning_preserves_branches_receipts_and_replay_guard(self):
        with a.fixture() as box,a.coordinator(box):
            e.repo(box)
            live=e.start(box,name='live')['task'];ident=live['id']
            cmd(box,['task','prune',ident,'--confirm',ident],7)
            cmd(box,['agent','prune',ident,'--confirm',ident],7)
            self.assertEqual(a.status(box,ident)['lifecycle'],'running')
            r=e.start(box,name='complete',argv=['/usr/bin/true'])['task'];ident=r['id'];a.await_exit(box,ident)
            cmd(box,['task','prune',ident,'--confirm',ident],7)
            cmd(box,['task','cleanup',ident])
            index=box.root/'state/txm/v1/tasks/index.json';before=index.read_bytes()
            cmd(box,['task','prune',ident,'--dry-run']);self.assertEqual(index.read_bytes(),before)
            cmd(box,['task','prune',ident],2);cmd(box,['task','prune',ident,'--confirm','0'*32],2)
            replacement=Path(r['worktree']);replacement.mkdir()
            cmd(box,['task','prune',ident,'--confirm',ident],7);replacement.rmdir()
            cmd(box,['task','prune',ident,'--confirm',ident]);cmd(box,['task','inspect',ident],3)
            receipt=a.state(box)/('r-'+r['instance_id']+'.json');data=receipt.read_bytes()
            registry=a.state(box)/'registry.json';before=registry.read_bytes()
            cmd(box,['agent','prune',ident,'--dry-run']);self.assertEqual(registry.read_bytes(),before)
            cmd(box,['agent','prune',ident],2)
            cmd(box,['agent','prune',ident,'--confirm',ident]);cmd(box,['agent','status',ident],3)
            self.assertEqual(receipt.read_bytes(),data);self.assertTrue(e.git(box,'rev-parse',r['branch']))
            body=dict(op='start',id=ident,backend='native',name='replay-must-not-run',cwd=str(box.root/'repo'),argv=['/usr/bin/true'],env_names=['HOME','PATH','TERM'])
            self.assertEqual(a.rpc(box,body)['error']['code'],'conflict')
            history=json.loads((a.state(box)/'pruned.json').read_text());self.assertIn(ident,history['ids'])
            self.assertEqual(a.status(box,live['id'])['lifecycle'],'running');cmd(box,['task','stop',live['id']])

    def test_unsupervised_prune_removes_registration_not_session(self):
        with a.fixture() as box,a.coordinator(box),native_server(box,BINARY,name='unmanaged') as (server,_,_):
            view=cmd(box,['agent','register','native/unmanaged'])['data']['agent'];ident=view['id']
            cmd(box,['agent','prune',ident,'--confirm',ident]);cmd(box,['agent','status',ident],3)
            self.assertIsNone(server.poll());self.assertIn('unmanaged',box.run([BINARY,'list']).stdout)

    def test_prune_resyncs_all_subscribers_and_ends_removed_id_stream(self):
        with a.fixture() as box,a.coordinator(box):
            view=a.start(box,'done',['/usr/bin/true']);ident=view['id'];a.await_exit(box,ident)
            all_events=box.start([BINARY,'events','--jsonl'],stdout=subprocess.PIPE,stderr=subprocess.PIPE)
            targeted=box.start([BINARY,'events','--jsonl','--id',ident],stdout=subprocess.PIPE,stderr=subprocess.PIPE)
            try:
                self.assertEqual(event(all_events)['kind'],'resync_required');self.assertEqual(event(targeted)['kind'],'resync_required')
                cmd(box,['agent','prune',ident,'--confirm',ident])
                found={}
                def removed():
                    value=event(all_events);found.update(value)
                    return value['kind']=='resync_required' and value['metadata'].get('reason')=='registry_pruned'
                wait_for(removed,timeout=8)
                self.assertNotIn(ident,[v['id'] for v in found['metadata'].get('agents',[])])
                targeted.communicate(timeout=8);self.assertEqual(targeted.returncode,3)
                self.assertIsNone(all_events.poll())
            finally:
                if all_events.poll() is None:all_events.terminate()
                all_events.communicate(timeout=5)
                if targeted.poll() is None:targeted.terminate()
                targeted.communicate(timeout=5)

    def test_corrupt_prune_history_blocks_agent_mutation_but_not_core(self):
        with a.fixture() as box,a.coordinator(box),native_server(box,BINARY,name='core_guard'):
            view=a.start(box,'done',['/usr/bin/true']);ident=view['id'];a.await_exit(box,ident)
            receipt=a.state(box)/('r-'+view['instance_id']+'.json');original=receipt.read_bytes();receipt.unlink()
            cmd(box,['agent','prune',ident,'--confirm',ident],7);receipt.write_bytes(original);receipt.chmod(0o600)
            cmd(box,['agent','prune',ident,'--confirm',ident])
            history=a.state(box)/'pruned.json';history.write_text('{bad')
            cmd(box,['agent','start','blocked','--','/usr/bin/true'],1)
            self.assertIn('core_guard',box.run([BINARY,'list']).stdout);self.assertEqual(history.read_text(),'{bad')


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--binary',required=True,type=Path);args,remaining=parser.parse_known_args()
    BINARY=args.binary.resolve(strict=True);a.BINARY=e.BINARY=BINARY
    unittest.main(argv=[sys.argv[0],*remaining])
