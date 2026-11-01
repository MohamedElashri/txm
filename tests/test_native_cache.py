"""Go must not reuse cached links after external native library bytes change."""
import importlib.util
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('native_cache', Path(__file__).resolve().parents[1]/'scripts/native-cache-key.py')
cache = importlib.util.module_from_spec(spec);spec.loader.exec_module(cache)


class NativeCacheTests(unittest.TestCase):
    def test_action_caches_each_native_abi_path_and_module_cache_explicitly(self):
        action = (Path(__file__).resolve().parents[1]/'.github/actions/setup-build/action.yml').read_text()
        self.assertIn('cache: false', action)
        self.assertIn('path: ~/go/pkg/mod', action)
        self.assertIn('path: ${{ github.workspace }}/.go-cache/${{ steps.go-cache.outputs.fingerprint }}', action)
        prefix = 'txm-go-v2-${{ runner.os }}-${{ runner.arch }}-1.26.10-${{ steps.go-cache.outputs.fingerprint }}-'
        self.assertIn('key: '+prefix, action)
        self.assertIn('restore-keys: '+prefix, action)

    def test_library_and_sdk_changes_invalidate_go_cache_but_metadata_does_not(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp);library = root/'.ghostty-host/lib/libghostty-vt.a';library.parent.mkdir(parents=True)
            library.write_bytes(b'verified first library')
            first = cache.fingerprint(root, 'host', None, 'sdk-1-deps-1')
            library.touch()
            self.assertEqual(first, cache.fingerprint(root, 'host', None, 'sdk-1-deps-1'))
            self.assertNotEqual(first, cache.fingerprint(root, 'host', None, 'sdk-2-deps-1'))
            library.write_bytes(b'new native library')
            self.assertNotEqual(first, cache.fingerprint(root, 'host', None, 'sdk-1-deps-1'))

    def test_missing_empty_or_symlinked_libraries_cannot_claim_a_cache_identity(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp);library = root/'.ghostty-host/lib/libghostty-vt.a';library.parent.mkdir(parents=True)
            with self.assertRaises(ValueError):cache.fingerprint(root, 'host', None, 'sdk')
            library.touch()
            with self.assertRaises(ValueError):cache.fingerprint(root, 'host', None, 'sdk')
            library.unlink();target = root/'target';target.write_bytes(b'library');library.symlink_to(target)
            with self.assertRaises(ValueError):cache.fingerprint(root, 'host', None, 'sdk')

    def test_partitions_do_not_require_or_hash_other_platform_libraries(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            for name in ['linux-amd64', 'linux-arm64', 'linux-musl-amd64', 'linux-musl-arm64']:
                library = root/'.ghostty-release'/name/'lib/libghostty-vt.a';library.parent.mkdir(parents=True);library.write_bytes(name.encode())
            linux = cache.fingerprint(root, 'release', 'linux', 'sdk')
            with self.assertRaises(ValueError):cache.fingerprint(root, 'release', 'darwin', 'sdk')
            for name in ['darwin-amd64', 'darwin-arm64']:
                library = root/'.ghostty-release'/name/'lib/libghostty-vt.a';library.parent.mkdir(parents=True);library.write_bytes(name.encode())
            self.assertEqual(linux, cache.fingerprint(root, 'release', 'linux', 'sdk'))
            self.assertNotEqual(linux, cache.fingerprint(root, 'release', 'darwin', 'sdk'))


if __name__ == '__main__':unittest.main()
