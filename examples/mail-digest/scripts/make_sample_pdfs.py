"""Regenerate the sample PDF Emails in samples/mail/ (optionally into the directory given as argument)."""
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from tests.support.sample_pdfs import write_sample_pdfs  # noqa: E402

if __name__ == "__main__":
    write_sample_pdfs(Path(sys.argv[1] if len(sys.argv) > 1 else "samples/mail"))
