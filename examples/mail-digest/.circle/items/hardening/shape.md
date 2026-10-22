## Goal
Every non-blocking finding from the daily-digest and pdf-ingest reviews that the product owner accepted is fixed, so the image, the language, the inputs and the page behave as the specs promised.

## Decisions
- Item `hardening`, kind fix — the findings are defects against existing specs, not new features.
- `docker-compose.yml` forwards `OWNER_EMAIL` to `web` and `test`, defaulting to `owner@example.com` (the address the sample mails are sent to) — a fresh `docker compose up` shows a meaningful Needs Reply.
- The `test` service runs the image as built: no bind mounts of `app`, `tests` or `samples` — the gates must prove the image. Circle runs `docker compose build` before each gate, so the image is current.
- The Digest Day is never called "date": JSON key `day`, query parameter `?day=`; `?date=` stays an accepted alias — fixes the DOMAIN.md violation without breaking links. Recorded in `app/DOMAIN.md`.
- Day validation: empty, malformed or unpadded day is 400; the first and last representable days render with no link beyond them.
- A malformed or undecodable `.eml` (or one with an unparseable Date) is a Skipped File, counted like a bad PDF, never a 500.
- An HTML-only email yields its text as body.
- `.EML`/`.PDF` suffixes match case-insensitively.
- Sender addresses are lower-cased once, in the Mailbox — the Owner comparison and ranking then agree.
- Action-item sentence splitting does not split on decimals or on "e.g."/"i.e." (and similar common abbreviations).
- A wrapped PDF header value continues on the next line even if unindented, provided that line is not a header and the body has not started.
- The page gets a viewport meta tag, a small inline stylesheet, a visible EML/PDF badge, and an explicit "No mail on this day" for a quiet day.
- `docs/STANDARDS.md` records the judgement rules (domain terms in code, no blanket `except Exception`, one place for normalisation, no dead loop variables), registered with `circle knowledge add docs/STANDARDS.md --as standards`.
- (assumed) Remove the dead `for svc in web test` loop in `scripts/build_check.sh`, since the rule on dead loop variables would otherwise be broken by the repo's own script.
- (assumed) The blanket `except Exception` in PDF parsing is narrowed to the specific pypdf/parse errors, per the new standard.
- (assumed) A repeated header (two `From:` lines) keeps its first value instead of silently overwriting it — the nit is cheap to fix while the header parser is open.
- (assumed) Unknown paths keep returning the HTML 404; request logging in `log_message` is restored to stderr so errors are visible.

## Behaviours
- `OWNER_EMAIL` set in the environment is the Owner in `web` and `test`; unset, the Owner is `owner@example.com` and the sample Mailbox shows a Needs Reply — proved by `test:integration`
- The `test` service has no bind mounts of `app`, `tests` or `samples` and runs the built image — proved by `test:build`
- Digest JSON carries `day`, not `date`; `?day=` selects the day; `?date=` is an equivalent alias — proved by `test:integration`
- Empty `?day=`, malformed or unpadded days (`2025-3-1`) return 400 — proved by `test:integration`
- `0001-01-01` and `9999-12-31` return 200 with no previous (respectively next) link — proved by `test:integration`
- A malformed or undecodable `.eml` is a Skipped File, counted with bad PDFs, and the request still returns 200 — proved by `test:unit` and `test:integration`
- An `.eml` with an unparseable Date is a counted Skipped File — proved by `test:unit`
- An HTML-only email's body is its text content — proved by `test:unit`
- `MAIL.PDF` and `MAIL.EML` are read like their lower-case forms — proved by `test:unit`
- `A@x.com` and `a@x.com` count as one Sender, listed in lower case — proved by `test:unit`
- "It costs 3.5 dollars. Can you confirm?" and "e.g. this, i.e. that" are not split mid-sentence — proved by `test:unit`
- An unindented wrapped header value joins its header; a non-header line after the Header Block starts the body — proved by `test:unit`
- A repeated header keeps its first value — proved by `test:unit`
- A malformed, empty or quiet day shows "No mail on this day"; the HTML has a viewport meta tag and inline stylesheet; EML/PDF badge visible on each message — proved by `test:integration` for the markup strings; looks readable by eye: manual (visual quality cannot be gated)
- `docs/STANDARDS.md` exists and is registered as `standards` knowledge — proved by `circle knowledge` listing (contracts `knowledge paths` resolves)
- `build_check.sh` has no unused loop variable and the unit, integration, smoke and image gates still pass — proved by `test:build`, `test:smoke`, `test:image`

## Out of scope
- Replacing the digest's dicts with typed dataclasses.
- Threads that span several days.
- `HTTP_PROXY`/`NO_PROXY` support.
- The machine path documented in `docs/DEV-ENVIRONMENT.md`.

## Open
<!-- must be empty to advance -->
