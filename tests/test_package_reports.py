"""A passing job must not expose proof for a different or incomplete package."""
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('package_reports', Path(__file__).resolve().parents[1]/'.github/scripts/annotate-package-report.py')
reports = importlib.util.module_from_spec(spec); spec.loader.exec_module(reports)


class PackageReportTests(unittest.TestCase):
    def fixture(self):
        temp = tempfile.TemporaryDirectory(prefix='txm-package-proof-'); self.addCleanup(temp.cleanup)
        root = Path(temp.name); directory = root/'reports'; directory.mkdir()
        archive = root/'txm_Darwin_universal.zip'; archive.write_bytes(b'owned synthetic archive')
        install = {'passed': True, 'archive': str(archive),
                   'sha256': hashlib.sha256(archive.read_bytes()).hexdigest(), 'version': 'txm version 2.1.0',
                   'runtime_smoke': dict.fromkeys(['native_child', 'authenticated_signal', 'exit_receipt',
                                                  'receipt_after_coordinator_exit', 'child_metrics'], True)}
        peer = {'passed': True, 'tests_run': 2, 'skipped': []}
        (directory/'install.json').write_text(json.dumps(install))
        (directory/'peer-credentials.json').write_text(json.dumps(peer))
        return directory, archive, install, peer

    def test_proof_binds_exact_archive_source_and_report_bytes_without_paths(self):
        directory, archive, _, _ = self.fixture()
        rows = reports.summaries(directory, archive, '2.1.0', 'a'*40, 'macOS')
        self.assertEqual(len(rows), 2)
        for name, text in rows:
            row = json.loads(text)
            self.assertEqual(row['source_sha'], 'a'*40)
            self.assertEqual(row['report_sha256'], hashlib.sha256((directory/name).read_bytes()).hexdigest())
            self.assertEqual(row['archive_sha256'], hashlib.sha256(archive.read_bytes()).hexdigest())
            self.assertNotIn(str(directory.parent), text)

    def test_wrong_bytes_version_failed_or_incomplete_smoke_cannot_qualify(self):
        for key, value in [('sha256', 'b'*64), ('version', '2.0.0'), ('passed', False),
                           ('runtime_smoke', {'native_child': True})]:
            with self.subTest(field=key):
                directory, archive, install, _ = self.fixture()
                install[key] = value; (directory/'install.json').write_text(json.dumps(install))
                with self.assertRaises(ValueError):
                    reports.summaries(directory, archive, '2.1.0', 'a'*40, 'macOS')

    def test_missing_or_skipped_peer_checks_and_extra_reports_are_rejected(self):
        for change in ['missing', 'skipped', 'extra']:
            with self.subTest(change=change):
                directory, archive, _, peer = self.fixture()
                if change == 'missing': (directory/'peer-credentials.json').unlink()
                elif change == 'extra': (directory/'unexpected.json').write_text('{}')
                else:
                    peer['skipped'] = ['fixture']; (directory/'peer-credentials.json').write_text(json.dumps(peer))
                with self.assertRaises(ValueError):
                    reports.summaries(directory, archive, '2.1.0', 'a'*40, 'macOS')

    def test_windows_proof_uses_windows_paths_and_bare_version(self):
        directory, archive, install, _ = self.fixture()
        archive = archive.rename(archive.parent/'txm_Windows_arm64.zip')
        for path in directory.iterdir(): path.unlink()
        report = dict(install, archive='C:\\runner\\txm_Windows_arm64.zip', version='2.1.0')
        report.pop('runtime_smoke')
        path = directory/'windows-cli.json'; path.write_text(json.dumps(report))
        rows = reports.summaries(directory, archive, '2.1.0', 'a'*40, 'Windows')
        self.assertEqual(json.loads(rows[0][1])['archive'], archive.name)
        report['version'] = 'txm version 2.1.0'; path.write_text(json.dumps(report))
        with self.assertRaises(ValueError):
            reports.summaries(directory, archive, '2.1.0', 'a'*40, 'Windows')


if __name__ == '__main__':
    unittest.main()
