"""Deterministic unavailable responses; never call real Gmail or AI providers."""
from http.server import BaseHTTPRequestHandler, HTTPServer
import json


class Handler(BaseHTTPRequestHandler):
    def respond(self):
        body = json.dumps({"error": "This integration is unavailable in the local demo."}).encode()
        self.send_response(503)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    do_GET = respond
    do_POST = respond
    do_PUT = respond
    do_DELETE = respond
    do_PATCH = respond


HTTPServer(("0.0.0.0", 8080), Handler).serve_forever()
