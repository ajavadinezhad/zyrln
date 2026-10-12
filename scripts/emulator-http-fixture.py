#!/usr/bin/env python3
"""Loopback-only emulator fixture with deterministic socket faults."""
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import socket, struct, time
class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == '/stall':
            time.sleep(30)
            return
        if self.path == '/reset':
            self.connection.setsockopt(socket.SOL_SOCKET, socket.SO_LINGER, struct.pack('ii', 1, 0))
            self.connection.close()
            return
        if self.path == '/close':
            self.connection.close()
            return
        body = b'zyrln-emulator-fixture'
        self.send_response(200)
        self.send_header('Content-Length', str(1000 if self.path == '/truncate' else len(body)))
        self.end_headers()
        self.wfile.write(body)
ThreadingHTTPServer(('127.0.0.1', 8765), Handler).serve_forever()
