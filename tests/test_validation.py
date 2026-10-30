"""Guard the final validator and privacy scanner without using a live backend."""
import importlib.util
import io
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


class ValidationChecks(unittest.TestCase):
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
            (root/'VERSION').write_text('2.0.0-rc.1')
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
