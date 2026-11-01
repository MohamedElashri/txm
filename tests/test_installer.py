"""Offline asset-selection contract; never downloads or installs a real release."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

INSTALLER = Path(__file__).resolve().parents[1] / 'utils/install.sh'


@unittest.skipUnless(os.name == 'posix', 'Unix shell installer')
class InstallerAssetTests(unittest.TestCase):
    def selected(self, system, architecture, assets):
        with tempfile.TemporaryDirectory(prefix='txm-installer-') as temp:
            root = Path(temp)
            for name in ['bin', 'home', 'tmp']:
                (root/name).mkdir(mode=0o700)
            release = {'tag_name': 'v2.1.0', 'assets': [
                {'browser_download_url': 'https://example.invalid/'+name} for name in assets]}
            (root/'release.json').write_text(json.dumps(release, indent=2))
            uname = root/'bin/uname'
            uname.write_text(f'#!{sys.executable}\nimport sys\nprint({system!r} if sys.argv[1]=="-s" else {architecture!r})\n')
            curl = root/'bin/curl'
            curl.write_text(f'''#!{sys.executable}
import json,sys
from pathlib import Path
root=Path({str(root)!r})
args=sys.argv[1:]
if args==['-sf','https://api.github.com/repos/MohamedElashri/txm/releases/latest']:
 print((root/'release.json').read_text());sys.exit(0)
assert len(args)==4 and args[0]=='-Lf' and args[2]=='-o'
(root/'selected.json').write_text(json.dumps({{'url':args[1],'archive':args[3]}}))
sys.exit(9) # End before extraction/installation; no network is ever used.
''')
            unzip = root/'bin/unzip'
            unzip.write_text('#!/bin/sh\nexit 99\n')
            for executable in [uname, curl, unzip]:
                executable.chmod(0o700)
            env = {'PATH': str(root/'bin')+':/usr/bin:/bin', 'HOME': str(root/'home'),
                   'TMPDIR': str(root/'tmp'), 'LANG': 'C'}
            result = subprocess.run(['/bin/bash', str(INSTALLER)], env=env, cwd=root,
                                    capture_output=True, text=True, timeout=10)
            self.assertNotEqual(result.returncode, 0)
            selected = root/'selected.json'
            return json.loads(selected.read_text()) if selected.exists() else None

    def test_both_macs_prefer_the_universal_asset_over_legacy_or_similar_names(self):
        for architecture in ['x86_64', 'arm64']:
            with self.subTest(architecture=architecture):
                assets = ['txm_Darwin_'+architecture+'.zip', 'txm_Darwin_universal.zip.sig',
                          'txm_Darwin_universal.zip', 'other_txm_Darwin_universal.zip']
                self.assertEqual(self.selected('Darwin', architecture, assets), {
                    'url': 'https://example.invalid/txm_Darwin_universal.zip',
                    'archive': 'txm_Darwin_universal.zip'})

    def test_existing_stable_mac_releases_keep_their_architecture_fallback(self):
        for architecture in ['x86_64', 'arm64']:
            with self.subTest(architecture=architecture):
                name = 'txm_Darwin_'+architecture+'.zip'
                self.assertEqual(self.selected('Darwin', architecture, [name])['archive'], name)

    def test_linux_keeps_separate_native_archives(self):
        assets = ['txm_Linux_x86_64.zip', 'txm_Linux_arm64.zip', 'txm_Darwin_universal.zip']
        for architecture, name in [('x86_64', assets[0]), ('aarch64', assets[1])]:
            with self.subTest(architecture=architecture):
                self.assertEqual(self.selected('Linux', architecture, assets)['archive'], name)

    def test_missing_or_unsupported_targets_never_start_a_download(self):
        self.assertIsNone(self.selected('Darwin', 'arm64', ['txm_Linux_arm64.zip']))
        self.assertIsNone(self.selected('Darwin', 'ppc64', ['txm_Darwin_universal.zip']))


if __name__ == '__main__':
    unittest.main()
