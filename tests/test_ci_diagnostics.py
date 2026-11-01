"""Failure details must survive GitHub's per-annotation byte limit."""
import importlib.util
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location('ci_failure', Path(__file__).resolve().parents[1]/'.github/scripts/annotate-failure.py')
diagnostics = importlib.util.module_from_spec(spec)
spec.loader.exec_module(diagnostics)


class CIDiagnosticTests(unittest.TestCase):
    def test_failure_is_retained_across_long_success_output(self):
        log = ('PASS: unrelated\n' * 3000 + '    probe_test.go:42: counter unit mismatch\n'
               '\x1b[31m--- FAIL: TestCounterUnits (0.15s)\x1b[0m\n'
               + 'PASS: subsequent\n' * 3000 + 'FAIL\nmake: *** [test] Error 1\n')
        parts = diagnostics.messages(log)
        self.assertIn('probe_test.go:42: counter unit mismatch', '\n'.join(parts))
        self.assertIn('--- FAIL: TestCounterUnits', '\n'.join(parts))
        self.assertNotIn('\x1b', '\n'.join(parts))
        self.assertTrue(all(len(part.encode()) < 4096 for part in parts))

    def test_unicode_and_many_failures_stay_within_api_and_step_limits(self):
        log = '\n'.join('--- FAIL: Test'+str(i)+'\n'+('界'*4000) for i in range(20))
        parts = diagnostics.messages(log)
        self.assertLessEqual(len(parts), 8)
        self.assertIn('Test0', parts[0])
        self.assertTrue(all(len(part.encode()) < 4096 for part in parts))

    def test_unrecognized_failure_keeps_a_bounded_tail(self):
        parts = diagnostics.messages('noise\n'*10000+'unexpected process exit\n')
        self.assertIn('unexpected process exit', '\n'.join(parts))
        self.assertLess(sum(map(len, parts)), 500)


if __name__ == '__main__':
    unittest.main()
