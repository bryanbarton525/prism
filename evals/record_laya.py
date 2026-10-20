#!/usr/bin/env python3
"""Loopback recorder for synthetic evals; every decision reaches real Laya."""
import argparse
import http.server
import json
import threading
import time
import urllib.error
import urllib.request


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--upstream", required=True)
    p.add_argument("--port", type=int, default=18011)
    p.add_argument("--output", required=True)
    args = p.parse_args()
    lock = threading.Lock()
    with open(args.output, "x") as journal:
        class Handler(http.server.BaseHTTPRequestHandler):
            def log_message(self, *_):
                pass

            def do_GET(self):
                self.forward()

            def do_POST(self):
                self.forward()

            def forward(self):
                if self.path not in ("/health", "/v1/systemone"):
                    self.send_error(404); return
                size = int(self.headers.get("Content-Length", "0"))
                if size < 0 or size > 2 << 20:
                    self.send_error(413); return
                body = self.rfile.read(size) if size else None
                start = time.time()
                record = dict(started=start, method=self.command, path=self.path, upstream=args.upstream)
                if body:
                    record["request"] = json.loads(body)
                request = urllib.request.Request(args.upstream.rstrip("/") + self.path, data=body, method=self.command, headers={"Content-Type": "application/json"})
                try:
                    with urllib.request.urlopen(request, timeout=10) as response:
                        status, result = response.status, response.read(2 << 20)
                except urllib.error.HTTPError as e:
                    status, result = e.code, e.read(2 << 20)
                except Exception as e:
                    status, result = 502, json.dumps({"error": str(e)}).encode()
                record.update(status=status, duration_ms=round((time.time() - start) * 1000, 3))
                try:
                    record["response"] = json.loads(result)
                except ValueError:
                    record["response_text"] = result.decode(errors="replace")
                with lock:
                    journal.write(json.dumps(record) + "\n"); journal.flush()
                self.send_response(status)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(result)))
                self.end_headers()
                try:
                    self.wfile.write(result)
                except BrokenPipeError:
                    pass

        server = http.server.ThreadingHTTPServer(("127.0.0.1", args.port), Handler)
        print(f"Recording real Laya calls on http://127.0.0.1:{args.port}", flush=True)
        try:
            server.serve_forever()
        except KeyboardInterrupt:
            pass
        finally:
            server.server_close()


if __name__ == "__main__":
    main()
