"""Build guard behavior with a disposable Git repo and fake compiler, no network."""
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest


class BuildGuardTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix="txm-build-test-")
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        scripts = self.root / "scripts"
        scripts.mkdir()
        shutil.copyfile(Path(__file__).resolve().parents[1] / "scripts/build-deps.sh", scripts / "build-deps.sh")
        home = self.root / "home"
        home.mkdir(mode=0o700)
        self.env = {"HOME": str(home), "XDG_CONFIG_HOME": str(home/"config"),
                    "PATH": os.defpath, "LANG": "C", "GIT_CONFIG_NOSYSTEM": "1"}
        self.source = self.root / "source"
        subprocess.run(["git", "-c", "init.templateDir=", "init", "-q", str(self.source)], check=True, env=self.env)
        subprocess.run(["git", "-C", str(self.source), "-c", "user.name=fixture", "-c",
                        "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false",
                        "-c", "core.hooksPath=/dev/null",
                        "commit", "--allow-empty", "-qm", "fixture"], check=True, env=self.env)
        self.head = subprocess.check_output(["git", "-C", str(self.source), "rev-parse", "HEAD"], text=True, env=self.env).strip()
        self.lock = scripts / "dependencies.env"
        self.lock.write_text(f"GHOSTTY_REVISION={self.head}\nZIG_VERSION=0.16.0\n")
        self.zig = self.root / "zig"
        self.zig.write_text(f'''#!{sys.executable}
import os, sys
from pathlib import Path
if sys.argv[1] == 'version':
    print(os.environ.get('FAKE_ZIG_VERSION', '0.16.0'))
else:
    assert '-Demit-xcframework=false' in sys.argv, 'txm must not require Xcode packaging'
    with open({str(self.root / 'builds')!r}, 'a') as f: f.write('build\\n')
    if os.environ.get('FAKE_ZIG_FAIL'): sys.exit(1)
    prefix=Path(sys.argv[sys.argv.index('--prefix')+1])
    for relative in ['share/pkgconfig/libghostty-vt-static.pc', 'lib/libghostty-vt.a']:
        p=prefix/relative
        p.parent.mkdir(parents=True,exist_ok=True)
        p.write_text('fixture')
''')
        self.zig.chmod(0o700)
        self.prefix = self.root / "prefix"
        self.env = {**self.env, "ZIG": str(self.zig), "GHOSTTY_SOURCE_DIR": str(self.source),
                    "GHOSTTY_HOST_DIR": str(self.prefix), "GHOSTTY_BUILD_JOBS": "2"}

    def run_build(self):
        return subprocess.run(["bash", str(self.root / "scripts/build-deps.sh"), "host"],
                              env=self.env, capture_output=True, text=True, timeout=10)

    def test_mismatched_version_rejected_before_build(self):
        self.env["FAKE_ZIG_VERSION"] = "0.17.0"
        self.assertNotEqual(self.run_build().returncode, 0)
        self.assertFalse(self.prefix.exists())

    def test_mismatched_and_dirty_source_left_unchanged(self):
        self.lock.write_text("GHOSTTY_REVISION=" + "0" * 40 + "\nZIG_VERSION=0.16.0\n")
        self.assertNotEqual(self.run_build().returncode, 0)
        self.lock.write_text(f"GHOSTTY_REVISION={self.head}\nZIG_VERSION=0.16.0\n")
        untracked = self.source / "unexpected"
        untracked.write_text("do not delete")
        self.assertNotEqual(self.run_build().returncode, 0)
        self.assertEqual(untracked.read_text(), "do not delete")
        self.assertEqual(subprocess.check_output(["git", "-C", str(self.source), "rev-parse", "HEAD"], text=True, env=self.env).strip(), self.head)
        self.assertFalse((self.root / "builds").exists())

    def test_existing_directory_is_not_a_cache_hit(self):
        self.prefix.mkdir()
        for _ in range(2):
            self.assertEqual(self.run_build().returncode, 0)
        (self.prefix / "lib/libghostty-vt.a").unlink()
        self.assertEqual(self.run_build().returncode, 0)
        self.assertEqual((self.root / "builds").read_text().splitlines(), ["build"] * 3)
        self.assertTrue((self.prefix / "lib/libghostty-vt.a").exists())

    def test_failed_build_does_not_stamp_success(self):
        self.env["FAKE_ZIG_FAIL"] = "1"
        self.assertNotEqual(self.run_build().returncode, 0)
        self.assertFalse((self.prefix / ".txm-build-info").exists())

    def test_release_requires_absolute_root_and_builds_exact_target_prefixes(self):
        command=["bash",str(self.root/"scripts/build-deps.sh"),"release"]
        self.env.pop("TXM_RELEASE_ROOT",None)
        self.assertNotEqual(subprocess.run(command,env=self.env,capture_output=True,timeout=10).returncode,0)
        self.assertFalse((self.root/".ghostty-release").exists())
        self.env["TXM_RELEASE_ROOT"]=str(self.root)
        out=subprocess.run(command,env=self.env,capture_output=True,text=True,timeout=10)
        self.assertEqual(out.returncode,0,out.stderr)
        prefixes=list((self.root/".ghostty-release").iterdir())
        self.assertEqual({p.name for p in prefixes}, {
            'linux-amd64', 'linux-arm64', 'linux-musl-amd64',
            'linux-musl-arm64', 'darwin-amd64', 'darwin-arm64'})
        for prefix in prefixes:self.assertTrue((prefix/".txm-build-info").exists())


class BuildGuardEnvironmentTests(unittest.TestCase):
    def test_inherited_git_routing_and_credentials_cannot_escape_fixture(self):
        from unittest.mock import patch
        with tempfile.TemporaryDirectory(prefix='txm-build-routing-') as temp:
            protected = Path(temp)/'protected'
            protected.write_text('fixture must not modify this')
            with patch.dict(os.environ, {'GIT_DIR': str(protected), 'GIT_WORK_TREE': temp,
                                         'SYNTHETIC_CANARY': 'PRIVATE_FIXTURE_ENV'}):
                fixture = BuildGuardTests('test_existing_directory_is_not_a_cache_hit')
                try:
                    fixture.setUp()
                    self.assertNotIn('GIT_DIR', fixture.env)
                    self.assertNotIn('GIT_WORK_TREE', fixture.env)
                    self.assertNotIn('SYNTHETIC_CANARY', fixture.env)
                    self.assertEqual(fixture.run_build().returncode, 0)
                    self.assertEqual(protected.read_text(), 'fixture must not modify this')
                finally:
                    fixture.doCleanups()
