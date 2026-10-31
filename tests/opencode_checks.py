#!/usr/bin/env python3
"""Explicit no-inference OpenCode v2 SDK/loader/event checks in an owned fixture."""
import argparse
import base64
import hashlib
import json
from pathlib import Path
import secrets
import select
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

from sandbox import Sandbox, wait_for

VERSION = '2.2.5'


def check(binary):
    with Sandbox() as box:
        version = box.run([binary, '--version']).stdout.strip()
        if version != 'opencode v' + VERSION:
            raise ValueError('local runtime fixture requires exact OpenCode ' + VERSION)
        box.run(['/usr/bin/git', 'init', '-q'])
        plugin = box.root / 'repo/.opencode/plugins/txm-observer'
        plugin.mkdir(parents=True)
        observed = box.root / 'observed.json'
        ready = box.root / 'plugin-ready.json'
        session = 'ses_txm_' + secrets.token_hex(16)
        directory = str(box.root / 'repo')
        # Values enter source only as JSON literals; no shell or command callback.
        source = '''import { Plugin } from "@opencode/plugin";
export default Plugin.define({
  id: "txm-offline-observer",
  async setup(ctx) {
    if (ctx.app.version !== VERSION || ctx.location.directory !== DIRECTORY)
      throw new Error("fixture location/version mismatch");
    const controller = new AbortController();
    const rows = [];
    await Bun.write(READY, JSON.stringify({version:ctx.app.version,directory:ctx.location.directory}));
    void (async () => {
      try {
        for await (const event of ctx.event.subscribe({types:["session.created","session.renamed"],signal:controller.signal})) {
          if (event.data.sessionID !== SESSION) continue;
          if (rows.length >= 4) throw new Error("fixture event quota");
          rows.push({id:event.id,created:Number(event.created),type:event.type,sessionID:event.data.sessionID,
            aggregateID:event.durable?.aggregateID,sequence:event.durable?.seq,version:event.durable?.version});
          await Bun.write(OBSERVED, JSON.stringify(rows));
        }
      } catch (error) { if (!controller.signal.aborted) throw error; }
    })();
    return () => controller.abort();
  }
});
'''
        constants = {'VERSION': VERSION, 'DIRECTORY': directory, 'READY': str(ready),
                     'SESSION': session, 'OBSERVED': str(observed)}
        source = '\n'.join('const ' + k + ' = ' + json.dumps(v) + ';' for k, v in constants.items()) + '\n' + source
        (plugin / 'index.ts').write_text(source)
        # No inference or update activity; all provider use denied inside this
        # disposable config. This does not alter an operator's trust/policies.
        config = {'plugins': [str(plugin)], 'snapshots': False, 'warming': False,
                  'policies': [{'action': 'provider.use', 'resource': '*', 'effect': 'deny'}]}
        (box.root / 'repo/opencode.json').write_text(json.dumps(config))
        global_config = box.root / 'config/opencode'
        global_config.mkdir(exist_ok=True)
        (global_config / 'opencode.json').write_text(json.dumps({'update': 'disable'}))
        password = secrets.token_hex(24)
        box.env['OPENCODE_PASSWORD'] = password
        authorization = 'Basic ' + base64.b64encode(('opencode:' + password).encode()).decode()
        processes = []

        def start():
            proc = box.start([binary, 'serve', '--hostname', '127.0.0.1', '--port', '0', '--stdio'],
                             stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            processes.append(proc)
            available, _, _ = select.select([proc.stdout], [], [], 10)
            if not available:
                raise AssertionError('local server startup deadline')
            address = json.loads(proc.stdout.readline())['url']
            parsed = urllib.parse.urlparse(address)
            if parsed.scheme != 'http' or parsed.hostname != '127.0.0.1' or not parsed.port:
                raise AssertionError('server did not bind loopback')
            return proc, address

        def request(address, path, body=None, authenticated=True):
            headers = {'Authorization': authorization} if authenticated else {}
            if body is not None:
                headers['Content-Type'] = 'application/json'
            req = urllib.request.Request(address + path,
                    data=None if body is None else json.dumps(body).encode(), headers=headers)
            with urllib.request.urlopen(req, timeout=5) as response:
                return json.load(response)

        query = '/api/plugin?' + urllib.parse.urlencode({'location[directory]': directory})
        try:
            proc, address = start()
            try:
                request(address, query, authenticated=False)
            except urllib.error.HTTPError as error:
                if error.code != 401:
                    raise
            else:
                raise AssertionError('unauthenticated inventory was accepted')

            def loaded():
                rows = request(address, query)['data']
                return any(row['id'] == 'txm-offline-observer' and
                           row['state']['status'] == 'active' and
                           row['source']['type'] == 'local' for row in rows)
            wait_for(loaded, timeout=10)
            wait_for(ready.exists)
            assert json.loads(ready.read_text()) == {'version': VERSION, 'directory': directory}
            request(address, '/api/session', {'id': 'ses_other_' + secrets.token_hex(16),
                    'location': {'directory': directory}, 'title': 'PRIVATE_SYNTHETIC_TITLE'})
            request(address, '/api/session', {'id': session, 'location': {'directory': directory},
                    'title': 'PRIVATE_SYNTHETIC_TITLE'})
            wait_for(observed.exists)
            rows = json.loads(observed.read_text())
            assert len(rows) == 1 and rows[0]['sessionID'] == session, rows
            assert rows[0]['type'] == 'session.created' and rows[0]['id'].startswith('evt_'), rows
            # Compare actual clock units without claiming turn/process completion.
            assert abs(rows[0]['created'] / 1000 - time.time()) < 10, rows
            assert rows[0]['version'] == 1 and isinstance(rows[0]['sequence'], int), rows
            assert 'PRIVATE_SYNTHETIC_TITLE' not in observed.read_text()
            before = observed.read_bytes()
            # A verified owned crash followed by a fresh server/plugin load must
            # not replay historical events into an apparently fresh observation.
            proc.kill(); proc.communicate(timeout=8)
            ready.unlink()
            proc, address = start()
            wait_for(loaded, timeout=10)
            wait_for(ready.exists)
            time.sleep(.3)
            assert observed.read_bytes() == before
            with Path(binary).open('rb') as executable:
                binary_hash = hashlib.file_digest(executable, 'sha256').hexdigest()
            return {'passed': True, 'version': version, 'runtime_binary_sha256': binary_hash,
                    'scope': 'local_sdk_loader_events_no_inference',
                    'sdk_import': '@opencode/plugin', 'sdk_package_reference': VERSION,
                    'plugin_source_sha256': hashlib.sha256(source.encode()).hexdigest(),
                    'checks': ['loopback_basic_auth', 'Plugin.define_setup', 'exact_location',
                               'exact_session_filter', 'created_milliseconds', 'durable_version_sequence',
                               'metadata_only', 'crash_restart_live_only'],
                    'unverified': ['execution', 'permission', 'interrupt', 'provider_turn_completion',
                                   'txm_canonical_activation', 'model_lifecycle']}
        finally:
            for proc in processes:
                if proc.poll() is None:
                    proc.terminate()
                _, stderr = proc.communicate(timeout=8)
                if b'DATA RACE' in stderr:
                    raise AssertionError('runtime race diagnostic')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--opencode', required=True, type=Path)
    parser.add_argument('--report', required=True, type=Path)
    args = parser.parse_args()
    # Refuse evidence overwrite, and avoid saving any password or server logs.
    if args.report.exists():
        parser.error('report already exists')
    result = check(str(args.opencode.resolve(strict=True)))
    with args.report.open('x') as output:
        json.dump(result, output, indent=2)
        output.write('\n')
    print(json.dumps(result))


if __name__ == '__main__':
    main()
