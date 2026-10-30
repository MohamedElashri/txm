#!/usr/bin/env python3
"""Phase 1 native framing/permissions/lifecycle and real tmux observer checks."""
import argparse
import concurrent.futures
import threading
import base64
import fcntl
import json
import os
from pathlib import Path
import secrets
import shutil
import socket
import struct
import subprocess
import sys
import termios
import time
import unittest
from sandbox import Sandbox, native_server, wait_for
from wire import Peer, packet, read, HEADER, HELLO, HELLO_OK, REQUEST, RESPONSE, DATA, RESIZE, ERROR


def snapshot(sock):
    with Peer(sock) as c:
        return base64.b64decode(c.request('snapshot', format='plain', lines=100, max_bytes=1048576)['content'])


def tty_attach(box, args):
    master, slave = os.openpty()
    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', 24, 80, 0, 0))
    original = termios.tcgetattr(slave)
    proc = box.start([BINARY, *args], stdin=slave, stdout=slave, stderr=slave)
    return proc, master, slave, original


class NativeChecks(unittest.TestCase):
    def test_fragmentation_coalescing_and_malformed_frames(self):
        with Sandbox() as box, native_server(box, BINARY) as (server, sock, _):
            with socket.socket(socket.AF_UNIX) as c:
                c.settimeout(6)
                c.connect(str(sock))
                for byte in packet(HELLO, 1, {'role': 'control'}):
                    c.sendall(bytes([byte]))
                    time.sleep(0.001)
                self.assertEqual(read(c)[0], HELLO_OK)
                c.sendall(packet(REQUEST, 2, {'op': 'status'}) + packet(REQUEST, 3, {'op': 'status'}))
                for ident in (2, 3):
                    kind, request_id, data = read(c)
                    self.assertEqual((kind, request_id), (RESPONSE, ident))
                    self.assertEqual(json.loads(data)['data']['process_state'], 'running')
            bad_packets = [
                packet(HELLO, 1, {'role': 'control'}, version=2),
                HEADER.pack(b'TXM1', 1, REQUEST, 0, 2, 1048577),
                HEADER.pack(b'TXM1', 1, DATA, 0, 0, 0),
                packet(HELLO, 1, b'{"role":"control","role":"observer"}'),
                packet(HELLO, 1, b'{"role":"control","unknown":true}'),
                packet(HELLO, 1, b'{"role":"control"}')[:-3],
            ]
            for bad in bad_packets:
                with self.subTest(packet=bad[:20]), socket.socket(socket.AF_UNIX) as c:
                    c.settimeout(6)
                    c.connect(str(sock))
                    c.sendall(bad)
                    c.shutdown(socket.SHUT_WR)
                    kind, _, data = read(c)
                    self.assertEqual(kind, ERROR)
                    self.assertIn(json.loads(data)['code'], ('protocol_error', 'unsupported_version'))
                self.assertIsNone(server.poll())
            with Peer(sock, 'control') as peer:
                peer.socket.sendall(packet(REQUEST, 2, b'{"op":"input","data":"U0VOVElORUwK","op":"kill"}'))
                self.assertEqual(read(peer.socket)[0], ERROR)
            self.assertNotIn(b'SENTINEL', snapshot(sock))

    def test_observer_denies_all_mutations_and_resize_is_real(self):
        with Sandbox() as box:
            dims = box.root / 'dims.json'
            probe = box.root / 'probe.py'
            probe.write_text(f'''import fcntl,json,signal,struct,termios,time
from pathlib import Path
path=Path({str(dims)!r})
def save(*_):
    rows,cols,_,_=struct.unpack('HHHH',fcntl.ioctl(0,termios.TIOCGWINSZ,b'\\0'*8))
    temp=path.with_suffix('.tmp');temp.write_text(json.dumps([rows,cols]));temp.replace(path)
signal.signal(signal.SIGWINCH,save)
save()
time.sleep(120)
''')
            with native_server(box, BINARY, argv=[sys.executable, probe]) as (server, sock, _):
                wait_for(dims.exists)
                with Peer(sock, 'control') as control:
                    control.socket.sendall(packet(RESIZE, 2, struct.pack('!HH', 90, 30)))
                    self.assertIsNone(json.loads(read(control.socket)[2])['error'])
                wait_for(lambda: json.loads(dims.read_text()) == [30, 90])
                with Peer(sock) as observer:
                    observer.socket.sendall(packet(RESIZE, 2, struct.pack('!HH', 40, 10)))
                    observer.next_id = 2
                    self.assertEqual(json.loads(read(observer.socket)[2])['error']['code'], 'denied')
                    for op, fields in [('input', {'data': base64.b64encode(b'SENTINEL\n').decode()}), ('kill', {})]:
                        with self.assertRaises(RuntimeError):
                            observer.request(op, **fields)
                    observer.socket.sendall(packet(DATA, 0, b'SENTINEL\n'))
                    self.assertEqual(json.loads(read(observer.socket)[2])['code'], 'denied')
                    self.assertEqual(observer.request('status')['process_state'], 'running')
                self.assertIsNone(server.poll())
                self.assertEqual(json.loads(dims.read_text()), [30, 90])
                self.assertNotIn(b'SENTINEL', snapshot(sock))

    def test_input_ack_and_kill_leave_other_session_alive(self):
        with Sandbox() as box, native_server(box, BINARY, 'first') as (first, sock, child), native_server(box, BINARY, 'second') as (second, second_sock, _):
            box.run([BINARY, 'exec', 'first', '0', '0', 'TXM_INPUT_SENTINEL'])
            wait_for(lambda: b'TXM_INPUT_SENTINEL' in snapshot(sock))
            box.run([BINARY, 'delete', 'first'])
            self.assertEqual(first.wait(timeout=3), 0)
            self.assertFalse(sock.exists())
            self.assertIsNone(second.poll())
            self.assertIn('second', box.run([BINARY, 'list']).stdout)
            with Peer(second_sock) as c:
                self.assertEqual(c.request('status')['process_state'], 'running')

    def test_duplicate_and_legacy_sockets_are_untouched(self):
        with Sandbox() as box, native_server(box, BINARY) as (server, sock, _):
            original = sock.stat().st_ino
            duplicate = box.run([BINARY, 'server', '--instance', secrets.token_hex(16), 'sample', '/bin/true'], check=False)
            self.assertNotEqual(duplicate.returncode, 0)
            self.assertEqual(sock.stat().st_ino, original)
            self.assertIsNone(server.poll())
            self.assertNotEqual(box.run([BINARY, 'create', 'sample'], check=False).returncode, 0)
            legacy_path = box.root / 'tmp/txm-legacy.sock'
            with socket.socket(socket.AF_UNIX) as legacy:
                legacy.bind(str(legacy_path))
                legacy.listen()
                legacy.setblocking(False)
                legacy_inode = legacy_path.stat().st_ino
                for args in (['attach', 'legacy'], ['delete', 'legacy'], ['create', 'legacy'], ['nuke']):
                    result = box.run([BINARY, *args], check=False)
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn('legacy', result.stderr)
                self.assertEqual(legacy_path.stat().st_ino, legacy_inode)
                with self.assertRaises(BlockingIOError):
                    legacy.accept()
                self.assertIn('legacy', box.run([BINARY, 'list']).stderr)

    def test_stale_paths_and_unsafe_runtime_are_not_live(self):
        with Sandbox() as box, native_server(box, BINARY) as (server, sock, _):
            stale = sock.parent / ('s-' + secrets.token_hex(16) + '.sock')
            with socket.socket(socket.AF_UNIX) as c:
                c.bind(str(stale))
                stale.chmod(0o600)
            inode = stale.stat().st_ino
            result = box.run([BINARY, 'list'])
            self.assertIn('sample', result.stdout)
            self.assertEqual(stale.stat().st_ino, inode)
            stale.unlink()  # This fixture created it and retained its inode.
            linked = sock.parent / ('s-' + secrets.token_hex(16) + '.sock')
            linked.symlink_to(sock)
            self.assertNotEqual(box.run([BINARY, 'list'], check=False).returncode, 0)
            self.assertIsNone(server.poll())
            linked.unlink()
            runtime = Path(box.env['XDG_RUNTIME_DIR'])
            runtime.chmod(0o755)
            try:
                self.assertNotEqual(box.run([BINARY, 'list'], check=False).returncode, 0)
            finally:
                runtime.chmod(0o700)

    def test_read_only_local_detach_and_interrupt_restore_tty(self):
        with Sandbox() as box, native_server(box, BINARY) as (server, sock, _):
            for signal_mode in (False, True):
                proc, master, slave, old = tty_attach(box, ['attach', '-r', 'sample'])
                try:
                    wait_for(lambda: PeerStatus(sock) == 1)
                    if signal_mode:
                        proc.terminate()
                    else:
                        os.write(master, b'SENTINEL\n\x1c')
                    proc.wait(timeout=4)
                    self.assertEqual(proc.returncode, 130 if signal_mode else 0)
                    self.assertEqual(termios.tcgetattr(slave), old)
                    wait_for(lambda: PeerStatus(sock) == 0)
                    self.assertIsNone(server.poll())
                    self.assertNotIn(b'SENTINEL', snapshot(sock))
                finally:
                    os.close(master)
                    os.close(slave)

    def test_cli_startup_readiness_short_exit_and_concurrent_names(self):
        with Sandbox() as box:
            missing = box.root / 'missing-executable'
            self.assertNotEqual(box.run([BINARY, 'create', 'bad', missing], check=False).returncode, 0)
            # All launchers share a guarded runtime and name lock; only one wins.
            procs = [box.start([BINARY, 'create', 'race', '/bin/sleep', '3'], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL) for _ in range(4)]
            for proc in procs:
                proc.wait(timeout=7)
            winners = sum(proc.returncode == 0 for proc in procs)
            self.assertEqual(winners, 1)
            self.assertIn('race', box.run([BINARY, 'list']).stdout)
            box.run([BINARY, 'delete', 'race'])
            runtime = box.root / 'runtime/txm/v1'
            self.assertFalse(list(runtime.glob('s-*.sock')))
            result = box.run([BINARY, 'create', 'short', '/bin/true'], check=False)
            # If the child was observed running during readiness, success is a
            # truthful start; otherwise startup reports its early exit. Phase 2
            # classifies a handshake observing exit as not-found (3), rather
            # than the Phase 1 general failure (1). Cleanup stays mandatory.
            self.assertIn(result.returncode, (0, 1, 3))
            if result.returncode == 3:
                self.assertIn('native child already exited', result.stderr)
                self.assertEqual(result.stdout, '')
            wait_for(lambda: not list(runtime.glob('s-*.sock')), timeout=7)
            self.assertNotIn('short', box.run([BINARY, 'list']).stdout)

    def test_concurrent_attach_resize_and_output(self):
        with Sandbox() as box:
            gate=box.root/'emit-concurrent'
            payload=''.join(f'MSG-{i:04d}-λ-雪\n' for i in range(100))+'CONCURRENT_END\n'
            emitter=box.root/'concurrent.py'
            emitter.write_text(f"import sys,time\nfrom pathlib import Path\nwhile not Path({str(gate)!r}).exists(): time.sleep(0.01)\nfor line in {payload!r}.splitlines(keepends=True):\n sys.stdout.buffer.write(line.encode());sys.stdout.buffer.flush();time.sleep(0.003)\ntime.sleep(120)\n")
            with native_server(box,BINARY,argv=[sys.executable,emitter]) as (server,sock,_):
                ready=threading.Barrier(5,timeout=5)
                started=threading.Event()
                def viewer(index):
                    chunks=[]
                    with Peer(sock,'control' if index==0 else 'observer') as peer:
                        peer.attach();ready.wait()
                        resizes=0
                        while b'CONCURRENT_END' not in b''.join(chunks):
                            chunks.append(peer.recv_data());started.set()
                            if index==0 and resizes<3:
                                resizes+=1;peer.next_id+=1
                                peer.socket.sendall(packet(RESIZE,peer.next_id,struct.pack('!HH',80+resizes,24+resizes)))
                                while True:
                                    kind,ident,data=read(peer.socket)
                                    if kind==DATA:chunks.append(data)
                                    else:
                                        self.assertEqual((kind,ident),(RESPONSE,peer.next_id))
                                        self.assertIsNone(json.loads(data)['error']);break
                    return b''.join(chunks).replace(b'\r\n',b'\n').decode('utf-8')
                def inspect():
                    self.assertTrue(started.wait(5))
                    positions=[]
                    with Peer(sock) as peer:
                        for _ in range(4):
                            view=peer.request('snapshot',format='plain',lines=100,max_bytes=1048576)
                            base64.b64decode(view['content']).decode('utf-8')
                            positions.append(view['position']);time.sleep(0.01)
                    self.assertEqual(positions,sorted(positions))
                with concurrent.futures.ThreadPoolExecutor(max_workers=5) as workers:
                    viewers=[workers.submit(viewer,i) for i in range(4)]
                    inspecting=workers.submit(inspect)
                    ready.wait();gate.touch()
                    for future in viewers:self.assertEqual(future.result(timeout=10),payload)
                    inspecting.result(timeout=10)
                wait_for(lambda: PeerStatus(sock)==0)
                self.assertIsNone(server.poll())

    def test_process_group_kills_resistant_descendant(self):
        with Sandbox() as box:
            stopped = box.root / 'stop-grandchild'
            ready = box.root / 'grandchild-ready'
            identity = box.root / 'grandchild.json'
            descendant = box.root / 'descendant.py'
            descendant.write_text(f"import signal,time\nfrom pathlib import Path\nsignal.signal(signal.SIGTERM,signal.SIG_IGN)\nsignal.signal(signal.SIGHUP,signal.SIG_IGN)\nPath({str(ready)!r}).touch()\nend=time.monotonic()+120\nwhile time.monotonic()<end and not Path({str(stopped)!r}).exists(): time.sleep(0.02)\n")
            parent = box.root / 'parent.py'
            parent.write_text(f"import os,subprocess,sys,time,json\nfrom pathlib import Path\np=subprocess.Popen([sys.executable,{str(descendant)!r}])\nwhile not Path({str(ready)!r}).exists(): time.sleep(0.01)\nPath({str(identity)!r}).write_text(json.dumps([p.pid,os.getpgid(p.pid)]))\np.wait()\n")
            try:
                with native_server(box, BINARY, argv=[sys.executable, parent]) as (_, _, leader):
                    wait_for(identity.exists)
                    pid, group = json.loads(identity.read_text())
                    self.assertEqual(group, leader)
                    box.run([BINARY, 'delete', 'sample'])
                    def gone():
                        out = subprocess.run(['/bin/ps', '-o', 'stat=', '-p', str(pid)], capture_output=True, text=True)
                        return out.returncode != 0 or not out.stdout.strip() or out.stdout.strip().startswith('Z')
                    wait_for(gone, timeout=1)
            finally:
                # Cooperative fixture cleanup also works if the group-kill
                # assertion fails, without ever signalling a discovered PID.
                stopped.touch()

    def test_connection_limit_and_cancelled_handshake(self):
        with Sandbox() as box, native_server(box, BINARY) as (_, sock, _):
            peers=[]
            try:
                for _ in range(32):
                    peers.append(Peer(sock))
                with socket.socket(socket.AF_UNIX) as extra:
                    extra.settimeout(2)
                    extra.connect(str(sock))
                    try:
                        extra.sendall(packet(HELLO,1,{'role':'observer'}))
                        self.assertEqual(extra.recv(1),b'')
                    except (ConnectionResetError, BrokenPipeError):
                        # Excess clients can be closed before their HELLO write.
                        # Both send-side EPIPE and read-side reset prove refusal.
                        pass
                self.assertEqual(peers[0].request('status')['process_state'],'running')
            finally:
                for peer in peers:
                    peer.close()
            wait_for(lambda: PeerStatus(sock)==0)
        with Sandbox() as box:
            runtime=box.root/'runtime/txm/v1'
            runtime.mkdir(parents=True,mode=0o700)
            (runtime.parent).chmod(0o700)
            fake=runtime/('s-'+secrets.token_hex(16)+'.sock')
            with socket.socket(socket.AF_UNIX) as listener:
                listener.bind(str(fake));fake.chmod(0o600);listener.listen();listener.settimeout(3)
                proc=box.start([BINARY,'list'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
                conn,_=listener.accept()
                with conn:
                    conn.sendall(b'TX')
                    proc.terminate();proc.wait(timeout=3)
                    self.assertEqual(proc.returncode,130)

    @unittest.skipUnless(shutil.which('tmux'), 'tmux unavailable')
    def test_real_tmux_read_only_attach(self):
        with Sandbox() as box:
            box.start_tmux(shutil.which('tmux'))
            box.run([BINARY, 'create', 'sample', '/bin/sh'])
            proc, master, slave, old = tty_attach(box, ['attach', '-r', 'sample'])
            try:
                wait_for(lambda: 'read-only' in box.run(['tmux', 'list-clients', '-F', '#{client_flags}']).stdout)
                os.set_blocking(master, False)
                os.write(master, b'printf TXM_RO_SENTINEL\n')
                time.sleep(0.1)
                pane=box.run(['tmux','list-panes','-t','sample','-F','#{pane_id}']).stdout.strip()
                self.assertNotIn('TXM_RO_SENTINEL', box.run(['tmux', 'capture-pane', '-p', '-t', pane]).stdout)
                # Read-only tmux retains its detach binding.
                try:
                    while os.read(master, 65536):
                        pass
                except BlockingIOError:
                    pass
                os.write(master, b'\x02d')
                proc.wait(timeout=5)
                self.assertEqual(proc.returncode, 0)
                self.assertEqual(termios.tcgetattr(slave), old)
            finally:
                os.close(master)
                os.close(slave)


def PeerStatus(sock):
    with Peer(sock) as c:
        return c.request('status')['attached_clients']


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--binary', required=True, type=Path)
    parser.add_argument('tests',nargs='*')
    args = parser.parse_args()
    BINARY = args.binary.resolve(strict=True)
    unittest.main(argv=[sys.argv[0],*args.tests])
