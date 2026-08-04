"""mcp_probe.py - JSON-RPC client for testing codeRAG MCP tools.

Implements FastMCP's split SSE transport over stdlib sockets.
Uses raw byte streaming with proper chunked-transfer decoding and SSE parsing.
"""

from __future__ import annotations

import json
import os
import re
import socket
import sys
import time
from typing import Any

HOST = os.environ.get("MCP_HOST", "localhost")
PORT = int(os.environ.get("MCP_PORT", "9622"))


class ChunkedStream:
    """Reads raw bytes from an HTTP/1.1 chunked-transfer-encoded response body."""

    def __init__(self, fp):
        self._fp = fp
        self._chunk_remaining = -1  # -1 = not in chunked body yet

    def enter_chunked(self):
        self._chunk_remaining = 0

    def read_chunk_line(self) -> bytes | None:
        """Read until \\r\\n. Returns line without trailing \\r\\n, or None on EOF."""
        buf = b""
        while True:
            b = self._fp.read(1)
            if not b:
                return None
            if b == b"\n":
                return buf.rstrip(b"\r")
            buf += b
            if len(buf) > 4096:
                return buf

    def _next_chunk_size(self) -> int | None:
        if self._chunk_remaining != 0:
            return self._chunk_remaining
        size_line = self.read_chunk_line()
        if size_line is None:
            return None
        try:
            size = int(size_line.split(b";", 1)[0].strip(), 16)
        except ValueError:
            return None
        if size == 0:
            self.read_chunk_line()  # trailing CRLF after final 0 chunk
            return 0
        self._chunk_remaining = size
        return size

    def read(self, n: int) -> bytes:
        if self._chunk_remaining == -1:
            return self._fp.read(n)
        if self._chunk_remaining == 0:
            size = self._next_chunk_size()
            if size is None or size == 0:
                return b""
        take = min(n, self._chunk_remaining)
        if take == 0:
            return b""
        data = self._fp.read(take)
        self._chunk_remaining -= len(data)
        return data


def _read_http_headers(sock_file) -> tuple[str, dict[str, str]]:
    """Read status line + headers. Returns (status, headers)."""
    raw_status = sock_file.readline()
    status = raw_status.decode("ascii", errors="replace").rstrip()
    headers: dict[str, str] = {}
    while True:
        line = sock_file.readline()
        if not line:
            break
        s = line.decode("ascii", errors="replace").rstrip()
        if s == "":
            break
        if ":" in s:
            k, _, v = s.partition(":")
            headers[k.strip().lower()] = v.strip()
    return status, headers


def _http_request(host: str, port: int, request: str, body: bytes = b"") -> tuple[str, dict[str, str]]:
    sock = socket.create_connection((host, port), timeout=15)
    sock.sendall(request.encode("ascii") + body)
    sock.settimeout(15)
    fp = sock.makefile("rb", buffering=0)
    status, headers = _read_http_headers(fp)
    body_len = int(headers.get("content-length", "0")) if "content-length" in headers else 0
    body_bytes = fp.read(body_len) if body_len else b""
    sock.close()
    return status, headers


def _open_sse_session(host: str, port: int) -> tuple[socket.socket, ChunkedStream, str]:
    """Open GET /sse. Return (socket, chunked_stream, session_id)."""
    sock = socket.create_connection((host, port), timeout=15)
    sock.sendall(
        f"GET /sse HTTP/1.1\r\nHost: {host}:{port}\r\n"
        f"Accept: text/event-stream\r\nCache-Control: no-cache\r\n"
        f"Connection: keep-alive\r\n\r\n".encode("ascii")
    )
    sock.settimeout(30)
    fp = sock.makefile("rb", buffering=0)
    status, headers = _read_http_headers(fp)
    if "200" not in status:
        sock.close()
        raise RuntimeError(f"GET /sse: {status}")

    chunked = ChunkedStream(fp)
    if headers.get("transfer-encoding") == "chunked":
        chunked.enter_chunked()

    deadline = time.monotonic() + 10
    session_id = None
    buf = b""
    while time.monotonic() < deadline and session_id is None:
        chunk = chunked.read(4096)
        if not chunk:
            break
        buf += chunk
        for line in buf.split(b"\n"):
            line = line.strip()
            if line.startswith(b"data:") and b"session_id=" in line:
                m = re.search(rb"session_id=([a-f0-9]+)", line)
                if m:
                    session_id = m.group(1).decode("ascii")
        if session_id:
            break

    if not session_id:
        sock.close()
        raise RuntimeError("no endpoint event received within 10s")

    return sock, chunked, session_id


def _post_message(host: str, port: int, session_id: str, body: bytes) -> str:
    """POST /messages/?session_id=... Returns status line."""
    sock = socket.create_connection((host, port), timeout=15)
    sock.sendall(
        f"POST /messages/?session_id={session_id} HTTP/1.1\r\n"
        f"Host: {host}:{port}\r\n"
        f"Content-Type: application/json\r\n"
        f"Accept: application/json, text/event-stream\r\n"
        f"Content-Length: {len(body)}\r\n"
        f"Connection: close\r\n\r\n".encode("ascii") + body
    )
    sock.settimeout(15)
    fp = sock.makefile("rb", buffering=0)
    status, _ = _read_http_headers(fp)
    sock.close()
    return status


