"""Documentation-only CI skips must not hide code changes or release validation."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location(
    'ci_changes', Path(__file__).resolve().parents[1]/'scripts/check-ci-changes.py')
ci = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ci)


class CIChangesTests(unittest.TestCase):
    def test_only_markdown_and_man_page_skip_full_ci(self):
        self.assertFalse(ci.needs_full_ci(['README.md', 'docs/user-guide.md', 'docs/txm.1']))
        for path in ['docs/embeds.go', 'docs/new.go', 'pkg/cmd/root.go', 'VERSION',
                     'Makefile', 'tests/test_release.py', '.github/workflows/ci.yml']:
            with self.subTest(path=path):
                self.assertTrue(ci.needs_full_ci(['docs/user-guide.md', path]))

    def test_manual_reusable_and_forced_runs_do_not_skip(self):
        with patch.object(ci.subprocess, 'run') as diff:
            for event_name in ['workflow_dispatch', 'workflow_call', 'unknown']:
                self.assertTrue(ci.select_scope(event_name, {}))
            self.assertTrue(ci.select_scope('pull_request', {}, force=True))
            diff.assert_not_called()

    def test_missing_invalid_or_unavailable_history_runs_full_ci(self):
        for event in [{}, {'before': '0'*40, 'after': 'a'*40},
                      {'before': '--output=README.md', 'after': 'a'*40}]:
            self.assertTrue(ci.select_scope('push', event))
        with patch.object(ci.subprocess, 'run', side_effect=subprocess.CalledProcessError(1, 'git')):
            self.assertTrue(ci.select_scope('push', {'before': 'a'*40, 'after': 'b'*40}))

    def test_real_git_ranges_cover_pushes_prs_deletions_and_renames(self):
        with tempfile.TemporaryDirectory(prefix='txm-ci-changes-') as temp:
            root = Path(temp)
            repo = root/'repo'
            repo.mkdir()
            (root/'home').mkdir()
            env = {'PATH': os.environ.get('PATH', os.defpath), 'HOME': str(root/'home'),
                   'GIT_CONFIG_NOSYSTEM': '1', 'GIT_CONFIG_GLOBAL': os.devnull,
                   'GIT_TERMINAL_PROMPT': '0', 'LC_ALL': 'C'}

            def git(*args):
                return subprocess.check_output(
                    ['git', '-C', str(repo), *args], env=env, text=True).strip()

            git('init', '-q', '--template=')
            git('config', 'user.name', 'CI fixture')
            git('config', 'user.email', 'ci@example.invalid')
            git('config', 'commit.gpgsign', 'false')

            def commit():
                git('add', '-A')
                git('commit', '-qm', 'fixture')
                return git('rev-parse', 'HEAD')

            (repo/'main.go').write_text('package main\n')
            base = commit()
            (repo/'README.md').write_text('# Documentation\n')
            docs = commit()
            event_path = root/'event.json'
            event_path.write_text(json.dumps({'before': base, 'after': docs}))
            output = root/'github-output'
            subprocess.check_output(
                [sys.executable, str(Path(ci.__file__))], cwd=repo,
                env=dict(env, GITHUB_EVENT_NAME='push', GITHUB_EVENT_PATH=str(event_path),
                         GITHUB_OUTPUT=str(output)), text=True)
            self.assertEqual(output.read_text(), 'full_ci=false\n')
            # The base branch changes code after the PR branches off. A PR must
            # compare against its merge base, while a push compares both tips.
            git('checkout', '-qb', 'fixture-base', base)
            (repo/'main.go').write_text('package main\n// base update\n')
            advanced = commit()
            real_run = subprocess.run

            def diff(*args, **kwargs):
                kwargs.setdefault('env', env)
                return real_run(*args, cwd=repo, **kwargs)

            with patch.object(ci.subprocess, 'run', side_effect=diff):
                self.assertFalse(ci.select_scope('push', {'before': base, 'after': docs}))
                pr = {'pull_request': {'base': {'sha': advanced}, 'head': {'sha': docs}}}
                self.assertFalse(ci.select_scope('pull_request', pr))
                self.assertTrue(ci.select_scope('push', {'before': advanced, 'after': docs}))
                git('checkout', '-q', '--detach', docs)
                (repo/'README.md').unlink()
                deleted = commit()
                self.assertFalse(ci.select_scope('push', {'before': docs, 'after': deleted}))
                git('mv', 'main.go', 'source.md')
                renamed = commit()
                self.assertTrue(ci.select_scope('push', {'before': deleted, 'after': renamed}))


if __name__ == '__main__':
    unittest.main()
