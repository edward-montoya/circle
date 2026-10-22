import tempfile
import unittest
from datetime import date, datetime, timezone
from pathlib import Path

from app.digest import build_digest
from app.mailbox import load_mailbox
from tests.support.pdf import write_pdf

HEADER = ["From: Carol Example <carol@example.com>", "To: owner@example.com",
          "Subject: Budget review", "Date: Tue, 07 Jan 2025 09:00:00 +0000", ""]


class PdfLoadTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.dir = Path(self.tmp.name)

    def test_pdf_email_becomes_message_with_pdf_source(self):
        write_pdf(self.dir / "a.pdf", [HEADER + ["Can you send the numbers?"]])
        messages, skipped = load_mailbox(self.dir)
        [m] = messages
        self.assertEqual(m.sender, "carol@example.com")
        self.assertEqual(m.date, datetime(2025, 1, 7, 9, 0, tzinfo=timezone.utc))
        self.assertEqual(m.subject, "Budget review")
        self.assertEqual(m.body.strip(), "Can you send the numbers?")
        self.assertEqual(m.source, "pdf")
        self.assertEqual(skipped, 0)

    def test_header_names_case_insensitive_wrapped_values_and_zone(self):
        write_pdf(self.dir / "a.pdf", [[
            "FROM: <carol@example.com>", "subject: Budget",
            "  review for Q1", "date: Mon, 06 Jan 2025 23:30:00 -0500", "", "Body"]])
        [m], _ = load_mailbox(self.dir)
        self.assertEqual(m.sender, "carol@example.com")
        self.assertEqual(m.subject, "Budget review for Q1")
        self.assertEqual(m.date, datetime(2025, 1, 7, 4, 30, tzinfo=timezone.utc))

    def test_unindented_wrapped_header_value_continues_its_header(self):
        write_pdf(self.dir / "a.pdf", [[
            "From: <carol@example.com>", "Subject: Budget", "review for Q1",
            "Date: Mon, 06 Jan 2025 23:30:00 -0500", "", "Body"]])
        [m], _ = load_mailbox(self.dir)
        self.assertEqual(m.subject, "Budget review for Q1")
        self.assertEqual(m.body.strip(), "Body")

    def test_unindented_wrapped_last_header_stays_in_its_header(self):
        write_pdf(self.dir / "a.pdf", [[
            "From: <carol@example.com>", "Date: Mon, 06 Jan",
            "2025 23:30:00 -0500", "", "Body"]])
        [m], _ = load_mailbox(self.dir)
        self.assertEqual(m.date, datetime(2025, 1, 7, 4, 30, tzinfo=timezone.utc))
        self.assertEqual(m.body.strip(), "Body")

    def test_unindented_line_continues_last_header_until_the_blank_line(self):
        write_pdf(self.dir / "a.pdf", [[
            "From: <carol@example.com>", "Date: Mon, 06 Jan 2025 23:30:00 -0500",
            "Subject: Budget", "review for Q1", "", "Body"]])
        [m], _ = load_mailbox(self.dir)
        self.assertEqual(m.subject, "Budget review for Q1")
        self.assertEqual(m.body.strip(), "Body")

    def test_unparseable_date_does_not_absorb_the_body(self):
        write_pdf(self.dir / "a.pdf", [[
            "From: <carol@example.com>", "Subject: Hi", "Date: someday", "", "Body"]])
        messages, skipped = load_mailbox(self.dir)
        self.assertEqual((messages, skipped), ([], 1))

    def test_repeated_header_keeps_its_first_value(self):
        write_pdf(self.dir / "a.pdf", [[
            "From: <carol@example.com>", "Subject: First", "Subject: Second",
            "Date: Mon, 06 Jan 2025 23:30:00 -0500", "", "Body"]])
        [m], _ = load_mailbox(self.dir)
        self.assertEqual(m.subject, "First")

    def test_missing_zone_is_utc_and_missing_subject_empty(self):
        write_pdf(self.dir / "a.pdf", [["From: c@example.com", "Date: 07 Jan 2025 09:00:00", "", "x"]])
        [m], _ = load_mailbox(self.dir)
        self.assertEqual(m.date, datetime(2025, 1, 7, 9, 0, tzinfo=timezone.utc))
        self.assertEqual(m.subject, "")

    def test_body_includes_later_pages(self):
        write_pdf(self.dir / "a.pdf", [HEADER + ["first page"], ["second page"]])
        [m], _ = load_mailbox(self.dir)
        self.assertIn("first page", m.body)
        self.assertIn("second page", m.body)
        self.assertNotIn("Budget review", m.body)

    def test_bad_pdfs_are_skipped_and_counted_others_load(self):
        write_pdf(self.dir / "good.pdf", [HEADER + ["hi"]])
        write_pdf(self.dir / "nofrom.pdf", [["Subject: x", "Date: 07 Jan 2025 09:00:00", "", "hi"]])
        write_pdf(self.dir / "nodate.pdf", [["From: a@example.com", "", "hi"]])
        write_pdf(self.dir / "noheader.pdf", [["Just some text"]])
        write_pdf(self.dir / "locked.pdf", [HEADER + ["hi"]], encrypt="secret")
        (self.dir / "garbage.pdf").write_bytes(b"not a pdf at all")
        (self.dir / "empty.pdf").write_bytes(b"")
        messages, skipped = load_mailbox(self.dir)
        self.assertEqual(len(messages), 1)
        self.assertEqual(skipped, 6)

    def test_mixed_mailbox_has_sources_and_deterministic_order(self):
        write_pdf(self.dir / "b.pdf", [HEADER + ["hi"]])
        (self.dir / "a.eml").write_text(
            "From: bob@example.com\nSubject: Hi\nDate: Tue, 07 Jan 2025 08:00:00 +0000\n\nYo\n")
        messages, _ = load_mailbox(self.dir)
        self.assertEqual([m.source for m in messages], ["eml", "pdf"])
        self.assertEqual(messages, load_mailbox(self.dir)[0])

    def test_digest_treats_pdf_like_eml(self):
        (self.dir / "a.eml").write_text(
            "From: owner@example.com\nSubject: Re: Budget review\n"
            "Date: Tue, 07 Jan 2025 10:00:00 +0000\n\nSent.\n")
        write_pdf(self.dir / "b.pdf", [HEADER + ["Can you send the numbers?"]])
        messages, _ = load_mailbox(self.dir)
        d = build_digest(messages, date(2025, 1, 7), "owner@example.com")
        self.assertEqual(d["total"], 2)
        self.assertEqual(len(d["threads"]), 1)
        self.assertFalse(d["threads"][0]["needs_reply"])
        self.assertEqual(d["action_items"], ["Can you send the numbers?"])


if __name__ == "__main__":
    unittest.main()