def _read_sse_event(chunked: ChunkedStream, timeout: float = 30.0) -> dict | None:
    """Read one SSE event from the chunked stream. Returns {event, data, json?} or None."""
    deadline = time.monotonic() + timeout
    event_type = "message"
    data_lines: list[str] = []
    buf = b""
    sock_fp = chunked._fp

    while time.monotonic() < deadline:
        if b"\n" in buf:
            line, _, buf = buf.partition(b"\n")
            line = line.rstrip(b"\r").decode("utf-8", errors="replace")
            if line == "":
                if not data_lines:
                    continue
                ev = {"event": event_type, "data": "\n".join(data_lines)}
                try:
                    ev["json"] = json.loads(ev["data"])
                except json.JSONDecodeError:
                    pass
                return ev
            if line.startswith(":"):
                continue
            if ":" in line:
                f, _, v = line.partition(":")
                v = v.lstrip(" ")
            else:
                f, v = line, ""
            if f == "event":
                event_type = v
            elif f == "data":
                data_lines.append(v)
        else:
            chunked._fp = sock_fp
            chunk = chunked.read(4096)
            chunked._fp = sock_fp
            if not chunk:
                if not data_lines:
                    return None
                return {"event": event_type, "data": "\n".join(data_lines)}
            buf += chunk
    return None


def call_tool(tool: str, arguments: dict[str, Any]) -> dict[str, Any]:
    sock, chunked, session_id = _open_sse_session(HOST, PORT)
    try:
        # MCP handshake: initialize + initialized notification
        init_body = json.dumps({
            "jsonrpc": "2.0",
            "id": "probe-init",
            "method": "initialize",
            "params": {
                "protocolVersion": "2025-11-25",
                "capabilities": {},
                "clientInfo": {"name": "mcp_probe", "version": "1.0"},
            },
        }).encode("utf-8")
        status = _post_message(HOST, PORT, session_id, init_body)
        if "202" not in status and "200" not in status:
            return {"tool": tool, "ok": False, "result": None, "error": f"initialize POST: {status or 'empty'}"}

        deadline = time.monotonic() + 10
        init_ok = False
        while time.monotonic() < deadline:
            ev = _read_sse_event(chunked, timeout=5)
            if ev is None:
                continue
            if ev.get("json", {}).get("id") == "probe-init":
                if "error" in ev["json"]:
                    return {"tool": tool, "ok": False, "result": None,
                            "error": f"initialize error: {ev['json']['error']}"}
                init_ok = True
                break
        if not init_ok:
            return {"tool": tool, "ok": False, "result": None, "error": "no initialize response"}

        notif_body = json.dumps({
            "jsonrpc": "2.0",
            "method": "notifications/initialized",
            "params": {},
        }).encode("utf-8")
        _post_message(HOST, PORT, session_id, notif_body)

        # Now call the tool
        body = json.dumps({
            "jsonrpc": "2.0",
            "id": f"probe-{tool}",
            "method": "tools/call",
            "params": {"name": tool, "arguments": arguments},
        }).encode("utf-8")
        status = _post_message(HOST, PORT, session_id, body)
        if "202" not in status and "200" not in status:
            return {"tool": tool, "ok": False, "result": None, "error": f"tools/call POST: {status or 'empty'}"}

        deadline = time.monotonic() + 30
        while time.monotonic() < deadline:
            ev = _read_sse_event(chunked, timeout=10)
            if ev is None:
                continue
            if "json" not in ev:
                continue
            payload = ev["json"]
            if not isinstance(payload, dict):
                continue
            if not str(payload.get("id", "")).startswith("probe-"):
                continue
            if "error" in payload:
                err = payload["error"]
                msg = err.get("message") if isinstance(err, dict) else str(err)
                return {"tool": tool, "ok": False, "result": None, "error": msg}
            result = payload.get("result", {})
            if isinstance(result, dict) and "content" in result:
                parts = [c.get("text", "") for c in result["content"]
                         if isinstance(c, dict) and c.get("type") == "text"]
                if parts:
                    try:
                        result = json.loads("\n".join(parts))
                    except json.JSONDecodeError:
                        result = "\n".join(parts)
            return {"tool": tool, "ok": True, "result": result, "error": None}
        return {"tool": tool, "ok": False, "result": None, "error": "no JSON-RPC response within timeout"}
    finally:
        try:
            sock.close()
        except OSError:
            pass


def main() -> int:
    if len(sys.argv) < 2:
        print(__doc__, file=sys.stderr)
        return 2
    tool = sys.argv[1]
    if len(sys.argv) >= 3:
        try:
            arguments = json.loads(sys.argv[2])
        except json.JSONDecodeError as e:
            print(json.dumps({"tool": tool, "ok": False, "error": f"bad args JSON: {e}"}))
            return 2
    else:
        arguments = {}
    try:
        out = call_tool(tool, arguments)
    except Exception as e:
        out = {"tool": tool, "ok": False, "result": None, "error": str(e)}
    print(json.dumps(out, ensure_ascii=False))
    return 0 if out.get("ok") else 1


if __name__ == "__main__":
    raise SystemExit(main())
