"""Protocol v1 test client; raw framing is also usable for adversarial cases."""
import base64
import json
import socket
import struct

HEADER = struct.Struct('!4sHBBQI')
HELLO, HELLO_OK, REQUEST, RESPONSE, DATA, RESIZE, CLOSE, ERROR, PING, PONG = range(1, 11)

def packet(kind, request_id, data=b'', version=1):
    if isinstance(data, dict):
        data = json.dumps(data, separators=(',', ':')).encode()
    return HEADER.pack(b'TXM1', version, kind, 0, request_id, len(data)) + data

def exact(sock, count):
    result = bytearray()
    while len(result) < count:
        data = sock.recv(count-len(result))
        if not data:
            raise EOFError('native peer disconnected')
        result.extend(data)
    return bytes(result)

def read(sock):
    magic, version, kind, flags, ident, size = HEADER.unpack(exact(sock, HEADER.size))
    if magic != b'TXM1' or version != 1 or flags or size > 1048576:
        raise AssertionError('invalid server frame')
    return kind, ident, exact(sock, size)

class Peer:
    def __init__(self, path, role='observer'):
        self.socket = socket.socket(socket.AF_UNIX)
        self.socket.settimeout(6)
        try:
            self.socket.connect(str(path))
            self.next_id = 1
            self.socket.sendall(packet(HELLO, 1, {'role': role}))
            kind, ident, data = read(self.socket)
            if kind != HELLO_OK or ident != 1:
                raise AssertionError(data.decode())
            self.identity = json.loads(data)
        except BaseException:
            self.socket.close()
            raise

    def request(self, op, **fields):
        self.next_id += 1
        self.socket.sendall(packet(REQUEST, self.next_id, {'op': op, **fields}))
        while True:
            kind, ident, data = read(self.socket)
            if kind == PING:
                self.socket.sendall(packet(PONG, ident))
                continue
            if kind != RESPONSE or ident != self.next_id:
                raise AssertionError((kind, ident, data[:100]))
            result = json.loads(data)
            if result['error'] is not None:
                raise RuntimeError(result['error'])
            return result['data']

    def attach(self):
        return base64.b64decode(self.request('attach')['content'])

    def recv_data(self):
        while True:
            kind, ident, data = read(self.socket)
            if kind == PING:
                self.socket.sendall(packet(PONG, ident))
            elif kind == DATA:
                return data
            else:
                raise AssertionError((kind, ident, data[:100]))

    def close(self):
        self.socket.close()
    def __enter__(self):
        return self
    def __exit__(self, *_):
        self.close()
