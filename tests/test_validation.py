"""Guard the final validator and privacy scanner without using a live backend."""
import importlib.util
import io
import json
import os
import re
import subprocess
import sys
import textwrap
from pathlib import Path
import tempfile
import unittest
import warnings
import zipfile

REPO = Path(__file__).resolve().parents[1]


def module(name):
    spec = importlib.util.spec_from_file_location(name, REPO/'scripts'/(name+'.py'))
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


validator = module('validate-runtime')
scanner = module('audit-artifacts')
release_gate = module('check-release-ci')


class ValidationChecks(unittest.TestCase):
    def test_full_matrix_includes_new_control_task_and_metrics_boundaries(self):
        # Source CI owns full regressions; release jobs own packaged-code smoke.
        self.assertTrue({'mcp_checks', 'task_review_checks', 'metrics_checks'}
                        .issubset(validator.SUITES))
        self.assertEqual(len(validator.SUITES), len(set(validator.SUITES)))
        makefile = (REPO/'Makefile').read_text()
        runtime = makefile.split('test-runtime:\n', 1)[1].split('\n\n', 1)[0]
        for suite in validator.SUITES:
            self.assertIn('tests/'+suite+'.py --binary "$(TEST_BINARY)"', runtime)
        ci = (REPO/'.github/workflows/ci.yml').read_text()
        self.assertEqual(ci.count('make test-runtime '), 1)  # One matrix definition, two OS executions.
        self.assertEqual(ci.count('make test-harness'), 1)
        release = (REPO/'.github/workflows/release.yml').read_text()
        self.assertNotIn('for suite in ', release)
        self.assertIn('tests/archive_checks.py', ci)
        self.assertIn('tests/peer_checks.py', ci)
        self.assertNotIn('tests/archive_checks.py', release)
        self.assertNotIn('tests/peer_checks.py', release)

    def test_mac_jobs_use_latest_apple_silicon_and_one_universal_package(self):
        ci = (REPO/'.github/workflows/ci.yml').read_text()
        release = (REPO/'.github/workflows/release.yml').read_text()
        for workflow in [ci, release]:
            self.assertNotIn('macos-15', workflow)
            self.assertNotIn('macos-26-intel', workflow)
        self.assertEqual(ci.count('os: macos-latest'), 3)  # Build, runtime and package validation, all Apple Silicon.
        for suite in ['macos_backend_checks.py', 'ssh_checks.py']:
            command = next(line for line in ci.splitlines() if 'python3 tests/'+suite in line)
            self.assertIn('--report ', command)
        self.assertEqual(release.count('os: macos-latest'), 0)  # Release consumes validated CI artifacts.
        self.assertEqual(ci.count('archive: txm_Darwin_universal.zip'), 1)
        self.assertIn('archives: ci-macos-universal', ci)
        self.assertIn('name: ci-release-bundle', release)
        self.assertIn('name: ci-release-evidence', release)
        self.assertNotIn('uses: goreleaser/goreleaser-action', release)
        self.assertIn('run-id: ${{ needs.preflight.outputs.ci_run_id }}', release)
        self.assertNotIn('platform: darwin', release)
        self.assertIn('test "$RUNNER_ARCH" = ARM64', ci)
        setup = (REPO/'.github/actions/setup-build/action.yml').read_text()
        self.assertIn('[[ "$RUNNER_ARCH" == ARM64 ]]', setup)

    def test_ci_stage_dependencies_enforce_lint_tests_builds_then_validation(self):
        ci = (REPO/'.github/workflows/ci.yml').read_text().split('jobs:\n', 1)[1]
        chunks = re.split(r'^  ([a-z][a-z0-9-]*):\n', ci, flags=re.M)
        jobs = dict(zip(chunks[1::2], chunks[2::2]))
        expected = {'lint': {'scope'}, 'tests': {'scope', 'lint'}, 'build': {'scope', 'tests'},
                    'validate-runtime': {'scope', 'build'}, 'validate-archives': {'scope', 'build'},
                    'validate-packages': {'scope', 'validate-archives'},
                    'ready': {'scope', 'lint', 'tests', 'build', 'validate-runtime', 'validate-archives', 'validate-packages'}}
        for job, required in expected.items():
            value = re.search(r'^    needs: (.+)$', jobs[job], re.M)[1]
            actual = set(x.strip() for x in value.strip('[]').split(','))
            self.assertEqual(actual, required, job)
        self.assertNotIn('make test-', jobs['build'])
        self.assertIn('name: ci-release-bundle', jobs['validate-archives'])
        self.assertEqual(jobs['validate-runtime'].count('target:'), 2)

    def test_ci_ready_refuses_failed_cancelled_unknown_or_incomplete_stages(self):
        ci = (REPO/'.github/workflows/ci.yml').read_text()
        body = re.search(r"python3 - <<'PY_READY'\n(.*?)\n          PY_READY", ci, re.S)[1]
        body = textwrap.dedent(body)
        names = ['lint', 'tests', 'build', 'validate-runtime', 'validate-archives', 'validate-packages']
        def run(full='true', changes=None, scope='success'):
            stages = {name: {'result': 'success' if full == 'true' else 'skipped'} for name in names}
            stages['scope'] = {'result': scope, 'outputs': {'full_ci': full}}
            for name, value in (changes or {}).items(): stages[name]['result'] = value
            return subprocess.run([sys.executable, '-c', body],
                                  env=dict(os.environ, STAGES=json.dumps(stages)), capture_output=True).returncode
        self.assertEqual(run(), 0)
        self.assertEqual(run(full='false'), 0)
        for name in names:
            for result in ['failure', 'cancelled', 'skipped']:
                with self.subTest(stage=name, result=result):
                    self.assertNotEqual(run(changes={name: result}), 0)
        self.assertNotEqual(run(full='unknown'), 0)
        self.assertNotEqual(run(scope='failure'), 0)
        self.assertNotEqual(run(full='false', changes={'lint': 'success'}), 0)

    def test_release_gate_requires_every_expanded_ci_stage(self):
        ci = (REPO/'.github/workflows/ci.yml').read_text().split('jobs:\n', 1)[1]
        chunks = re.split(r'^  ([a-z][a-z0-9-]*):\n', ci, flags=re.M)
        names = []
        for body in chunks[2::2]:
            name = re.search(r'^    name: (.+)$', body, re.M)[1]
            variable = re.search(r'\$\{\{ matrix\.(\w+) \}\}', name)
            if variable:
                values = re.findall(r'^          - '+variable[1]+r': (.+)$', body, re.M)
                self.assertTrue(values)
                names.extend(name.replace(variable[0], value) for value in values)
            else: names.append(name)
        self.assertEqual(len(names), len(set(names)))
        self.assertEqual(set(names), release_gate.REQUIRED_JOBS)

    def test_extracts_only_exact_binary_and_rejects_missing_or_duplicate_members(self):
        with tempfile.TemporaryDirectory(prefix='txm-validator-') as temp:
            root = Path(temp)/'work'; root.mkdir(); archive = root/'payload.zip'
            with zipfile.ZipFile(archive, 'w') as payload:
                payload.writestr('txm', b'fixture executable')
                payload.writestr('../outside', b'must not extract')
            result = validator.extract_binary(archive, root/'txm')
            self.assertEqual(result.read_bytes(), b'fixture executable')
            self.assertFalse((root.parent/'outside').exists())
            with zipfile.ZipFile(archive, 'w') as payload:
                payload.writestr('other', b'no exact txm member')
            with self.assertRaises(ValueError):
                validator.extract_binary(archive, root/'absent')
            self.assertFalse((root/'absent').exists())
            with warnings.catch_warnings():
                warnings.simplefilter('ignore', UserWarning)
                with zipfile.ZipFile(archive, 'w') as payload:
                    payload.writestr('txm', b'one')
                    payload.writestr('txm', b'two')
            with self.assertRaises(ValueError):
                validator.extract_binary(archive, root/'duplicate')
            self.assertFalse((root/'duplicate').exists())

    def test_fingerprint_tracks_source_harness_and_runner(self):
        with tempfile.TemporaryDirectory(prefix='txm-fingerprint-') as temp:
            root = Path(temp)
            for directory in ['pkg', 'scripts', 'tests', 'docs']:
                (root/directory).mkdir()
            for name in ['main.go','go.mod','go.sum','VERSION','docs/embeds.go','docs/txm.1','scripts/validate.py','tests/fixture.py']:
                (root/name).write_text('fixture')
            before = validator.source_fingerprint(root)
            (root/'docs/notes.md').write_text('mutable documentation excluded')
            self.assertEqual(before, validator.source_fingerprint(root))
            (root/'tests/containers').mkdir()
            (root/'tests/containers/Dockerfile.linux').write_text('changed container recipe')
            self.assertNotEqual(before, validator.source_fingerprint(root))
            before = validator.source_fingerprint(root)
            (root/'VERSION').write_text('2.0.0')
            self.assertNotEqual(before, validator.source_fingerprint(root))
            before = validator.source_fingerprint(root)
            (root/'tests/fixture.py').write_text('changed harness')
            self.assertNotEqual(before, validator.source_fingerprint(root))

    def test_scanner_detects_split_credentials_without_echoing_them(self):
        key = b'AK'+b'IA'+b'0'*16
        data = b'\0'*((1 << 20)-8)+key+b'\0'
        findings = scanner.scan_stream(io.BytesIO(data))
        self.assertEqual(findings, [{'rule': 'aws-access-key', 'offset': (1 << 20)-8}])
        self.assertNotIn(key.decode(), repr(findings))
        self.assertEqual(scanner.scan_stream(io.BytesIO(b'PRIVATE_SYNTHETIC_PROMPT fixture')), [])

    def test_scanner_checks_compressed_zip_members(self):
        with tempfile.TemporaryDirectory(prefix='txm-privacy-') as temp:
            archive = Path(temp)/'payload.zip'
            key = b'gh'+b'p_'+b'x'*36
            with zipfile.ZipFile(archive, 'w', compression=zipfile.ZIP_DEFLATED) as payload:
                payload.writestr('fixture', key)
            result = scanner.audit([archive])
            self.assertFalse(result['passed'])
            self.assertTrue(any(f.get('member') == 'fixture' for f in result['findings']))
            self.assertNotIn(key.decode(), repr(result))


if __name__ == '__main__':
    unittest.main()
