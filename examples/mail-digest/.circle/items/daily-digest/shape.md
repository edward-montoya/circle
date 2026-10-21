## Goal
A Dockerised, stdlib-only Python web app reads a directory of `.eml` files and shows a deterministic daily Digest in HTML and JSON.

## Decisions
- Item `daily-digest`, kind feature — settled by the product owner.
- Python 3.12, standard library only — no third-party packages in v1.
- Mail source is a directory of RFC 822 `.eml` files, `MAIL_DIR` default `/data/mail`; 9–12 sample emails over 3 consecutive days (including a reply thread and several requests needing an answer) ship in `samples/mail/` — IMAP is out.
- The Digest is deterministic, no LLM, no network, for one UTC Digest Day: total count; Senders ranked by count; Threads grouped by Normalised Subject (strip Re:/RE:/Fwd:); Action Items are body sentences containing a question mark, "please", "can you", "could you", "deadline" or "by <weekday/date>"; a Thread Needs Reply when its last message is not from `OWNER_EMAIL`.
- Web: `GET /` (HTML, today or `?date=YYYY-MM-DD`, previous/next day links), `GET /api/digest?date=` (same Digest as JSON), `GET /health` (200); server-rendered, no JS framework.
- Everything runs in Docker: compose service `web` (port 8080) and service `test` (same image, runs the suite); gates run through `docker compose`, so they prove the container.
- Image built from `mirror.gcr.io/library/python:3.12-slim`; proxy variables and CA are passed from compose with empty defaults, never hard-coded (docs/DEV-ENVIRONMENT.md). No pip installs are needed, so the proxy matters only for the base pull.
- Tests use `unittest`: unit tests in `tests/unit` at the Digest seam; integration test in `tests/integration` starts the server in-process on a free port.
- Code lives under `app/`, domain language in `app/DOMAIN.md`; gates declared in `.circle/project.toml` as `test:unit` and `test:integration`.
- (assumed) Pure function seam: `build_digest(messages, day, owner)` takes parsed Messages, so unit tests need no filesystem server; parsing is a separate function over a directory.
- (assumed) Message day is taken from the `Date` header converted to UTC; a message with no parseable Date is skipped.
- (assumed) Subject matching for Thread grouping is case-insensitive after stripping markers (repeatedly) and whitespace collapse.
- (assumed) Sender identity is the lowercased address from `From`; ties in ranking break alphabetically for determinism.
- (assumed) Owner comparison is case-insensitive on address; with `OWNER_EMAIL` unset every Thread Needs Reply when non-empty.
- (assumed) An invalid `date` returns 400 on both `/` and `/api/digest`; a day with no mail renders an empty Digest, not an error.
- (assumed) Action Item sentences split on `.`, `!`, `?` and newlines; quoted reply lines (starting with `>`) are ignored so a reply does not repeat earlier requests.
- (assumed) The HTML escapes all mail-derived text; the server reads the mail directory on each request (no caching, no persistence).
- (assumed) JSON field names: `date`, `total`, `senders` (`address`, `count`), `threads` (`subject`, `messages`, `needs_reply`, `action_items`), `action_items`.

## Behaviours
- Parsing a `.eml` file yields sender, UTC date, subject and body — proved by `test:unit`
- A Digest counts messages for the Digest Day only and ranks Senders by count — proved by `test:unit`
- Subjects with Re:/RE:/Fwd: group into one Thread — proved by `test:unit`
- Sentences asking for something become Action Items — proved by `test:unit`
- A Thread whose last message is not from the Owner Needs Reply — proved by `test:unit`
- `/health` returns 200, `/api/digest?date=` returns the Digest JSON, `/` returns HTML with previous/next day links — proved by `test:integration`
- An invalid date is rejected with 400 — proved by `test:integration`
- The app and the tests run in the container, `web` on port 8080 — proved by `test:unit` and `test:integration` via `docker compose run --rm test`
- Sample mail spans 3 consecutive days and contains a reply thread — proved by `test:unit` (fixture-based assertions on `samples/mail/`)
- Visual quality of the HTML page — manual (no browser gate declared; v1 page is plain server-rendered HTML)

## Out of scope
- Authentication
- IMAP
- Sending mail
- Attachments
- Persistence
- Timezones other than UTC
- Third-party packages

## Open
