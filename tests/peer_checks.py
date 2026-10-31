#!/usr/bin/env python3
"""Actual foreign-UID IPC rejection; isolated container or disposable macOS CI only."""
import argparse
import base64
from contextlib import contextmanager
import json
import os
from pathlib import Path
import socket
import stat
import struct
import subprocess
import sys
import unittest

import agent_checks as a
from sandbox import native_server
from wire import Peer, packet, HELLO, REQUEST

BINARY = None
OBSERVATIONS = {}

def envelope(box, args):
    p = box.run([BINARY, '--json', *args], check=False, timeout=20)
    assert p.returncode == 0 and not p.stderr, (p.returncode, p.stdout, p.stderr)
    return json.loads(p.stdout)

@contextmanager
def traversable_for_peer_test(box, sock):
    # Prove the kernel peer-credential gate separately from ordinary filesystem denial.
    # Relax only this fixture's ancestors/socket temporarily; restore before
    # Sandbox.guard/teardown or any normal owner CLI operation.
    paths = []
    p = sock.parent
    while p != box.root.parent:
        paths.append(p)
        if p == box.root:
            break
        p = p.parent
    original = [(p, stat.S_IMODE(p.stat().st_mode)) for p in paths]
    mode = stat.S_IMODE(sock.stat().st_mode)
    try:
        for p, _ in original:
            p.chmod(0o711)
        sock.chmod(0o666)
        yield
    finally:
        sock.chmod(mode)
        for p, saved in original:
            p.chmod(saved)


def foreign_probe(path, payload):
    script = '''import base64,json,os,socket,sys
os.setgroups([]);os.setgid(65534);os.setuid(65534)
s=socket.socket(socket.AF_UNIX);s.settimeout(4)
result={"uid":os.geteuid(),"connected":False,"rejected":False}
try:
 s.connect(sys.argv[1]);result['connected']=True
 try:s.sendall(base64.b64decode(sys.argv[2]));data=s.recv(4096)
 except (BrokenPipeError,ConnectionResetError):data=b''
 result['rejected']=not data
except PermissionError:result['filesystem_denied']=True
finally:s.close()
print(json.dumps(result))
'''
    p = subprocess.run([sys.executable, '-c', script, str(path), base64.b64encode(payload).decode()],
                       cwd='/tmp', env={'PATH': '/usr/bin:/bin', 'LANG': 'C'},
                       capture_output=True, text=True, timeout=6)
    assert p.returncode == 0, p.stderr
    return json.loads(p.stdout)


class ForeignUIDChecks(unittest.TestCase):
    def test_native_filesystem_and_actual_foreign_peer_rejection(self):
        with a.fixture() as box, native_server(box, BINARY, name='foreign_guard') as (server, sock, _):
            data = packet(HELLO, 1, {'role': 'control'})+packet(REQUEST, 2, {'op': 'kill'})
            denied = foreign_probe(sock, data)
            self.assertEqual(denied['uid'], 65534);self.assertTrue(denied['filesystem_denied'])
            with traversable_for_peer_test(box, sock):
                peer = foreign_probe(sock, data)
                self.assertTrue(peer['connected']);self.assertTrue(peer['rejected'])
            with Peer(sock, 'control') as owner:
                self.assertEqual(owner.identity['name'], 'foreign_guard')
                self.assertEqual(owner.request('status')['process_state'], 'running')
            self.assertIsNone(server.poll())
            OBSERVATIONS['foreign_native'] = {'filesystem_denied': True, 'connected_uid': peer['uid'],
                                               'peer_rejected': True, 'owner_and_child_survived': True}

    def test_coordinator_foreign_peer_cannot_stop_or_mutate_registry(self):
        with a.fixture() as box, a.coordinator(box) as service:
            envelope(box, ['agent', 'list'])
            registry = a.state(box)/'registry.json';before = registry.read_bytes()
            metadata = json.loads((a.runtime(box)/'coordinator.json').read_text())
            path = a.runtime(box)/('s-'+metadata['epoch']+'.sock')
            host = json.loads((a.state(box).parent/'host.json').read_text())['host_id']
            body = json.dumps({'schema_version': 1, 'epoch': metadata['epoch'], 'host_id': host,
                               'state_root': str(a.state(box)), 'op': 'stop'}).encode()
            framed = struct.pack('!I', len(body))+body
            self.assertTrue(foreign_probe(path, framed)['filesystem_denied'])
            with traversable_for_peer_test(box, path):
                result = foreign_probe(path, framed)
                self.assertTrue(result['connected']);self.assertTrue(result['rejected'])
            self.assertIsNone(service.poll());self.assertEqual(registry.read_bytes(), before)
            self.assertEqual(envelope(box, ['agent', 'list'])['data']['agents'], [])
            OBSERVATIONS['foreign_coordinator'] = {'connected_uid': result['uid'], 'peer_rejected': True,
                                                    'service_survived': True, 'registry_unchanged': True}


def main():
    global BINARY
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', required=True, type=Path)
    parser.add_argument('--report', required=True, type=Path)
    args, remaining = parser.parse_known_args()
    container = sys.platform == 'linux' and Path('/.dockerenv').exists() and os.environ.get('TXM_LINUX_FIXTURE') == '1'
    macos_ci = sys.platform == 'darwin' and os.environ.get('GITHUB_ACTIONS') == 'true' and os.environ.get('TXM_ISOLATED_CI') == '1'
    if os.geteuid() != 0 or not (container or macos_ci):
        parser.error('requires an isolated Linux container or explicitly enabled disposable macOS CI; never host root')
    BINARY = args.binary.resolve(strict=True);a.BINARY = BINARY
    program = unittest.main(argv=[sys.argv[0], *remaining], exit=False)
    result = {'passed': program.result.wasSuccessful(), 'tests_run': program.result.testsRun,
              'skipped': program.result.skipped, 'platform': sys.platform, 'observations': OBSERVATIONS,
              'scope': 'Fixture-only UID 65534 filesystem and peer denial, with legitimate owner/children preserved; no account or service changes'}
    with args.report.open('x') as f:f.write(json.dumps(result, indent=2)+'\n')
    return 0 if result['passed'] else 1

if __name__ == '__main__':raise SystemExit(main())
