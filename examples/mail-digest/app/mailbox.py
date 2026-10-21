import re
from dataclasses import dataclass
from datetime import datetime, timezone
from email import policy
from email.parser import BytesParser
from email.utils import parseaddr, parsedate_to_datetime
from pathlib import Path

from pypdf import PdfReader


@dataclass(frozen=True)
class Message:
    sender: str
    date: datetime
    subject: str
    body: str
    source: str = "eml"


def _parse_date(value):
    try:
        parsed = parsedate_to_datetime(value)
    except (TypeError, ValueError):
        return None
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=timezone.utc)
    return parsed.astimezone(timezone.utc)


def _parse_file(path):
    with open(path, "rb") as f:
        mail = BytesParser(policy=policy.default).parse(f)
    date = _parse_date(mail["Date"])
    if date is None:
        return None
    part = mail.get_body(preferencelist=("plain",))
    body = part.get_content() if part is not None else ""
    return Message(
        sender=parseaddr(str(mail["From"] or ""))[1],
        date=date,
        subject=str(mail["Subject"] or ""),
        body=body,
        source="eml",
    )


_HEADER_LINE = re.compile(r"^(from|to|subject|date)\s*:\s*(.*)$", re.IGNORECASE)


def _split_header_block(first_page):
    headers, last, lines = {}, None, first_page.splitlines()
    i = 0
    while i < len(lines) and not lines[i].strip():
        i += 1
    while i < len(lines):
        line = lines[i]
        match = _HEADER_LINE.match(line)
        if match:
            last = match.group(1).lower()
            headers[last] = match.group(2).strip()
        elif last and line[:1].isspace() and line.strip():
            headers[last] += " " + line.strip()
        else:
            break
        i += 1
    return headers, lines[i:]


def _parse_pdf(path):
    try:
        reader = PdfReader(str(path))
        if reader.is_encrypted:
            return None
        pages = [page.extract_text() or "" for page in reader.pages]
        if not pages:
            return None
        headers, rest = _split_header_block(pages[0])
        sender = parseaddr(headers.get("from", ""))[1]
        date = _parse_date(headers.get("date"))
    except Exception:
        return None
    if not sender or date is None:
        return None
    body = "\n".join(rest + pages[1:]).strip()
    return Message(sender=sender, date=date, subject=headers.get("subject", ""),
                   body=body, source="pdf")


def load_mailbox(directory):
    """Messages from every .eml and .pdf file, plus the count of Skipped Files."""
    messages, skipped = [], 0
    for path in sorted(Path(directory).iterdir()):
        if path.suffix == ".eml":
            message = _parse_file(path)
        elif path.suffix == ".pdf":
            message = _parse_pdf(path)
            skipped += message is None
            if message is None:
                continue
        else:
            continue
        if message is not None:
            messages.append(message)
    return messages, skipped


def load_messages(directory):
    return load_mailbox(directory)[0]
