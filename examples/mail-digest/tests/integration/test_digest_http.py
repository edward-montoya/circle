import json
import os
import threading
import unittest
import urllib.error
import urllib.request

from app.server import make_server


class DigestHttpTest(unittest.TestCase):
    def setUp(self):
        self.server = make_server("127.0.0.1", 0, mail_dir="samples/mail")
        self.base = f"http://127.0.0.1:{self.server.server_address[1]}"
        threading.Thread(target=self.server.serve_forever, daemon=True).start()

    def tearDown(self):
        self.server.shutdown()
        self.server.server_close()

    def get(self, path):
        try:
            with urllib.request.urlopen(self.base + path) as r:
                return r.status, r.headers, r.read().decode()
        except urllib.error.HTTPError as e:
            return e.code, e.headers, e.read().decode()

    def test_api_returns_digest_for_date(self):
        status, headers, body = self.get("/api/digest?date=2025-03-10")
        self.assertEqual(status, 200)
        self.assertIn("application/json", headers["Content-Type"])
        digest = json.loads(body)
        self.assertEqual(digest["date"], "2025-03-10")
        self.assertEqual(digest["total"], 5)

    def test_api_quiet_day_is_empty_digest(self):
        status, _, body = self.get("/api/digest?date=2025-01-01")
        self.assertEqual(status, 200)
        digest = json.loads(body)
        self.assertEqual(digest["total"], 0)
        self.assertEqual(digest["threads"], [])

    def test_invalid_date_is_400(self):
        for path in ("/api/digest?date=nope", "/?date=2025-13-40"):
            self.assertEqual(self.get(path)[0], 400, path)

    def test_default_date_is_today(self):
        from datetime import datetime, timezone
        today = datetime.now(timezone.utc).date().isoformat()
        self.assertEqual(json.loads(self.get("/api/digest")[2])["date"], today)

    def test_html_page_has_day_links_and_content(self):
        status, headers, body = self.get("/?date=2025-03-10")
        self.assertEqual(status, 200)
        self.assertIn("text/html", headers["Content-Type"])
        self.assertIn('href="/?date=2025-03-09"', body)
        self.assertIn('href="/?date=2025-03-11"', body)
        self.assertIn("Q1 budget review", body)
        self.assertIn("priya@acme.example", body)

    def test_html_escapes_text(self):
        from app import server
        html = server.render_html({
            "date": "2025-03-10", "total": 1,
            "senders": [{"address": "<b>x</b>@e.example", "count": 1}],
            "threads": [{"subject": "<script>alert(1)</script>", "messages": [],
                         "needs_reply": True, "action_items": ["a & b?"]}],
            "action_items": ["a & b?"],
        })
        self.assertNotIn("<script>", html)
        self.assertNotIn("<b>x", html)
        self.assertIn("&lt;script&gt;", html)
        self.assertIn("a &amp; b?", html)


class OwnerEmailTest(unittest.TestCase):
    def setUp(self):
        self._saved = {k: os.environ.pop(k, None) for k in ("OWNER", "OWNER_EMAIL")}
        self.server = make_server("127.0.0.1", 0, mail_dir="samples/mail")
        self.base = f"http://127.0.0.1:{self.server.server_address[1]}"
        threading.Thread(target=self.server.serve_forever, daemon=True).start()

    def tearDown(self):
        self.server.shutdown()
        self.server.server_close()
        for k, v in self._saved.items():
            os.environ.pop(k, None)
            if v is not None:
                os.environ[k] = v

    def q1_needs_reply(self):
        url = self.base + "/api/digest?date=2025-03-10"
        with urllib.request.urlopen(url) as r:
            threads = json.loads(r.read())["threads"]
        return {t["subject"]: t["needs_reply"] for t in threads}["Q1 budget review"]

    def test_owner_email_is_used_case_insensitively(self):
        os.environ["OWNER_EMAIL"] = "priya@acme.example"
        self.assertTrue(self.q1_needs_reply())
        os.environ["OWNER_EMAIL"] = "OWNER@Example.COM"
        self.assertFalse(self.q1_needs_reply())

    def test_old_owner_variable_is_ignored(self):
        os.environ["OWNER"] = "priya@acme.example"
        self.assertFalse(self.q1_needs_reply())


class PdfSourceHttpTest(unittest.TestCase):
    def setUp(self):
        import shutil
        import tempfile
        from pathlib import Path
        from tests.support.pdf import write_pdf
        self.dir = Path(tempfile.mkdtemp())
        self.addCleanup(shutil.rmtree, self.dir)
        (self.dir / "a.eml").write_text(
            "From: a@e.example\nTo: owner@example.com\nSubject: Hello\n"
            "Date: Mon, 10 Mar 2025 09:00:00 +0000\n\nHi.\n")
        write_pdf(self.dir / "b.pdf", [[
            "From: p@e.example", "To: owner@example.com", "Subject: Printed <b>",
            "Date: Mon, 10 Mar 2025 10:00:00 +0000", "", "Body."]])
        self.server = make_server("127.0.0.1", 0, mail_dir=str(self.dir))
        self.base = f"http://127.0.0.1:{self.server.server_address[1]}"
        threading.Thread(target=self.server.serve_forever, daemon=True).start()
        self.addCleanup(self.server.server_close)
        self.addCleanup(self.server.shutdown)

    def get(self, path):
        with urllib.request.urlopen(self.base + path) as r:
            return r.read().decode()

    def add_bad_pdf(self):
        (self.dir / "bad.pdf").write_bytes(b"not a pdf")

    def test_api_shows_source_on_each_message(self):
        digest = json.loads(self.get("/api/digest?date=2025-03-10"))
        sources = {m["sender"]: m["source"]
                   for t in digest["threads"] for m in t["messages"]}
        self.assertEqual(sources, {"a@e.example": "eml", "p@e.example": "pdf"})

    def test_api_skipped_count(self):
        self.assertEqual(json.loads(self.get("/api/digest?date=2025-03-10"))["skipped"], 0)
        self.add_bad_pdf()
        self.assertEqual(json.loads(self.get("/api/digest?date=2025-03-10"))["skipped"], 1)

    def test_page_shows_badges_escaped(self):
        html = self.get("/?date=2025-03-10")
        self.assertIn("PDF", html)
        self.assertIn("EML", html)
        self.assertNotIn("<b>", html)

    def test_page_skipped_line_only_when_above_zero(self):
        self.assertNotIn("skipped", self.get("/?date=2025-03-10").lower())
        self.add_bad_pdf()
        self.assertIn("1 skipped", self.get("/?date=2025-03-10").lower())
