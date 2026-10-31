"""Reject incomplete, swapped, stale or tampered bundles before draft upload."""
import hashlib
import importlib.util
from pathlib import Path
import struct
import shutil
import tempfile
import unittest
import zipfile

REPO = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('release_artifacts', REPO/'scripts/check-release-artifacts.py')
artifacts = importlib.util.module_from_spec(spec)
spec.loader.exec_module(artifacts)


def executable(system, arch, dynamic=False):
    data = bytearray(128)
    if system == 'Linux':
        data[:6] = b'\x7fELF\x02\x01';struct.pack_into('<H', data, 18, 62 if arch == 'x86_64' else 183)
        struct.pack_into('<Q', data, 32, 64);struct.pack_into('<HH', data, 54, 56, 1)
        struct.pack_into('<I', data, 64, 3 if dynamic else 1)
    elif system == 'Darwin':
        data[:4] = b'\xcf\xfa\xed\xfe';struct.pack_into('<I', data, 4, 0x1000007 if arch == 'x86_64' else 0x100000c)
    else:
        data[:2] = b'MZ';struct.pack_into('<I', data, 60, 64);data[64:68] = b'PE\0\0'
        struct.pack_into('<H', data, 68, 0x8664 if arch == 'x86_64' else 0xaa64)
    return data


class ReleaseArtifactTests(unittest.TestCase):
    def fixture(self):
        temp = tempfile.TemporaryDirectory(prefix='txm-bundle-test-');self.addCleanup(temp.cleanup)
        root = Path(temp.name);bundle=root/'dist';bundle.mkdir();repo=root/'repo';(repo/'docs').mkdir(parents=True)
        for name in ['README.md','LICENSE','docs/txm.1']:(repo/name).write_text('fixture '+name)
        for name,(system,arch,static) in artifacts.TARGETS.items():
            with zipfile.ZipFile(bundle/name,'w') as z:
                for member in artifacts.COMMON:z.writestr(member,(repo/member).read_bytes() if (repo/member).exists() else b'completion')
                z.writestr('txm.exe' if system=='Windows' else 'txm',executable(system,arch))
        self.checksums(bundle);return bundle,repo

    def checksums(self,bundle):
        (bundle/'checksums.txt').write_text(''.join(hashlib.sha256(p.read_bytes()).hexdigest()+'  '+p.name+'\n' for p in sorted(bundle.glob('*.zip'))))

    def test_complete_bundle_and_binary_targets(self):
        bundle,repo=self.fixture();self.assertEqual(len(artifacts.verify(bundle,repo)['archives']),8)

    def test_missing_extra_duplicate_and_tampered_artifacts(self):
        for kind in ['missing','extra','duplicate','tampered']:
            with self.subTest(kind=kind):
                bundle,repo=self.fixture();zipfile_path=next(bundle.glob('*.zip'))
                if kind=='missing':zipfile_path.unlink()
                elif kind=='extra':(bundle/'unexpected.zip').write_bytes(b'extra')
                elif kind=='duplicate':
                    p=bundle/'checksums.txt';p.write_text(p.read_text()+p.read_text().splitlines()[0]+'\n')
                else:zipfile_path.write_bytes(b'tampered')
                with self.assertRaises(ValueError):artifacts.verify(bundle,repo)

    def test_source_mismatch_wrong_arch_and_dynamic_musl(self):
        for kind in ['source','arch','musl']:
            with self.subTest(kind=kind):
                bundle,repo=self.fixture()
                if kind=='source':(repo/'README.md').write_text('stale')
                else:
                    name='txm_Linux_x86_64-musl.zip' if kind=='musl' else 'txm_Linux_x86_64.zip'
                    path=bundle/name
                    with zipfile.ZipFile(path) as z:members={n:z.read(n) for n in z.namelist()}
                    members['txm']=executable('Linux','arm64' if kind=='arch' else 'x86_64',dynamic=kind=='musl')
                    with zipfile.ZipFile(path,'w') as z:
                        for n,data in members.items():z.writestr(n,data)
                    self.checksums(bundle)
                with self.assertRaises(ValueError):artifacts.verify(bundle,repo)

    def partitions(self):
        parts, repo = self.fixture()
        lines = (parts/'checksums.txt').read_text().splitlines()
        (parts/'checksums.txt').unlink()
        for platform in ('linux', 'darwin'):
            selected = [line for line in lines if ('Darwin' in line) == (platform == 'darwin')]
            (parts/f'checksums-{platform}.txt').write_text('\n'.join(selected)+'\n')
        return parts, repo

    def test_build_partitions_reassemble_without_changing_checksums(self):
        parts, repo = self.partitions();out = parts.parent/'bundle'
        self.assertEqual(len(artifacts.assemble(parts, out, repo)['archives']), 8)
        self.assertEqual((out/'checksums.txt').read_text().splitlines(), sorted(
            (parts/'checksums-linux.txt').read_text().splitlines()+(parts/'checksums-darwin.txt').read_text().splitlines()))
        with self.assertRaises(FileExistsError):artifacts.assemble(parts, out, repo)

    def test_each_partition_is_complete_but_cannot_substitute_for_full_bundle(self):
        for platform, count in [('linux', 6), ('darwin', 2)]:
            with self.subTest(platform=platform):
                parts, repo = self.partitions();part = parts.parent/'part';part.mkdir()
                shutil.copyfile(parts/f'checksums-{platform}.txt',part/f'checksums-{platform}.txt')
                for name in artifacts.TARGETS:
                    if ('Darwin' in name) == (platform == 'darwin'):shutil.copyfile(parts/name,part/name)
                self.assertEqual(len(artifacts.verify(part,repo,platform=platform)['archives']),count)
                with self.assertRaises(ValueError):artifacts.verify(part,repo)
                next(part.glob('*.zip')).unlink()
                with self.assertRaises(ValueError):artifacts.verify(part,repo,platform=platform)

    def test_missing_duplicate_wrong_partition_and_tampered_builds_are_rejected(self):
        for kind in ('missing', 'duplicate', 'wrong-partition', 'tampered'):
            with self.subTest(kind=kind):
                parts, repo = self.partitions();manifest = parts/'checksums-linux.txt'
                if kind == 'missing':next(parts.glob('*.zip')).unlink()
                elif kind == 'duplicate':manifest.write_text(manifest.read_text()+manifest.read_text().splitlines()[0]+'\n')
                elif kind == 'wrong-partition':manifest.write_text((parts/'checksums-darwin.txt').read_text())
                else:next(parts.glob('*.zip')).write_bytes(b'tampered')
                with self.assertRaises(ValueError):artifacts.assemble(parts, parts.parent/'bundle', repo)


if __name__=='__main__':unittest.main()
