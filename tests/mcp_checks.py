#!/usr/bin/env python3
"""Independent newline JSON-RPC clients and owned local-state MCP fixtures."""
import argparse
from contextlib import contextmanager
import hashlib
import json
from pathlib import Path
import select
import subprocess
import unittest

import agent_checks as a
import extension_checks as x
from sandbox import Sandbox, wait_for

BINARY = None
MODERN = '2026-07-28'
LEGACY = '2025-11-25'
READS = {'list_sessions', 'get_session', 'list_agents', 'get_agent',
         'wait_agent', 'list_tasks', 'get_task', 'get_capabilities'}


def snapshot(box):
    result = {}
    for p in box.root.rglob('*'):
        info = p.lstat()
        if p.is_symlink():
            continue
        digest = hashlib.sha256(p.read_bytes()).hexdigest() if p.is_file() else ''
        result[str(p.relative_to(box.root))] = (info.st_mode, info.st_mtime_ns, digest)
    return result


def await_runtime_cleanup(box):
    # The caller already checked receipts. Observe independently completing
    # socket teardown before taking a no-side-effect read snapshot.
    wait_for(lambda: not list((box.root / 'runtime/txm/v1').glob('s-*.sock')), timeout=8)
    wait_for(lambda: not list(a.runtime(box).glob('s-*.sock')), timeout=8)


