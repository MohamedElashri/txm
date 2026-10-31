"""Deterministically exercise the readiness races seen in hosted CI."""
import concurrent.futures
import json
from pathlib import Path
import tempfile
import threading
import unittest
from unittest.mock import Mock, patch

import native_checks
from sandbox import publish_json
from wire import Peer


class NativeFixtureTests(unittest.TestCase):
    def test_identity_is_not_visible_until_serialization_finishes(self):
        for replacing in (False, True):
            with self.subTest(replacing=replacing), tempfile.TemporaryDirectory() as tmp:
                receipt = Path(tmp)/'identity.json'
                if replacing:
                    receipt.write_text('[1, 1]')
                writing, finish = threading.Event(), threading.Event()
                def delayed_dump(value, stream):
                    stream.write('[');stream.flush();writing.set()
                    if not finish.wait(3):
                        raise TimeoutError('fixture test did not release writer')
                    stream.write('2, 2]')
                with patch('sandbox.json.dump', side_effect=delayed_dump), concurrent.futures.ThreadPoolExecutor() as pool:
                    future = pool.submit(publish_json, receipt, [2, 2])
                    try:
                        self.assertTrue(writing.wait(3))
                        if replacing:
                            self.assertEqual(json.loads(receipt.read_text()), [1, 1])
                        else:
                            self.assertFalse(receipt.exists())
                    finally:
                        finish.set()
                    future.result(timeout=3)
                self.assertEqual(json.loads(receipt.read_text()), [2, 2])

    def test_slot_probe_recovers_only_from_transient_pool_refusals(self):
        with patch('native_checks.PeerStatus', side_effect=[ConnectionResetError(), BrokenPipeError(), EOFError(), 1, 0]) as probe:
            native_checks.wait_for_native_slots('private-fixture-socket')
            self.assertEqual(probe.call_count, 5)

    def test_pool_that_never_recovers_still_times_out(self):
        with patch('native_checks.PeerStatus', side_effect=ConnectionResetError()):
            with self.assertRaises(TimeoutError):
                native_checks.wait_for_native_slots('private-fixture-socket', timeout=.03)

    def test_dead_servers_and_protocol_errors_are_not_retried(self):
        for failure in (FileNotFoundError(), ConnectionRefusedError(), AssertionError('bad frame'), RuntimeError('bad response')):
            with self.subTest(failure=type(failure).__name__), patch('native_checks.PeerStatus', side_effect=failure) as probe:
                with self.assertRaises(type(failure)):
                    native_checks.wait_for_native_slots('private-fixture-socket')
                probe.assert_called_once()

    def test_rejected_handshake_closes_its_client_socket(self):
        sock = Mock();sock.recv.return_value = b''
        with patch('wire.socket.socket', return_value=sock):
            with self.assertRaises(EOFError):
                Peer('private-fixture-socket')
        sock.close.assert_called_once()


if __name__ == '__main__':
    unittest.main()
