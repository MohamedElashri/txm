#!/usr/bin/env python3
"""Owned task setup/review fixtures; no operator repository or provider access."""
import argparse
import json
import sys
from pathlib import Path
import unittest

import agent_checks as a
import extension_checks as x
from mcp_checks import snapshot

BINARY = None


class TaskReviewChecks(unittest.TestCase):
    def test_setup_timeout_retains_worker_and_blocks_early_cleanup(self):
        with a.fixture() as box, a.coordinator(box):
            source=x.repo(box);ident='c'*32;base=x.git(box,'rev-parse','HEAD')
            worktree=box.root/'state/txm/v1/task-worktrees'/ident;marker=box.root/'main-must-not-run'
            policy=box.root/'timeout-setup.json'
            policy.write_text(json.dumps({'schema_version':1,'task_id':ident,'name':'timeout-setup',
                'repository':str(source),'worktree':str(worktree),'backend':'native','base_commit':base,
                'main_argv':['/usr/bin/touch',str(marker)],'main_env_names':['HOME','PATH','TERM'],
                'setup_argv':['/usr/bin/sleep','2'],'timeout_ms':500}));policy.chmod(0o600)
            args=['--backend','native','task','start','timeout-setup','--repo',str(source),'--setup-policy',str(policy)]
            digest=x.command(box,[*args,'--dry-run','--','/usr/bin/touch',str(marker)])['data']['setup_digest']
            failed=box.run([BINARY,'--json',*args,'--accept-setup',digest,'--','/usr/bin/touch',str(marker)],check=False)
            self.assertNotEqual(failed.returncode,0);self.assertTrue(worktree.is_dir());self.assertFalse(marker.exists())
            state=x.command(box,['task','setup-status',ident])['data']
            self.assertEqual(state['state'],'uncertain');self.assertFalse(state['main_launch_intent'])
            refused=box.run([BINARY,'--json','task','cleanup',ident,'--dry-run'],check=False)
            self.assertNotEqual(refused.returncode,0);self.assertTrue(worktree.is_dir())
            a.await_exit(box,state['managed_id'])
            x.command(box,['task','cleanup',ident,'--dry-run'])
            self.assertFalse(marker.exists());self.assertTrue(worktree.is_dir())

    def test_lost_git_acknowledgement_never_reapplies_uncertain_intent(self):
        with a.fixture() as box, a.coordinator(box):
            source = x.repo(box)
            attempts = box.root / 'merge-attempts'
            wrapper = box.root / 'bin/git'
            wrapper.write_text(f"""#!{sys.executable}
import os,sys,subprocess,signal
from pathlib import Path
env={{k:v for k,v in os.environ.items() if not k.startswith('GIT_')}}
env.update(GIT_CONFIG_NOSYSTEM='1',GIT_CONFIG_GLOBAL='/dev/null')
if 'merge' in sys.argv[1:]:
 p=Path({str(attempts)!r});p.write_text(p.read_text()+'attempt\\n' if p.exists() else 'attempt\\n')
 result=subprocess.run([{x.GIT!r},*sys.argv[1:]],env=env)
 if result.returncode==0:os.kill(os.getpid(),signal.SIGKILL)
 sys.exit(result.returncode)
os.execve({x.GIT!r},[{x.GIT!r},*sys.argv[1:]],env)
"""); wrapper.chmod(0o700)
            job = x.start(box, 'uncertain', ['/usr/bin/true']); ident = job['task']['id']
            a.await_exit(box, ident)
            wt = Path(job['task']['worktree']); (wt/'tracked').write_text('frozen change\n')
            x.git(box, 'add', 'tracked', repo=wt); x.git(box, 'commit', '-m', 'work', repo=wt)
            dry = x.command(box, ['task','merge',ident,'--target','refs/heads/main'])['data']
            review = dry['review']
            args = [BINARY,'--json','task','merge',ident,'--target','refs/heads/main','--dry-run=false',
                '--quiescent','--confirm',ident,'--expected-task',review['task_commit'],
                '--expected-target',review['target_commit'],'--review-digest',dry['review_digest']]
            first = box.run(args,check=False); self.assertNotEqual(first.returncode,0)
            self.assertEqual(json.loads(first.stdout)['data']['state'],'uncertain')
            self.assertEqual(x.git(box,'rev-parse','HEAD'),review['task_commit'])
            repeated = box.run(args,check=False); self.assertNotEqual(repeated.returncode,0)
            self.assertEqual(json.loads(repeated.stdout)['data']['state'],'uncertain')
            self.assertEqual(attempts.read_text().splitlines(),['attempt'])

    def test_checked_out_merge_requires_frozen_review_and_quiescence(self):
        with a.fixture() as box, a.coordinator(box):
            source = x.repo(box)
            job = x.start(box, 'merge', ['/usr/bin/true'])
            ident = job['task']['id']; a.await_exit(box, ident)
            wt = Path(job['task']['worktree'])
            (wt / 'tracked').write_text('reviewed change\n')
            x.git(box, 'add', 'tracked', repo=wt); x.git(box, 'commit', '-m', 'reviewed', repo=wt)
            marker = box.root / 'merge-hook-ran'
            hook = source / '.git/hooks/post-merge'
            hook.write_text('#!/bin/sh\ntouch ' + str(marker) + '\n'); hook.chmod(0o700)
            dry = x.command(box, ['task', 'merge', ident, '--target', 'refs/heads/main'])['data']
            review = dry['review']
            args = ['task', 'merge', ident, '--target', 'refs/heads/main', '--dry-run=false',
                    '--expected-task', review['task_commit'], '--expected-target', review['target_commit'],
                    '--review-digest', dry['review_digest'], '--confirm', ident]
            denied = box.run([BINARY, '--json', *args], check=False)
            self.assertNotEqual(denied.returncode, 0)
            self.assertEqual(x.git(box, 'rev-parse', 'HEAD'), review['target_commit'])
            # An unreviewed target edit invalidates acceptance before Git starts.
            (source / 'dirty').write_text('untracked\n')
            dirty = box.run([BINARY, '--json', *args, '--quiescent'], check=False)
            self.assertNotEqual(dirty.returncode, 0); (source / 'dirty').unlink()
            self.assertFalse((box.root / 'state/txm/v1/tasks' / ('merge-'+ident+'.json')).exists())
            applied = x.command(box, [*args, '--quiescent'])['data']
            self.assertEqual(applied['state'], 'applied')
            self.assertEqual(x.git(box, 'rev-parse', 'HEAD'), review['task_commit'])
            self.assertEqual((source / 'tracked').read_text(), 'reviewed change\n')
            self.assertFalse(marker.exists())
            self.assertEqual(x.command(box, [*args, '--quiescent'])['data'], applied)
            self.assertTrue(wt.is_dir())

    def test_ref_only_merge_is_explicitly_disabled(self):
        with a.fixture() as box, a.coordinator(box):
            x.repo(box); job = x.start(box, 'ref-only', ['/usr/bin/true'])
            ident = job['task']['id']; a.await_exit(box, ident)
            x.git(box, 'branch', 'unseen-target')
            dry = x.command(box, ['task', 'merge', ident, '--target', 'refs/heads/unseen-target'])['data']
            self.assertEqual(dry['review']['merge_mode'], 'manual_ref_only')
            before = x.git(box, 'rev-parse', 'refs/heads/unseen-target')
            refused = box.run([BINARY, '--json', 'task', 'merge', ident, '--target', 'refs/heads/unseen-target',
                '--dry-run=false', '--quiescent', '--confirm', ident, '--expected-task', dry['review']['task_commit'],
                '--expected-target', before, '--review-digest', dry['review_digest']], check=False)
            self.assertNotEqual(refused.returncode, 0)
            self.assertEqual(json.loads(refused.stdout)['errors'][0]['code'], 'unsupported')
            self.assertEqual(x.git(box, 'rev-parse', 'refs/heads/unseen-target'), before)

    def test_review_is_pure_and_diff_is_explicit(self):
        with a.fixture() as box:
            source = x.repo(box)
            with a.coordinator(box):
                job = x.start(box, 'review', ['/usr/bin/true'])
                ident = job['task']['id']; a.await_exit(box, ident)
            before = snapshot(box)
            review = x.command(box, ['task', 'review', ident, '--target', 'refs/heads/main'])['data']
            self.assertEqual(review['compare_from'], job['task']['base_commit'])
            self.assertEqual(review['compare_to'], review['task_commit'])
            self.assertNotIn('diff', review)
            self.assertTrue(review['fast_forward']); self.assertFalse(review['task_dirty'])
            self.assertEqual({k: v for k, v in snapshot(box).items() if before.get(k) != v}, {})
            self.assertEqual(Path(review['target_worktree']), source)
            self.assertFalse((a.runtime(box) / 'coordinator.json').exists())

    def test_setup_success_failure_and_changed_acceptance(self):
        for success in (True, False):
            with self.subTest(success=success), a.fixture() as box, a.coordinator(box):
                source = x.repo(box); base = x.git(box, 'rev-parse', 'HEAD')
                ident = ('a' if success else 'b') * 32
                worktree = box.root / 'state/txm/v1/task-worktrees' / ident
                policy = box.root / 'setup.json'
                document = {'schema_version': 1, 'task_id': ident, 'name': 'setup-fixture',
                    'repository': str(source), 'worktree': str(worktree), 'backend': 'native',
                    'base_commit': base, 'main_argv': ['/usr/bin/true'],
                    'main_env_names': ['HOME', 'PATH', 'TERM'],
                    'setup_argv': ['/usr/bin/true' if success else '/usr/bin/false'], 'timeout_ms': 5000}
                policy.write_text(json.dumps(document)); policy.chmod(0o600)
                args = ['--backend', 'native', 'task', 'start', 'setup-fixture', '--repo', str(source),
                        '--setup-policy', str(policy)]
                dry = x.command(box, [*args, '--dry-run', '--', '/usr/bin/true'])['data']
                self.assertEqual(dry['reserved_id'], ident)
                digest = dry['setup_digest']; self.assertEqual(len(digest), 64)
                denied = box.run([BINARY, '--json', *args, '--accept-setup', '0'*64, '--', '/usr/bin/true'], check=False)
                self.assertEqual(denied.returncode, 8)
                self.assertFalse(worktree.exists())
                result = box.run([BINARY, '--json', *args, '--accept-setup', digest, '--', '/usr/bin/true'], check=False)
                self.assertEqual(result.returncode == 0, success, (result.stdout, result.stderr))
                status = x.command(box, ['task', 'setup-status', ident])['data']
                self.assertEqual(status['state'], 'succeeded' if success else 'failed')
                self.assertEqual(status['receipt']['exit_code'], 0 if success else 1)
                self.assertEqual(status['main_launch_intent'], success)
                self.assertTrue(worktree.is_dir())
                if success:
                    a.await_exit(box, ident)
                else:
                    inspect = x.command(box, ['task', 'inspect', ident])['data']
                    self.assertTrue(inspect['orphan']); self.assertIsNone(inspect['agent'])
                x.command(box, ['task', 'cleanup', ident, '--dry-run'])


if __name__ == '__main__':
    parser = argparse.ArgumentParser(); parser.add_argument('--binary', required=True, type=Path)
    args, rest = parser.parse_known_args()
    BINARY = args.binary.resolve(strict=True); a.BINARY = BINARY; x.BINARY = BINARY
    unittest.main(argv=[__file__, *rest])
