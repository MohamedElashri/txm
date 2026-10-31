#!/usr/bin/env python3
"""Opt-in metrics over owned managed children; disabled mode stays artifact-free."""
import argparse
from pathlib import Path
import sys
import unittest
import agent_checks as a
import extension_checks as x
from mcp_checks import snapshot
from sandbox import wait_for

BINARY = None
class MetricsChecks(unittest.TestCase):
    def test_disabled_metrics_and_explicit_read_create_no_metrics_artifacts(self):
        with a.fixture() as box:
            with a.coordinator(box):
                job = a.start(box, 'no-metrics', ['/usr/bin/true']); a.await_exit(box, job['id'])
            before = snapshot(box)
            view = x.command(box, ['agent', 'metrics', job['id']])['data']
            self.assertIsNone(view['latest']); self.assertEqual(view['freshness'], 'unavailable')
            self.assertFalse((box.root / 'state/txm/v1/metrics').exists())
            self.assertEqual(snapshot(box), before)

    def test_child_samples_survive_coordinator_restart_and_read_is_pure(self):
        with a.fixture() as box:
            with a.coordinator(box):
                job = a.start(box, 'sampled', [sys.executable, '-c',
                    'import time\nend=time.monotonic()+4.5\nwhile time.monotonic()<end: pass'],
                    extra=['--metrics', '--metrics-interval', '2'])
            seen = []
            def sampled():
                value = x.command(box, ['agent', 'metrics', job['id']])['data']
                if value['samples'] >= 2:
                    seen.append(value); return True
                return False
            wait_for(sampled, timeout=8)
            latest = seen[-1]['latest']
            self.assertEqual(latest['scope'], 'child_pid'); self.assertEqual(latest['state'], 'available')
            self.assertGreater(latest['rss_bytes'], 0); self.assertGreater(latest['cpu_percent'], 0)
            self.assertTrue(latest['start_identity'])
            receipt = a.state(box) / ('r-' + job['instance_id'] + '.json')
            wait_for(receipt.exists, timeout=8)
            # Receipt publication precedes final supervisor/native socket cleanup.
            # Observe that independently completing fixture teardown before the
            # read-only snapshot, rather than attributing it to the read.
            wait_for(lambda: not list((box.root / 'runtime/txm/v1').glob('s-*.sock')), timeout=8)
            wait_for(lambda: not list(a.runtime(box).glob('s-*.sock')), timeout=8)
            self.assertFalse((a.runtime(box) / 'coordinator.json').exists())
            before = snapshot(box)
            value = x.command(box, ['agent', 'metrics', job['id']])['data']
            self.assertGreaterEqual(value['samples'], 2)
            self.assertEqual(snapshot(box), before)
            files = list((box.root / 'state/txm/v1/metrics').glob('samples-*.json'))
            self.assertTrue(all(p.stat().st_size <= 1 << 20 for p in files))
            self.assertLessEqual(sum(p.stat().st_size for p in files), 30 << 20)

if __name__ == '__main__':
    parser = argparse.ArgumentParser(); parser.add_argument('--binary', required=True, type=Path)
    args, rest = parser.parse_known_args(); BINARY = args.binary.resolve(strict=True)
    a.BINARY = BINARY; x.BINARY = BINARY
    unittest.main(argv=[__file__, *rest])
