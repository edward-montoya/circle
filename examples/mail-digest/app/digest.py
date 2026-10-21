import re
from collections import Counter

_MARKERS = re.compile(r"^\s*(re|fwd?)\s*:\s*", re.IGNORECASE)
_SENTENCE = re.compile(r"[^.!?]*(?:[.!?]+|$)")
_WEEKDAYS = "monday|tuesday|wednesday|thursday|friday|saturday|sunday"
_MONTHS = "jan|feb|mar|apr|may|jun|jul|aug|sep|oct|nov|dec"
_REQUEST = re.compile(
    r"\?|\bplease\b|\bcan you\b|\bcould you\b|\bdeadline\b"
    rf"|\bby\s+(?:{_WEEKDAYS}|\d{{4}}-\d{{2}}-\d{{2}}|\d{{1,2}}/\d{{1,2}}|\d{{1,2}}\s+(?:{_MONTHS})|(?:{_MONTHS})[a-z]*\s+\d{{1,2}})",
    re.IGNORECASE,
)


def normalise_subject(subject):
    while True:
        stripped = _MARKERS.sub("", subject)
        if stripped == subject:
            return subject.strip()
        subject = stripped


def action_items(body):
    text = " ".join(l for l in body.splitlines() if not l.lstrip().startswith(">"))
    sentences = (s.strip() for s in _SENTENCE.findall(text))
    return [s for s in sentences if s and _REQUEST.search(s)]


def build_digest(messages, day, owner, skipped=0):
    todays = sorted((m for m in messages if m.date.date() == day), key=lambda m: m.date)
    counts = Counter(m.sender for m in todays)
    senders = [{"address": a, "count": c}
               for a, c in sorted(counts.items(), key=lambda kv: (-kv[1], kv[0]))]
    grouped = {}
    for m in todays:
        grouped.setdefault(normalise_subject(m.subject).lower(), []).append(m)
    threads = []
    for group in grouped.values():
        items = [i for m in group for i in action_items(m.body)]
        threads.append({
            "subject": normalise_subject(group[0].subject),
            "messages": [{"sender": m.sender, "date": m.date.isoformat(), "subject": m.subject,
                         "source": m.source}
                         for m in group],
            "needs_reply": group[-1].sender.lower() != owner.lower(),
            "action_items": items,
        })
    return {
        "date": day.isoformat(),
        "total": len(todays),
        "skipped": skipped,
        "senders": senders,
        "threads": threads,
        "action_items": [i for t in threads for i in t["action_items"]],
    }
