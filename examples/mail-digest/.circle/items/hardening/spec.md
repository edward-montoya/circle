# Hardening: fix non-blocking review findings

## Problem
The reviews of the daily-digest and pdf-ingest items accepted the work but left defects. The Owner cannot be set in the containers, and the gates run host files instead of the built image. The Digest calls its Digest Day "date". Bad input breaks the page: an empty or unpadded day is not rejected, the first and last representable days fail, and one malformed email file makes every request fail. Senders differing in case are counted twice, HTML-only Messages have no body, `.PDF`/`.EML` files are ignored uncounted, Action Items are cut at decimals and abbreviations, wrapped unindented PDF header values spill into the body, and the page is bare HTML with no sign of a quiet day. The judgement rules the reviews used are written nowhere.

## Solution
The Owner is set from `OWNER_EMAIL` in both services, defaulting to the address the sample Mailbox writes to. The `test` service runs the built image alone. The Digest and its web interface speak of the Digest Day as `day` (`?date=` still accepted). Invalid days are rejected, the edge days render, and any unreadable email file becomes a counted Skipped File. Message reading and Action Item detection are made robust as listed below. The page is readable and says when a day is quiet. The judgement rules are recorded as registered standards.

## Behaviours
1. As an operator, I want `OWNER_EMAIL` forwarded to `web` and `test` with the default `owner@example.com`, so that a fresh `docker compose up` shows a Needs Reply for the sample Mailbox.
2. As a maintainer, I want the `test` service to run the image as built, with no bind mounts of app, tests or samples, so that the gates prove the image.
3. As a reader of the Digest, I want the Digest Day under the key `day` and selected by `?day=`, with `?date=` as an equivalent alias, so that the language is consistent and old links work.
4. As a reader, I want an empty, malformed or unpadded day (e.g. `2025-3-1`) to be rejected with 400, so that I am never silently shown today.
5. As a reader, I want the first (`0001-01-01`) and last (`9999-12-31`) representable days to render with no previous link before the first and no next link after the last, so that they do not fail.
6. As a reader, I want a malformed or undecodable `.eml` to be a Skipped File counted like a bad PDF, so that one bad file never fails a request.
7. As a reader, I want an `.eml` with an unparseable Date to be a counted Skipped File, so that no file is dropped uncounted.
8. As a reader, I want an HTML-only Message to yield its text as body, so that its Action Items are found.
9. As a reader, I want `.EML` and `.PDF` suffixes read like `.eml` and `.pdf`, so that file case does not hide Messages.
10. As a reader, I want Senders normalised to lower case once, in the Mailbox, so that `A@x.com` and `a@x.com` rank as one Sender and match the Owner comparison.
11. As a reader, I want Action Item sentences not split at decimals ("3.5") or abbreviations ("e.g.", "i.e."), so that requests stay whole.
12. As a reader, I want a PDF Email header value that wraps onto an unindented line to continue its header when that line is not a header and the body has not started, so that long Subjects are intact.
13. As a reader, I want a repeated header to keep its first value, so that a later line does not overwrite it silently.
14. As a reader, I want the page to carry a viewport meta tag, a small inline stylesheet, a visible EML/PDF badge on each Message, and "No mail on this day" for a quiet day, so that it is readable on any screen.
15. As a reviewer, I want the page to look readable by eye, which no gate can judge.
16. As a maintainer, I want `docs/STANDARDS.md` registered as `standards` knowledge, so that future reviews have registered standards.
17. As a maintainer, I want `scripts/build_check.sh` free of its unused loop variable and the existing gates still passing, so that the repo follows its own standards.
18. As an operator, I want request errors logged, and unknown paths still answered with the HTML 404, so that failures in the running service are visible.

## Seams
| Seam | Behaviours | Gate | Misses |
|---|---|---|---|
| Web interface (`/`, `/health`, JSON) driven over HTTP against a sample Mailbox | 1, 3, 4, 5, 6, 14 | `test:integration` | Visual quality; `OWNER_EMAIL` in a real running compose stack |
| Mailbox reading and Digest building (`load_mailbox`, `build_digest`, header and body parsing) | 6, 7, 8, 9, 10, 11, 12, 13 | `test:unit` | Wiring through HTTP |
| Compose and build configuration (`docker compose config`, `build_check.sh`) | 1, 2, 17 | `test:build` | Runtime behaviour of the built containers |
| The built images | 2, 17 | `test:image` | Page content |
| The running web service | 18 | `test:smoke` | Log content of every error path |
| Knowledge registration (`circle knowledge` listing; contract `knowledge paths`) | 16 | `test:build` | Quality of the standards text (reviewed by reading) |
| Reading the page by eye | 15 | manual — visual readability cannot be gated | everything else |

## Decisions
- The Mailbox is the one place that normalises Sender case, and the one place that counts Skipped Files for every Source; the Digest and web interface trust it.
- Day parsing is strict `YYYY-MM-DD` and returns a typed failure the web interface turns into 400, replacing the `None` sentinel; `day` is read before the `date` alias when both are given.
- Neighbouring days are computed so that the first and last days simply have no link.
- PDF parsing catches only the specific reading and parsing errors; other errors surface.
- The `test` service default command and project gate commands stay as declared in the contracts; the only compose change is dropping bind mounts and forwarding `OWNER_EMAIL`.
- Vocabulary follows `app/DOMAIN.md` (Digest Day, Skipped File, Source, Header Block, Sender).

## Out of scope
- Replacing the digest's dicts with typed dataclasses.
- Threads that span several days.
- `HTTP_PROXY`/`NO_PROXY` support.
- The machine path documented in `docs/DEV-ENVIRONMENT.md`.

## Risks
- Without bind mounts, tests and samples must be baked into the `test` image; a stale image would test old code, mitigated by Circle's bootstrap build before each gate.
- Strict day parsing may reject links that formerly worked (unpadded days); accepted by the product owner.
- Abbreviation handling is a list, not a language model; other abbreviations may still split.
- The knowledge registration (behaviour 16) is proved only by configuration resolving, not by the text's content.
