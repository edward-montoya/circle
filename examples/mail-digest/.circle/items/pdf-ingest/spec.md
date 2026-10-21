# PDF ingest

## Problem
Some mail reaches the Mailbox as a PDF: an email printed or exported to PDF. Mail Digest only reads email files, so that mail is missing from the Digest, and the Owner cannot tell from a Digest which Messages came from which kind of file.

## Solution
Each `*.pdf` file in the Mailbox is read as a PDF Email: the Header Block on its first page gives sender, date and subject, and the rest is the body. The resulting Message joins the `.eml` Messages, and the Digest is computed exactly as before. Every Message records its Source (`eml` or `pdf`), shown in the JSON and as a small badge on the page. A PDF that cannot become a Message is a Skipped File: counted and reported, never fatal. Sample PDF Emails ship on the same three days as the `.eml` samples, and the image installs the one new dependency, `pypdf`, both on a normal machine and behind the proxy of this one.

## Behaviours
1. As the Owner, I want a `.pdf` file whose first page has a Header Block with a parseable From and Date turned into a Message with sender address, UTC date, subject and body, so that mail that arrived as PDF is in my Digest.
2. As the Owner, I want Header Block names matched case-insensitively, wrapped header values joined and `Name <addr>` reduced to the address, so that the way different mail clients print headers does not matter.
3. As the Owner, I want the body to be the text after the Header Block, including text on later pages, so that nothing of the mail is lost.
4. As the Owner, I want a PDF without a parseable From and Date, a malformed PDF and an encrypted PDF skipped and counted without raising, with the other files still loaded, so that one bad file never breaks my Digest.
5. As the Owner, I want `.eml` and `.pdf` files in one Mailbox to load in a deterministic order, each Message with its Source, so that I can tell where it came from.
6. As the Owner, I want a PDF Email counted, ranked as a Sender, grouped into Threads (a PDF reply joining an `.eml` Thread and the reverse) and mined for Action Items exactly like an `.eml` Message, so that the Source does not change the Digest.
7. As a reader, I want `GET /api/digest?date=` to show the PDF Email with `"source": "pdf"` (and `.eml` Messages with `"source": "eml"`) and a `skipped` count of Skipped Files, so that other tools see the Source and the loss.
8. As a reader, I want `GET /` to show a small `PDF` or `EML` badge per Message, a skipped line only when the count is above zero, and all PDF text escaped, so that I see the Source in the browser safely.
9. As the Owner, I want three sample PDF Emails on the same three days as the `.eml` samples, one a reply in an existing Thread, so that the app has realistic PDF data.
10. As a developer, I want the sample PDFs reproducible from a committed script, byte for byte, so that they can be regenerated and reviewed.
11. As a developer, I want the image to install `pypdf` (and the test-only PDF writer) at build, so that the app and the tests run in the container.
12. As a developer, I want that build to work with and without the proxy of docs/DEV-ENVIRONMENT.md, with nothing machine-specific in committed files, so that it is portable.
13. As the Owner, I want the badge and skipped line to look right on the page — manual.

## Seams
| Seam | Behaviours | Gate | Misses |
|---|---|---|---|
| Mailbox seam: a directory of real `.eml`/`.pdf` files loaded into Messages and a Skipped File count, then summarised by the Digest | 1, 2, 3, 4, 5, 6, 9, 10 | `test:unit` | HTTP, rendering, container wiring |
| HTTP seam: server in-process on a directory with real PDFs (one malformed), requests to `/api/digest` and `/` | 7, 8 | `test:integration` | Real browser rendering |
| Image seam: the image builds and imports `pypdf` and the PDF writer | 11 | `test:image` | Behaviour of the proxy path; whether versions are the pinned ones |
| Proxy build: build with proxy variables and CA supplied from the environment, and without; read Dockerfile and compose for hard-coded values | 12 | manual (only this machine has a proxy to exercise; a negative "nothing hard-coded" is a reading) | Everything automated |
| Visual quality of the badge and line | 13 | manual (no browser gate declared) | Everything automated |

## Decisions
- Extends `daily-digest`: the PDF reader sits beside the `.eml` reader inside Mailbox loading; the Digest rules are untouched, so behaviour 6 is proved by feeding it mixed Messages.
- A Message gains a Source (`eml` | `pdf`). Loading returns Messages plus the Skipped File count from one pass; the web layer still reads the Mailbox once per request.
- Header Block: `From:`, `To:`, `Subject:`, `Date:` lines at the top of page one, ended by the first blank or non-header line; continuation lines fold into the previous header; names case-insensitive; headers sought on the first page only. Missing `Subject:` gives an empty subject; missing `To:` is fine; a missing zone is UTC.
- Skipped Files are PDFs only (assumed in shape); `.eml` files without a date keep skipping silently as in v1. `skipped` is a whole-Mailbox integer, additive in the JSON; each message entry gains `source`; existing keys unchanged.
- The page shows an escaped `PDF`/`EML` badge per Message and one quiet skipped line only when the count is above zero.
- `pypdf` pinned in `requirements.txt`, the test-only PDF writer `reportlab` pinned in `requirements-test.txt`, both installed in the one image serving `web` and `test`. Proxy and CA come into the build as empty-default arguments from compose; nothing machine-specific is committed.
- Sample PDFs are produced by a committed script with fixed metadata and invariant output, and committed in `samples/mail/`; the unit suite regenerates and compares them. The same helper writes test fixtures to temp directories; a malformed PDF is garbage bytes in a `.pdf` file.
- New gate `test:image` (`docker compose build` then import of both libraries in the test service) proves behaviour 11, since `test:unit` alone would only fail obscurely.
- Digest-day totals in the daily-digest tests change when PDF samples join the directory; this item updates those expectations.
- Language is in `app/DOMAIN.md` (Source, PDF Email, Header Block, Skipped File).

## Out of scope
- OCR of scanned PDFs (no text means no Header Block, a Skipped File)
- Attachments inside `.eml` files
- Encrypted PDFs (counted as Skipped Files, never decrypted)
- PDFs with headers in tables, columns or images
- Rendering the PDF itself in the page
- Reading headers from PDF document properties
- Time zone names RFC 2822 does not carry
- Any third-party dependency beyond `pypdf` and `reportlab`
- Changes to Digest rules, ranking, Threads or Action Items

## Risks
- Text extraction order and line breaks differ by PDF producer; the Header Block rule is tuned to PDFs the sample script writes and to common client output, and real exports may fall through to Skipped Files.
- `test:image` proves the install, not the pinned versions or the proxy path; behaviour 12 rests on a manual check on this machine.
- Depends on the daily-digest worktree landing first (compose file, loader, tests); its names may shift.
- Relies on the Mail Digest definitions in `app/DOMAIN.md` (Message, Digest Day, Thread, Action Item, Needs Reply, Source, PDF Email, Header Block, Skipped File).
- reportlab sits in the runtime image though only tests and the sample script need it — accepted for one image.
