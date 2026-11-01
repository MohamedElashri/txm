"""Release builds may reuse only successful CI of their exact trusted commit."""
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('release_ci', Path(__file__).resolve().parents[1]/'scripts/check-release-ci.py')
ci = importlib.util.module_from_spec(spec);spec.loader.exec_module(ci)
SHA = 'a'*40
REPO = 'owner/project'


def run(**changes):
    return dict({'id': 100, 'created_at': '2026-10-31T10:00:00Z', 'head_sha': SHA, 'head_branch': 'main',
                 'repository': {'full_name': REPO}, 'event': 'push', 'path': '.github/workflows/ci.yml',
                 'status': 'completed', 'conclusion': 'success'}, **changes)


class ReleaseCITests(unittest.TestCase):
    def test_successful_exact_main_commit_is_reused(self):
        self.assertEqual(ci.select_run([run()], REPO, SHA, 'main')['id'], 100)
        self.assertEqual(ci.select_run([run(event='workflow_dispatch')], REPO, SHA, 'main')['id'], 100)

    def test_other_commits_repositories_workflows_branches_and_prs_are_rejected(self):
        for change in [{'head_sha': 'b'*40}, {'repository': {'full_name': 'fork/project'}},
                       {'path': '.github/workflows/other.yml'}, {'head_branch': 'dev'}, {'event': 'pull_request'}]:
            with self.subTest(change=change), self.assertRaises(ValueError):
                ci.select_run([run(**change)], REPO, SHA, 'main')

    def test_newer_pending_failed_or_cancelled_ci_cannot_be_hidden_by_old_success(self):
        for change in [{'status': 'in_progress', 'conclusion': None}, {'conclusion': 'failure'}, {'conclusion': 'cancelled'}]:
            with self.subTest(change=change), self.assertRaises(ValueError):
                ci.select_run([run(), run(id=101, created_at='2026-10-31T11:00:00Z', **change)], REPO, SHA, 'main')

    def test_manual_branch_validation_is_explicit_and_never_qualifies_as_main_push(self):
        manual = run(head_branch='dev', event='workflow_dispatch')
        self.assertEqual(ci.select_run([manual], REPO, SHA, 'dev', manual=True)['id'], 100)
        with self.assertRaises(ValueError):ci.select_run([manual], REPO, SHA, 'dev')
        with self.assertRaises(ValueError):ci.select_run([run(head_branch='dev')], REPO, SHA, 'dev', manual=True)

    def test_missing_skipped_failed_or_duplicate_ci_jobs_block_builds(self):
        good = [{'name': name, 'status': 'completed', 'conclusion': 'success'} for name in sorted(ci.REQUIRED_JOBS)]
        ci.verify_jobs(good)
        for bad in [good[:-1], good+[good[0]], [dict(job, conclusion='skipped') for job in good],
                    [dict(job, conclusion='failure') for job in good]]:
            with self.subTest(jobs=bad), self.assertRaises(ValueError):ci.verify_jobs(bad)

    def test_universal_artifact_must_be_present_unexpired_and_bound_to_source(self):
        good = {'id': 7, 'name': ci.MACOS_ARTIFACT, 'expired': False, 'workflow_run': {'head_sha': SHA},
                'digest': 'sha256:'+'b'*64, 'size_in_bytes': 42}
        self.assertEqual(ci.select_macos_artifact([good], SHA)['id'], 7)
        for items in [[], [good, good], [dict(good, expired=True)], [dict(good, digest='')],
                      [dict(good, workflow_run={'head_sha': 'c'*40})], [dict(good, size_in_bytes=0)]]:
            with self.subTest(artifacts=items), self.assertRaises(ValueError):
                ci.select_macos_artifact(items, SHA)

    def test_readiness_requests_repository_scoped_jobs_and_artifacts(self):
        artifact = {'id': 7, 'name': ci.MACOS_ARTIFACT, 'expired': False,
                    'workflow_run': {'head_sha': SHA}, 'digest': 'sha256:'+'b'*64, 'size_in_bytes': 42}
        calls = []
        def request(endpoint):
            self.assertTrue(endpoint.startswith('repos/'+REPO+'/'), endpoint)
            calls.append(endpoint)
            if '/workflows/' in endpoint:
                return {'workflow_runs': [run(run_attempt=1, html_url='https://example.invalid/run')]}
            if '/jobs?' in endpoint:
                return {'jobs': [{'name': name, 'status': 'completed', 'conclusion': 'success'} for name in ci.REQUIRED_JOBS]}
            if '/artifacts?' in endpoint: return {'artifacts': [artifact]}
            self.fail('unexpected API route: '+endpoint)
        with tempfile.TemporaryDirectory(prefix='txm-readiness-') as temp:
            report = Path(temp)/'readiness.json'
            argv = ['check-release-ci', '--repository', REPO, '--sha', SHA, '--report', str(report)]
            with patch('sys.argv', argv), patch.object(ci, 'api', side_effect=request), patch('builtins.print'):
                ci.main()
            value = json.loads(report.read_text())
            self.assertTrue(value['passed']); self.assertEqual(value['macos_artifact']['id'], 7)
            self.assertEqual(len(calls), 3)


if __name__ == '__main__':unittest.main()
