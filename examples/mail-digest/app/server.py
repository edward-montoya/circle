import json
import os
from datetime import date, datetime, timedelta, timezone
from html import escape
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlparse

from app.digest import build_digest
from app.mailbox import load_mailbox

DEFAULT_OWNER = "owner@example.com"


def render_html(digest):
    day = date.fromisoformat(digest["date"])
    prev_day = (day - timedelta(days=1)).isoformat()
    next_day = (day + timedelta(days=1)).isoformat()
    e = escape
    parts = [
        "<!doctype html><html><head><meta charset=\"utf-8\">",
        f"<title>Digest {e(digest['date'])}</title></head><body>",
        f"<h1>Digest for {e(digest['date'])}</h1>",
        f"<nav><a href=\"/?date={prev_day}\">Previous day</a> "
        f"<a href=\"/?date={next_day}\">Next day</a></nav>",
        f"<p>{digest['total']} messages</p>",
    ]
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
        raw = parse_qs(query).get("date", [None])[0]
        if raw is None:
            day = datetime.now(timezone.utc).date()
        else:
            try:
                day = datetime.strptime(raw, "%Y-%m-%d").date()
            except ValueError:
                return None
        owner = os.environ.get("OWNER_EMAIL", DEFAULT_OWNER)
        messages, skipped = load_mailbox(self.server.mail_dir)
        return build_digest(messages, day, owner, skipped)

    def do_GET(self):
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

    def log_message(self, format, *args):
        pass


def make_server(host, port, mail_dir=None):
    server = ThreadingHTTPServer((host, port), Handler)
    server.mail_dir = mail_dir or os.environ.get("MAIL_DIR", "samples/mail")
    return server


if __name__ == "__main__":
    make_server("0.0.0.0", 8080).serve_forever()
