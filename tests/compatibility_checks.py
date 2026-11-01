#!/usr/bin/env python3
"""Actual v2.0 coordinator/new client and persisted-state upgrade fixtures."""
import argparse
import json
from pathlib import Path
import unittest
import agent_checks as a
from agent_events_checks import cred, signal_request, stamp

BINARY = OLD_BINARY = None
class CompatibilityChecks(unittest.TestCase):
    def test_current_provider_sidecars_survive_tagged_coordinator_without_registry_migration(self):
        for provider,version in [('codex','codex-cli 0.166.0'),('claude','2.1.309 (Claude Code)')]:
            with a.fixture() as box:
                box.executable(provider,'#!/bin/sh\necho '+repr(version)+'\n')
                with a.coordinator(box):
                    job=a.start(box,provider+'-compatible',['/bin/sleep','60'])
                    registry=a.state(box)/'registry.json'
                    before=registry.read_bytes()
                    current=a.envelope(box,['agent','hooks','enable',job['id'],'--provider',provider])['data']['agent']
                    self.assertEqual(registry.read_bytes(),before)
                    self.assertIsNone(json.loads(before)['agents'][0].get('hook'))
                    self.assertEqual(cred(box,current)['schema_version'],2)
                    callback=[BINARY,'agent','hook',job['id'],'--generation',current['hooks']['generation'],'--provider',provider]
                    for event,extra in [('SessionStart',{'source':'startup'}),('UserPromptSubmit',{'turn_id':'turn-one'}),('Stop',{'turn_id':'turn-one','stop_hook_active':False})]:
                        response=box.run(callback,input=json.dumps({'hook_event_name':event,'session_id':'thread',**extra}))
                        self.assertEqual(response.stdout,'{}\n');self.assertFalse(response.stderr)
                    current=a.status(box,job['id'])
                    self.assertEqual(current['provider_observation']['turn_outcome'],'completed')
                    old=box.run([OLD_BINARY,'--json','agent','status',job['id']])
                    self.assertEqual(json.loads(old.stdout)['data']['agent']['lifecycle'],'running')
                with a.coordinator(box,binary=OLD_BINARY):
                    old=box.run([OLD_BINARY,'--json','agent','status',job['id']])
                    data=json.loads(old.stdout)['data']['agent']
                    self.assertEqual(data['lifecycle'],'running');self.assertIsNone(data.get('hooks'))
                with a.coordinator(box):
                    restored=a.status(box,job['id'])
                    self.assertEqual(restored['hooks']['generation'],current['hooks']['generation'])
                    self.assertEqual(restored['provider_observation']['observed_at'],current['provider_observation']['observed_at'])
                    # Explicit legacy rotation revokes and removes the newer secret.
                    old_generation=current['hooks']['generation']
                    a.envelope(box,['agent','hooks','enable',job['id'],'--provider','generic'])
                    self.assertFalse((a.state(box)/('h2-'+old_generation+'.json')).exists())
                    rejected=box.run(callback,input=json.dumps({'hook_event_name':'SessionStart','session_id':'thread','source':'resume'}))
                    self.assertEqual(rejected.stdout,'{}\n');self.assertIn('not accepted',rejected.stderr)

    def test_old_coordinator_rejects_new_ops_then_new_coordinator_upgrades_state(self):
        with a.fixture() as box:
            with a.coordinator(box, binary=OLD_BINARY):
                job = a.start(box, 'compatible', ['/bin/sleep','30'], extra=['--provider','generic'])
                self.assertIsNone(signal_request(box,job,1,'working')['error'])
                legacy = a.status(box,job['id'])
                self.assertEqual(legacy['attention'],'working')
                self.assertIsNone(legacy['provider_observation'])
                before = (a.state(box)/'registry.json').read_bytes()
                c = cred(box,job)
                reply = a.rpc(box, {'op':'provider-event','id':job['id'],
                    'signal':{'instance_id':c['instance_id'],'generation':c['generation'],
                              'token':c['token'],'sequence':2,'time':stamp(),'state':'unknown','session_id':''},
                    'provider_event':{'kind':'SessionStart','session_id':'fixture','start_source':'startup'}})
                self.assertIsNotNone(reply['error'])
                marker = box.root/'unexpected-launch'
                refused = box.run([BINARY,'--json','--backend','native','agent','start','unsupported-metrics',
                                   '--metrics','--','/usr/bin/touch',marker],check=False)
                self.assertNotEqual(refused.returncode,0)
                self.assertFalse(marker.exists())
                self.assertEqual((a.state(box)/'registry.json').read_bytes(),before)
            # The new coordinator reads the strict v1 registry/credential and
            # adds only a separate canonical sidecar on the next valid signal.
            with a.coordinator(box):
                current = a.status(box,job['id'])
                self.assertEqual(current['instance_id'],job['instance_id'])
                self.assertEqual(current['attention'],'working')
                self.assertIsNone(signal_request(box,current,2,'needs-input')['error'])
                current = a.status(box,job['id'])
                self.assertEqual(current['provider_observation']['attention'],'needs_input')
                stopped = a.envelope(box,['agent','stop',job['id'],'--confirm',job['id']])['data']['agent']
                self.assertEqual(stopped['lifecycle'],'exited')
                self.assertEqual(stopped['receipt']['instance_id'],job['instance_id'])
            # A tagged old CLI can still read the new coordinator's unchanged
            # strict v1 IPC projection; canonical fields stay in public output.
            with a.coordinator(box):
                response=box.run([OLD_BINARY,'--json','agent','status',job['id']])
                old=json.loads(response.stdout)['data']['agent']
                self.assertEqual(old['receipt'],stopped['receipt'])

if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--binary',required=True,type=Path)
    parser.add_argument('--old-binary',required=True,type=Path)
    args,rest=parser.parse_known_args();BINARY=args.binary.resolve(strict=True);OLD_BINARY=args.old_binary.resolve(strict=True)
    a.BINARY=BINARY
    unittest.main(argv=[__file__,*rest])
