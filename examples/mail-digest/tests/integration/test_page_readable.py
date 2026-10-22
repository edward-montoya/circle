import threading
import unittest
import urllib.error
import urllib.request
from unittest import mock

from app.server import make_server


class PageReadableTest(unittest.TestCase):
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
                return r.status, r.read().decode()
        except urllib.error.HTTPError as e:
            return e.code, e.read().decode()

    def test_page_has_viewport_and_inline_stylesheet(self):
        _, body = self.get("/?day=2025-03-10")
        self.assertIn('<meta name="viewport" content="width=device-width, initial-scale=1">', body)
        self.assertIn("<style>", body)

    def test_page_shows_source_badge(self):
        _, body = self.get("/?day=2025-03-10")
        self.assertIn('<span class="badge">EML</span>', body)

    def test_quiet_day_says_no_mail(self):
        _, body = self.get("/?day=2025-01-01")
        self.assertIn("No mail on this day", body)
        _, body = self.get("/?day=2025-03-10")
        self.assertNotIn("No mail on this day", body)

    def test_bad_request_is_logged(self):
        with self.assertLogs("app.server", level="WARNING") as cm:
            self.get("/?day=nope")
        self.assertTrue(any("400" in line for line in cm.output))

    def test_internal_error_is_logged_and_500(self):
        with mock.patch("app.server.load_mailbox", side_effect=RuntimeError("boom")):
            with self.assertLogs("app.server", level="ERROR") as cm:
                status, _ = self.get("/?day=2025-03-10")
        self.assertEqual(status, 500)
        self.assertTrue(any("boom" in line for line in cm.output))
