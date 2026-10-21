# Mail Digest v1

## Problem
The Owner has a Mailbox full of Messages and no quick way to see, for one Digest Day, how much arrived, who wrote most, which Threads are active, what is being asked of them, and which Threads still Need Reply.

## Solution
A web app reads the Mailbox (a directory of `.eml` files) and shows a deterministic Digest for one Digest Day, as a server-rendered page and as JSON, with links to the previous and next day. It runs in Docker, with a service for the app and a service that runs the tests in the same image. Sample Messages over three consecutive days ship with it.

## Behaviours
1. As the Owner, I want each `.eml` file parsed into a Message with sender, UTC date, subject and body, so that the Digest has something to summarise. A Message without a parseable date is skipped.
2. As the Owner, I want the Digest to count only the Messages of the chosen Digest Day (UTC), so that the total is the day's mail.
3. As the Owner, I want Senders ranked by message count, ties broken alphabetically, so that I see who wrote most.
4. As the Owner, I want Messages grouped into Threads by Normalised Subject (Re:/RE:/Fwd: stripped repeatedly, case-insensitive), so that a reply thread reads as one.
5. As the Owner, I want sentences in a body that ask for something (question mark, "please", "can you", "could you", "deadline", "by <weekday/date>") listed as Action Items, ignoring quoted `>` lines, so that I see what is asked of me.
6. As the Owner, I want a Thread flagged Needs Reply when its last Message is not from the Owner (`OWNER_EMAIL`, case-insensitive), so that I know what I owe an answer to.
7. As a reader, I want `GET /health` to return 200, so that the service can be checked.
8. As a reader, I want `GET /api/digest?date=` to return the Digest as JSON (`date`, `total`, `senders`, `threads`, `action_items`), so that other tools can use it.
9. As a reader, I want `GET /` to render the Digest as HTML for today or `?date=YYYY-MM-DD`, with previous and next day links and all mail text escaped, so that I can read it in a browser.
10. As a reader, I want an invalid date rejected with 400 and a day with no mail shown as an empty Digest, so that bad input is clear and quiet days are not errors.
11. As a developer, I want the app (`web`, port 8080) and the test suite (`test`) to run in the same container image through `docker compose`, so that the gates prove the container.
12. As a developer, I want 9–12 sample Messages over 3 consecutive days in `samples/mail/`, including a reply Thread and several requests needing an answer, so that the app has realistic data to show.
13. As the Owner, I want the HTML page to be pleasant to read — manual, because no browser gate is declared and v1 is plain server-rendered HTML.

## Seams
| Seam | Behaviours | Gate | Misses |
|---|---|---|---|
| Digest seam: parse a directory of `.eml` files, then summarise Messages for a Digest Day and Owner | 1, 2, 3, 4, 5, 6, 12 | `test:unit` | HTTP, rendering, container wiring |
| HTTP seam: server started in-process on a free port, requests to `/health`, `/api/digest`, `/` | 7, 8, 9, 10, 11 | `test:integration` | Real browser rendering, port 8080 mapping from the host |
| Visual quality of the page | 13 | manual (no browser gate declared; plain HTML) | Everything automated |

## Decisions
- Parsing is separate from summarising: a pure function turns parsed Messages, a Digest Day and the Owner into a Digest, so unit tests need no server or disk — it is the highest seam that sees behaviours 2–6.
- The web layer calls the same Digest for both HTML and JSON, so the two views cannot differ.
- The Mailbox is read on every request; nothing is cached or stored.
- Both gates run through `docker compose run --rm test`, so passing them proves the container (behaviour 11); the `test` service is checked by the fact that the gates run in it.
- The image comes from the registry mirror; proxy variables and CA are passed from compose with empty defaults (docs/DEV-ENVIRONMENT.md).
- JSON shape: `senders` entries have `address`, `count`; `threads` entries have `subject`, `messages`, `needs_reply`, `action_items`.

## Out of scope
- Authentication
- IMAP
- Sending mail
- Attachments
- Persistence
- Timezones other than UTC
- Third-party packages

## Risks
- Action Item detection by keyword will have false positives ("by" in ordinary prose); the rule is deliberately simple and deterministic.
- Behaviour 11 is proved indirectly: the gates run in the container, but nothing asserts that `web` actually serves on 8080 from the host.
- Sample dates must be fixed in the past, so tests pass an explicit Digest Day rather than depending on today.
- Relies on the Mail Digest definitions in `app/DOMAIN.md` (Digest Day, Thread, Action Item, Needs Reply).
