"""Exercise CI installer host branches without network or user configuration."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest

REPO = Path(__file__).resolve().parents[1]


class ZigInstallerTests(unittest.TestCase):
    def fixture(self, system, architecture, mismatch=False):
        temp = tempfile.TemporaryDirectory(prefix='txm-ci-zig-');self.addCleanup(temp.cleanup)
        root = Path(temp.name);(root/'scripts').mkdir();tools=root/'bin';tools.mkdir();(root/'home').mkdir();(root/'runner').mkdir()
        shutil.copyfile(REPO/'scripts/install-ci-zig.sh',root/'scripts/install-ci-zig.sh')
        (root/'scripts/dependencies.env').write_text('ZIG_VERSION=0.16.0\n')
        archive=b'private fixture archive';digest=hashlib.sha256(archive).hexdigest()
        platform='x86_64-linux' if system=='Linux' else ('aarch64-macos' if architecture=='arm64' else 'x86_64-macos')
        index={'0.16.0':{platform:{'tarball':'https://fixture.invalid/zig.tar.xz','shasum':'0'*64 if mismatch else digest}}}
        def executable(name,code):
            p=tools/name;p.write_text('#!'+sys.executable+'\n'+code);p.chmod(0o700)
        executable('uname',f'import sys\nprint({system!r} if sys.argv[1]=="-s" else {architecture!r})\n')
        executable('curl',f'''import sys
from pathlib import Path
out=Path(sys.argv[sys.argv.index('-o')+1])
out.write_bytes({json.dumps(index).encode()!r} if 'https://ziglang.org/download/index.json' in sys.argv else {archive!r})
''')
        executable('jq','''import json,sys
args=sys.argv[1:];values={args[i+1]:args[i+2] for i in range(len(args)) if args[i]=='--arg'}
data=json.load(open(args[-1]))[values['v']][values['p']]
print(data['tarball' if args[-2].endswith('.tarball') else 'shasum'])
''')
        (tools/'python3').symlink_to(sys.executable)
        # Neither GNU nor hosted macOS checksum utility behavior is required.
        for name in ['sha256sum','shasum']:
            executable(name, "raise SystemExit('external checksum utility must not be invoked')\n")
        executable('tar',f'''import sys
from pathlib import Path
out=Path(sys.argv[sys.argv.index('-C')+1]);(out/{('zig-'+platform+'-0.16.0')!r}).mkdir()
(out/'extracted').write_text('verified')
''')
        env={'HOME':str(root/'home'),'PATH':str(tools)+os.pathsep+os.defpath,'LANG':'C','RUNNER_TEMP':str(root/'runner'),'GITHUB_PATH':str(root/'github-path')}
        return root,env,platform

    def test_verified_download_supports_all_ci_hosts_without_checksum_utilities(self):
        for system,arch in [('Linux','x86_64'),('Darwin','arm64'),('Darwin','x86_64')]:
            with self.subTest(system=system,arch=arch):
                root,env,platform=self.fixture(system,arch)
                p=subprocess.run(['/bin/bash',str(root/'scripts/install-ci-zig.sh')],env=env,cwd=root,capture_output=True,text=True,timeout=10)
                self.assertEqual(p.returncode,0,p.stderr)
                installed=Path((root/'github-path').read_text().strip())
                self.assertEqual(installed.name,'zig-'+platform+'-0.16.0');self.assertTrue(installed.is_dir())
                self.assertEqual(installed.parent.parent,root/'runner')

    def test_checksum_failure_cannot_extract_or_publish_path(self):
        root,env,_=self.fixture('Darwin','arm64',mismatch=True)
        p=subprocess.run(['/bin/bash',str(root/'scripts/install-ci-zig.sh')],env=env,cwd=root,capture_output=True,text=True,timeout=10)
        self.assertNotEqual(p.returncode,0)
        self.assertFalse((root/'github-path').exists());self.assertFalse(list((root/'runner').rglob('extracted')))


if __name__=='__main__':unittest.main()