class Wire:
    def __init__(self, box, flags=(), modern=True):
        self.proc = box.start([BINARY, 'mcp', 'serve', *flags], stdin=subprocess.PIPE,
                              stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        self.modern = modern
        self.ident = 0
        self.discard_reply = None

    def send(self, method, params=None, notification=False, metadata=True):
        params = dict(params or {})
        if self.modern and metadata:
            params['_meta'] = {'io.modelcontextprotocol/protocolVersion': MODERN,
                               'io.modelcontextprotocol/clientCapabilities': {}}
        body = {'jsonrpc': '2.0', 'method': method, 'params': params}
        if not notification:
            self.ident += 1
            body['id'] = self.ident
        self.proc.stdin.write((json.dumps(body, separators=(',', ':')) + '\n').encode())
        self.proc.stdin.flush()
        return body.get('id')

    def receive(self, timeout=5):
        ready, _, _ = select.select([self.proc.stdout], [], [], timeout)
        if not ready:
            raise AssertionError('MCP response deadline exceeded')
        line = self.proc.stdout.readline()
        if not line:
            raise AssertionError('MCP closed stdout unexpectedly: ' + str(self.proc.poll()))
        value = json.loads(line)
        assert value.get('jsonrpc') == '2.0', value
        assert 'method' not in value, 'unexpected server request/notification'
        return value

    def call(self, method, params=None, metadata=True):
        ident = self.send(method, params, metadata=metadata)
        reply = self.receive()
        assert reply.get('id') == ident, reply
        return reply

    def initialize(self):
        if self.modern:
            reply = self.call('server/discover')
            assert set(reply['result']['supportedVersions']) == {MODERN, LEGACY}, reply
        else:
            reply = self.call('initialize', {'protocolVersion': LEGACY, 'capabilities': {},
                                           'clientInfo': {'name': 'independent-txm-fixture', 'version': '1'}})
            assert reply['result']['protocolVersion'] == LEGACY, reply
            self.send('notifications/initialized', notification=True)

    def tool(self, name, arguments=None):
        return self.call('tools/call', {'name': name, 'arguments': arguments or {}})

    def close(self, check=True):
        if self.proc.stdin:
            try:
                self.proc.stdin.close()
            except BrokenPipeError:
                pass
            self.proc.stdin = None
        output, error = self.proc.communicate(timeout=5)
        if check:
            assert self.proc.returncode == 0, (self.proc.returncode, error)
            assert not error, error
            if self.discard_reply is None:
                assert not output, output
            elif output:
                assert json.loads(output)['id'] == self.discard_reply
        return output, error


@contextmanager
def wire(box, flags=(), modern=True):
    client = Wire(box, flags, modern)
    try:
        yield client
    finally:
        client.close()


def output(reply):
    assert 'error' not in reply, reply
    assert not reply['result'].get('isError'), reply
    return reply['result']['structuredContent']


class MCPChecks(unittest.TestCase):
    def test_legacy_missing_fields_and_unsupported_proposal_negotiation(self):
        with Sandbox() as box, wire(box, modern=False) as client:
            invalid = client.call('initialize', {})
            self.assertEqual(invalid['error']['code'], -32602)
            reply = client.call('initialize', {'protocolVersion': '2099-01-01',
                'capabilities': {}, 'clientInfo': {'name': 'fixture', 'version': '1'}})
            self.assertEqual(reply['result']['protocolVersion'], LEGACY)
            client.send('notifications/initialized', notification=True)
            self.assertEqual({t['name'] for t in client.call('tools/list')['result']['tools']}, READS)

    def test_wait_capacity_and_modern_cancellation_leave_job_running(self):
        with a.fixture() as box, a.coordinator(box):
            job = a.start(box, 'cancellable', ['/bin/sleep', '3'])
            with wire(box, ['--read-id', job['id'], '--read-backend', 'native']) as client:
                client.initialize()
                waits = [client.send('tools/call', {'name': 'wait_agent', 'arguments': {
                    'id': job['id'], 'until': 'exited', 'timeout_ms': 60000}}) for _ in range(5)]
                refused = client.receive()
                self.assertIn(refused['id'], waits)
                self.assertEqual(refused['result']['structuredContent']['error']['code'], 'conflict')
                for ident in waits:
                    client.send('notifications/cancelled', {'requestId': ident}, notification=True)
                ping = client.send('ping')
                reply = client.receive()
                self.assertEqual(reply['id'], ping, 'cancelled modern wait emitted a response')
                current = output(client.tool('get_agent', {'id': job['id']}))['data']
                self.assertEqual(current['lifecycle'], 'running')
            a.await_exit(box, job['id'])

    def test_template_creates_replay_across_server_and_coordinator_restart(self):
        with a.fixture() as box:
            repo = x.repo(box)
            base = box.run(['git', '-C', repo, 'rev-parse', 'HEAD']).stdout.strip()
            grant = box.root / 'create.json'
            policy = {'schema_version': 1, 'namespace': '1' * 32, 'templates': [
                {'id': 'worker', 'kind': 'agent', 'name': 'mcp-worker', 'backend': 'native',
                 'directory': str(repo), 'argv': ['/usr/bin/true']},
                {'id': 'lost', 'kind': 'agent', 'name': 'mcp-lost-response', 'backend': 'native',
                 'directory': str(repo), 'argv': ['/usr/bin/true']},
                *[{'id': 'task' + str(i), 'kind': 'task', 'name': 'mcp-task' + str(i),
                   'backend': 'native', 'directory': str(repo), 'base_commit': base,
                   'argv': ['/usr/bin/true']} for i in range(3)]]}
            grant.write_text(json.dumps(policy)); grant.chmod(0o600)
            flags = ['--create-policy', str(grant), '--read-repo', str(repo), '--read-backend', 'native']
            created = []
            with a.coordinator(box):
                with wire(box, flags) as client:
                    client.initialize()
                    catalog = {t['name'] for t in client.call('tools/list')['result']['tools']}
                    self.assertEqual(catalog, READS | {'start_agent', 'start_task'})
                    for i, (tool, template) in enumerate([('start_agent', 'worker'), *[('start_task', 'task'+str(j)) for j in range(3)]], 2):
                        args = {'operation_id': str(i) * 32, 'template_id': template}
                        result = output(client.tool(tool, args))['data']
                        self.assertEqual(result['state'], 'accepted', result)
                        self.assertEqual(output(client.tool(tool, args))['data'], result)
                        created.append((tool, args, result))
                    bad = client.tool('start_agent', {'operation_id': '2'*32, 'template_id': 'task0'})
                    self.assertTrue(bad['result']['isError'])
                    forged = client.tool('start_agent', {'operation_id': '9'*32, 'template_id': 'worker', 'argv': ['/bin/sh']})
                    self.assertTrue('error' in forged or forged['result'].get('isError'))
                    # Commit the create while deliberately dropping its response.
                    lost = {'operation_id': '9'*32, 'template_id': 'lost'}
                    client.discard_reply = client.send('tools/call', {'name': 'start_agent', 'arguments': lost})
                    journal = box.root / ('state/txm/v1/mcp-' + '1'*32) / 'operations.json'
                    committed = []
                    def accepted():
                        entries = json.loads(journal.read_text())['entries']
                        for entry in entries:
                            if entry['operation_id'] == lost['operation_id'] and entry['state'] == 'accepted':
                                committed.append(entry); return True
                        return False
                    wait_for(accepted)
                    created.append(('start_agent', lost, committed[-1]))
                for _, _, result in created:
                    a.await_exit(box, result['resource_id'])
            # Restarted server does not need or start the coordinator to replay.
            await_runtime_cleanup(box)
            before = snapshot(box)
            with wire(box, flags) as client:
                client.initialize()
                for tool, args, result in created:
                    self.assertEqual(output(client.tool(tool, args))['data'], result)
                tasks = output(client.tool('list_tasks'))['data']['items']
                self.assertEqual(len(tasks), 3)
                self.assertTrue(all(t['receipt']['exit_code'] == 0 for t in tasks))
            for task in tasks:
                reviewed = x.command(box, ['task', 'review', task['id'], '--target', 'refs/heads/main'])['data']
                self.assertEqual(reviewed['compare_from'], task['base_commit'])
                handoff = x.command(box, ['task', 'merge', task['id'], '--target', 'refs/heads/main'])['data']
                self.assertEqual(handoff['state'], 'dry_run')
                self.assertEqual(handoff['review']['id'], task['id'])
            # A writer lock may be opened but no file bytes, modes or mtimes change.
            self.assertEqual(snapshot(box), before)
            self.assertFalse((a.runtime(box) / 'coordinator.json').exists())

    def test_both_eras_default_deny_and_no_state(self):
        for modern in (False, True):
            with self.subTest(modern=modern), Sandbox() as box:
                await_runtime_cleanup(box)
                before = snapshot(box)
                with wire(box, modern=modern) as client:
                    client.initialize()
                    catalog = client.call('tools/list')['result']['tools']
                    self.assertEqual({t['name'] for t in catalog}, READS)
                    self.assertTrue(all(t['annotations']['readOnlyHint'] for t in catalog))
                    for name in ('list_agents', 'list_tasks', 'list_sessions'):
                        value = output(client.tool(name))
                        self.assertEqual(value['data']['items'], [])
                        self.assertFalse(value['partial'])
                    caps = output(client.tool('get_capabilities'))['data']
                    self.assertFalse(caps['destructive_tools']); self.assertFalse(caps['process_create'])
                    self.assertFalse(caps['remote_reads']); self.assertFalse(caps['output_preview'])
                    for name in ('get_agent', 'get_session', 'get_task', 'wait_agent'):
                        result = client.tool(name, {'id': 'a' * 32})['result']
                        self.assertTrue(result['isError'])
                        self.assertEqual(result['structuredContent']['error']['code'], 'denied')
                    for name in ('start_agent', 'start_task', 'stop_agent', 'cleanup_task', 'merge_task'):
                        self.assertIn('error', client.tool(name))
                self.assertEqual(snapshot(box), before)

    def test_existing_receipts_read_without_coordinator_or_rewrites(self):
        with a.fixture() as box:
            with a.coordinator(box):
                allowed = a.start(box, 'allowed', ['/usr/bin/true'])
                other = a.start(box, 'excluded', ['/usr/bin/true'])
                a.await_exit(box, allowed['id']); a.await_exit(box, other['id'])
            await_runtime_cleanup(box)
            before = snapshot(box)
            flags = ['--read-id', allowed['id'], '--read-backend', 'native']
            with wire(box, flags) as client:
                client.initialize()
                rows = output(client.tool('list_agents'))['data']['items']
                self.assertEqual([r['id'] for r in rows], [allowed['id']])
                self.assertEqual(rows[0]['receipt']['exit_code'], 0)
                sessions = output(client.tool('list_sessions'))['data']['items']
                self.assertEqual(sessions[0]['session_status'], 'unverified')
                self.assertEqual(sessions[0]['agent_id'], allowed['id'])
                result = client.tool('get_agent', {'id': other['id']})['result']
                self.assertTrue(result['isError']); self.assertNotIn(other['id'], json.dumps(result))
                result = output(client.tool('wait_agent', {'id': allowed['id'], 'until': 'exited', 'timeout_ms': 10}))
                self.assertEqual(result['data']['receipt']['exit_code'], 0)
                bad = client.tool('get_agent', {'id': allowed['id'], 'repository': '/PRIVATE_OUTSIDE'})
                self.assertTrue('error' in bad or bad['result'].get('isError'))
            self.assertEqual(snapshot(box), before)
            self.assertFalse((a.runtime(box) / 'coordinator.json').exists())

    def test_task_repository_scope_does_not_probe_excluded_replaced_repo(self):
        with a.fixture() as box:
            source = x.repo(box)
            other_repo = box.root / 'other-repo'
            box.run(['git', 'init', '-b', 'main', other_repo])
            box.run(['git', '-C', other_repo, '-c', 'user.name=fixture', '-c',
                     'user.email=fixture@example.invalid', 'commit', '--allow-empty', '-m', 'fixture'])
            with a.coordinator(box):
                first = x.start(box, argv=['/usr/bin/true'])
                second = x.command(box, ['--backend', 'native', 'task', 'start', 'excluded-task',
                                        '--repo', str(other_repo), '--', '/usr/bin/true'])['data']
                a.await_exit(box, first['task']['id']); a.await_exit(box, second['task']['id'])
            other_repo.rename(box.root / 'moved-excluded-repo')
            await_runtime_cleanup(box)
            before = snapshot(box)
            with wire(box, ['--read-repo', str(source), '--read-backend', 'native']) as client:
                client.initialize()
                rows = output(client.tool('list_tasks'))['data']['items']
                self.assertEqual([r['id'] for r in rows], [first['task']['id']])
                self.assertEqual(rows[0]['repository'], str(source))
                self.assertEqual(rows[0]['receipt']['exit_code'], 0)
                self.assertNotIn(str(a.state(box)), json.dumps(rows))
                agents = output(client.tool('list_agents'))['data']['items']
                self.assertEqual([r['id'] for r in agents], [first['task']['id']])
                self.assertTrue(client.tool('get_task', {'id': second['task']['id']})['result']['isError'])
            self.assertEqual(snapshot(box), before)

    def test_protocol_eras_cannot_be_mixed_and_unknown_version_is_refused(self):
        with Sandbox() as box, wire(box) as client:
            client.initialize()
            self.assertIn('error', client.call('tools/list', metadata=False))
            self.assertIn('error', client.call('initialize', {'protocolVersion': MODERN, 'capabilities': {},
                                                           'clientInfo': {'name': 'fixture', 'version': '1'}}))
            ident = client.send('tools/list', {'_meta': {}}, metadata=False)
            # An explicit unsupported modern version must never execute a tool.
            client.proc.stdin.write((json.dumps({'jsonrpc': '2.0', 'id': 1000, 'method': 'tools/list',
                'params': {'_meta': {'io.modelcontextprotocol/protocolVersion': '2099-01-01',
                                    'io.modelcontextprotocol/clientCapabilities': {}}}}) + '\n').encode())
            client.proc.stdin.flush()
            self.assertEqual(client.receive()['id'], ident)
            reply = client.receive(); self.assertEqual(reply['id'], 1000); self.assertIn('error', reply)
            self.assertEqual(set(reply['error']['data']['supported']), {LEGACY, MODERN})


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--binary', required=True, type=Path)
    args, rest = parser.parse_known_args()
    BINARY = args.binary.resolve(strict=True)
    a.BINARY = BINARY
    x.BINARY = BINARY
    unittest.main(argv=[__file__, *rest])
