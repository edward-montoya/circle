import tempfile
import unittest
from datetime import datetime, timezone
from pathlib import Path

from app.mailbox import load_mailbox, load_messages

EML = """From: Alice Example <alice@example.com>
To: owner@example.com
Subject: Lunch plans
Date: {date}
Message-ID: <{n}@example.com>

Shall we meet at noon?
"""


def write(directory, name, date="Mon, 06 Jan 2025 23:30:00 -0500", n=1):
    Path(directory, name).write_text(EML.format(date=date, n=n))


class LoadMessagesTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)

    def test_parses_sender_utc_date_subject_and_body(self):
        write(self.tmp.name, "a.eml")
        [m] = load_messages(self.tmp.name)
        self.assertEqual(m.sender, "alice@example.com")
        self.assertEqual(m.date, datetime(2025, 1, 7, 4, 30, tzinfo=timezone.utc))
        self.assertEqual(m.subject, "Lunch plans")
        self.assertEqual(m.body.strip(), "Shall we meet at noon?")

    def test_skips_messages_without_parseable_date(self):
        write(self.tmp.name, "good.eml")
        write(self.tmp.name, "garbage.eml", date="not a date", n=2)
        Path(self.tmp.name, "nodate.eml").write_text(
            "From: bob@example.com\nSubject: Hi\n\nNo date here\n")
        self.assertEqual([m.subject for m in load_messages(self.tmp.name)], ["Lunch plans"])

    def test_ignores_files_that_are_not_eml(self):
        write(self.tmp.name, "a.eml")
        Path(self.tmp.name, "notes.txt").write_text("hello")
        self.assertEqual(len(load_messages(self.tmp.name)), 1)

    def test_sender_is_lower_cased(self):
        Path(self.tmp.name, "a.eml").write_text(
            EML.format(date="Mon, 06 Jan 2025 10:00:00 +0000", n=1)
            .replace("alice@example.com", "Alice@Example.COM"))
        [m] = load_messages(self.tmp.name)
        self.assertEqual(m.sender, "alice@example.com")

    def test_suffix_matches_case_insensitively(self):
        write(self.tmp.name, "A.EML")
        self.assertEqual(len(load_messages(self.tmp.name)), 1)

    def test_html_only_message_has_the_text_as_body(self):
        Path(self.tmp.name, "h.eml").write_text(
            "From: bob@example.com\nSubject: Hi\nDate: Mon, 06 Jan 2025 10:00:00 +0000\n"
            "Content-Type: text/html\n\n<html><body><p>Can you call me?</p></body></html>\n")
        [m] = load_messages(self.tmp.name)
        self.assertIn("Can you call me?", m.body)
        self.assertNotIn("<p>", m.body)

    def test_unreadable_undecodable_and_undated_files_are_skipped_and_counted(self):
        write(self.tmp.name, "good.eml")
        Path(self.tmp.name, "nodate.eml").write_text("From: b@example.com\n\nx\n")
        Path(self.tmp.name, "dir.eml").mkdir()
        Path(self.tmp.name, "bad.eml").write_bytes(
            b"From: b@example.com\nDate: Mon, 06 Jan 2025 10:00:00 +0000\n"
            b"Content-Type: text/plain; charset=no-such-charset\n"
            b"Content-Transfer-Encoding: 8bit\n\n\xff\xfe caf\xe9\n")
        messages, skipped = load_mailbox(self.tmp.name)
        self.assertEqual(len(messages), 1)
        self.assertEqual(skipped, 3)


SAMPLES = Path(__file__).resolve().parents[2] / "samples" / "mail"


class ShippedSamplesTest(unittest.TestCase):
    def test_samples_span_three_consecutive_days_with_a_reply_thread(self):
        messages = load_messages(SAMPLES)
        self.assertTrue(9 <= len(messages) <= 13, len(messages))
        days = sorted({m.date.date() for m in messages})
        self.assertEqual(len(days), 3)
        self.assertEqual((days[1] - days[0]).days, 1)
        self.assertEqual((days[2] - days[1]).days, 1)
        subjects = [m.subject for m in messages]
        self.assertTrue(any(s.lower().startswith("re:") for s in subjects))


if __name__ == "__main__":
    unittest.main()
