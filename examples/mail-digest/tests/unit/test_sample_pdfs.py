import tempfile
import unittest
from datetime import date
from pathlib import Path

from app.digest import build_digest
from app.mailbox import load_mailbox
from tests.support.sample_pdfs import write_sample_pdfs

SAMPLES = Path("samples/mail")
OWNER = "owner@example.com"


class SamplePdfTest(unittest.TestCase):
    def test_committed_pdfs_are_reproduced_byte_for_byte(self):
        with tempfile.TemporaryDirectory() as tmp:
            write_sample_pdfs(Path(tmp))
            names = sorted(p.name for p in Path(tmp).glob("*.pdf"))
            self.assertEqual(len(names), 3)
            for name in names:
                self.assertEqual((SAMPLES / name).read_bytes(), (Path(tmp) / name).read_bytes(), name)

    def test_mailbox_loads_all_samples_with_nothing_skipped(self):
        messages, skipped = load_mailbox(SAMPLES)
        self.assertEqual(skipped, 0)
        self.assertEqual(sorted(m.source for m in messages).count("pdf"), 3)

    def test_digest_totals_on_the_three_days(self):
        messages, _ = load_mailbox(SAMPLES)
        totals = {d: build_digest(messages, date(2025, 3, d), OWNER)["total"] for d in (10, 11, 12)}
        self.assertEqual(totals, {10: 5, 11: 5, 12: 3})

    def test_one_pdf_replies_in_an_existing_thread(self):
        messages, _ = load_mailbox(SAMPLES)
        d = build_digest(messages, date(2025, 3, 11), OWNER)
        [t] = [t for t in d["threads"] if t["subject"] == "Q1 budget review"]
        self.assertEqual(len(t["messages"]), 2)
        self.assertIn("jo@acme.example", [m["sender"] for m in t["messages"]])


if __name__ == "__main__":
    unittest.main()
