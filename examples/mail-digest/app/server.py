import json
import logging
import os
import re
from datetime import date, datetime, timedelta, timezone
from html import escape
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlparse

from app.digest import build_digest
from app.mailbox import load_mailbox

DEFAULT_OWNER = "owner@example.com"

log = logging.getLogger("app.server")

STYLE = (
    "body{font-family:system-ui,sans-serif;max-width:42rem;margin:1rem auto;padding:0 1rem;line-height:1.5}"
    "nav a{margin-right:1rem}"
    ".badge{display:inline-block;padding:0 .4em;border-radius:.3em;background:#e3e8ef;"
    "font-size:.75em;font-weight:bold}"
)


def render_html(digest):
    day = date.fromisoformat(digest["day"])
    prev_day = None if day == date.min else (day - timedelta(days=1)).isoformat()
    next_day = None if day == date.max else (day + timedelta(days=1)).isoformat()
    e = escape
    parts = [
        "<!doctype html><html><head><meta charset=\"utf-8\">"
        "<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">",
        f"<title>Digest {e(digest['day'])}</title><style>{STYLE}</style></head><body>",
        f"<h1>Digest for {e(digest['day'])}</h1>",
        "<nav>"
        + (f"<a href=\"/?day={prev_day}\">Previous day</a> " if prev_day else "")
        + (f"<a href=\"/?day={next_day}\">Next day</a>" if next_day else "")
        + "</nav>",
        f"<p>{digest['total']} messages</p>",
    ]
    if digest["total"] == 0:
        parts.append("<p>No mail on this day</p>")
    if digest.get("skipped", 0) > 0:
        parts.append(f"<p>{int(digest['skipped'])} skipped files</p>")
    parts += [
        "<h2>Senders</h2><ul>",
    ]
    parts += [f"<li>{e(s['address'])} ({s['count']})</li>" for s in digest["senders"]]
    parts.append("</ul><h2>Threads</h2><ul>")
    for t in digest["threads"]:
        flag = " (needs reply)" if t["needs_reply"] else ""
        parts.append(f"<li>{e(t['subject'])}{flag}<ul>")
        parts += [f"<li><span class=\"badge\">{e(m.get('source', 'eml').upper())}</span> "
                  f"{e(m['sender'])}</li>" for m in t["messages"]]
        parts += [f"<li>{e(i)}</li>" for i in t["action_items"]]
        parts.append("</ul></li>")
    parts.append("</ul><h2>Action items</h2><ul>")
    parts += [f"<li>{e(i)}</li>" for i in digest["action_items"]]
    parts.append("</ul></body></html>")
    return "".join(parts)


class Handler(BaseHTTPRequestHandler):
    def _send(self, status, content_type, body):
        data = body.encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def _digest(self, query):
        params = parse_qs(query, keep_blank_values=True)
        raw = params.get("day", params.get("date", [None]))[0]
        if raw is None:
            day = datetime.now(timezone.utc).date()
        else:
            if not re.fullmatch(r"[0-9]{4}-[0-9]{2}-[0-9]{2}", raw):
                return None
            try:
                day = date.fromisoformat(raw)
            except ValueError:
                return None
        owner = os.environ.get("OWNER_EMAIL", DEFAULT_OWNER)
        messages, skipped = load_mailbox(self.server.mail_dir)
        digest = build_digest(messages, day, owner, skipped)
        return digest

    def do_GET(self):
        try:
            self._route()
        except Exception:
            log.exception("error handling GET %s", self.path)
            self._send(500, "text/plain; charset=utf-8", "internal error")

    def _route(self):
        url = urlparse(self.path)
        if url.path == "/health":
            self._send(200, "text/plain; charset=utf-8", "ok")
        elif url.path in ("/api/digest", "/"):
            digest = self._digest(url.query)
            if digest is None:
                self._send(400, "text/plain; charset=utf-8", "invalid date")
            elif url.path == "/":
                self._send(200, "text/html; charset=utf-8", render_html(digest))
            else:
                self._send(200, "application/json", json.dumps(digest))
        else:
            self.send_error(404)

    def log_request(self, code="-", size="-"):
        if str(code).isdigit() and int(code) >= 400:
            log.warning("%s %s -> %s", self.command, self.path, code)

    def log_message(self, format, *args):
        pass


def make_server(host, port, mail_dir=None):
    server = ThreadingHTTPServer((host, port), Handler)
    server.mail_dir = mail_dir or os.environ.get("MAIL_DIR", "samples/mail")
    return server


if __name__ == "__main__":
    make_server("0.0.0.0", 8080).serve_forever()
