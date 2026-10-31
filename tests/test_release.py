"""Release identity and published notes must agree before any tag is shipped."""
import importlib.util
from pathlib import Path
import tempfile
import unittest

REPO = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('release_metadata', REPO/'scripts/check-release.py')
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


class ReleaseMetadataTests(unittest.TestCase):
    def fixture(self, version='2.0.0'):
        temp = tempfile.TemporaryDirectory(prefix='txm-release-test-')
        self.addCleanup(temp.cleanup)
        repo = Path(temp.name)
        (repo/'docs').mkdir()
        (repo/'VERSION').write_text(version+'\n')
        (repo/'docs/CHANGELOG.md').write_text('''# Changes

## [Unreleased]

Future work must stay out of these release notes.

## [2.0.0] - 2026-10-31

### Upgrade notes

Keep the old binary. See [validation](development.md#release-process).

## [1.3.1] - 2026-09-17

Old changes must stay out of these release notes.
''')
        return repo

    def test_notes_include_only_this_release_and_resolve_links_from_a_release_page(self):
        version, notes = release.release_notes(self.fixture(), 'v2.0.0')
        self.assertEqual(version, '2.0.0')
        self.assertIn('Keep the old binary', notes)
        self.assertIn('https://github.com/MohamedElashri/txm/blob/v2.0.0/docs/development.md#release-process', notes)
        self.assertNotIn('Future work', notes)
        self.assertNotIn('Old changes', notes)

    def test_stable_mode_rejects_prerelease_identity(self):
        repo = self.fixture()
        self.assertEqual(release.release_notes(repo, 'v2.0.0', stable=True)[0], '2.0.0')
        (repo/'VERSION').write_text('2.0.0-rc.1\n')
        with self.assertRaisesRegex(ValueError, 'stable release'):
            release.release_notes(repo, stable=True)

    def test_wrong_tag_cannot_publish_notes_for_a_different_version(self):
        with self.assertRaisesRegex(ValueError, 'tag must match'):
            release.release_notes(self.fixture(), 'v1.3.1')

    def test_missing_or_duplicate_release_section_is_rejected(self):
        repo = self.fixture('2.0.1')
        with self.assertRaisesRegex(ValueError, 'exactly one'):
            release.release_notes(repo)
        (repo/'VERSION').write_text('2.0.0\n')
        changelog = repo/'docs/CHANGELOG.md'
        changelog.write_text(changelog.read_text()+'\n## [2.0.0] - 2026-10-31\n\n### Upgrade notes\n\nDuplicate\n')
        with self.assertRaisesRegex(ValueError, 'exactly one'):
            release.release_notes(repo)

    def test_invalid_release_identity_is_rejected(self):
        for value in ('v2.0.0', '2.0', '02.0.0', '2.0.0-01', '2.0.0-rc..1'):
            with self.subTest(version=value), self.assertRaises(ValueError):
                release.release_notes(self.fixture(value))


if __name__ == '__main__':
    unittest.main()
