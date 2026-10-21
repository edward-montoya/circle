# Mail Digest

Turns a mailbox into a daily summary: what arrived on one day, from whom, in which conversations, and what the reader still owes an answer to.

## Language

**Mailbox**:
The collection of messages the digest reads, belonging to one owner.
_Avoid_: inbox, account

**Owner**:
The person whose mailbox it is, identified by their email address. Messages from the owner are the owner's own replies.
_Avoid_: user, me

**Message**:
A single email, with a sender, a date, a subject and a body.
_Avoid_: mail, email (as a count noun)

**Digest Day**:
One calendar day in UTC. A message belongs to the Digest Day on which it was sent.
_Avoid_: date range, period

**Digest**:
The deterministic summary of the Mailbox for one Digest Day: total count, ranked senders, threads, and action items.
_Avoid_: report, summary

**Sender**:
The address a message came from, ranked in a Digest by how many messages it sent that day.
_Avoid_: author, from

**Thread**:
The messages of a Digest Day that share a normalised subject, ordered by date.
_Avoid_: conversation

**Normalised Subject**:
A subject with any leading reply or forward markers (Re:, RE:, Fwd:) removed, used to group a Thread.
_Avoid_: topic

**Action Item**:
A sentence in a message body that asks the reader for something: a question, a polite request, a deadline, or a "by <weekday or date>" demand.
_Avoid_: task, todo

**Needs Reply**:
A Thread is in this state when its last message is not from the Owner.
_Avoid_: unanswered, pending

**Source**:
The form a Message arrived in: `eml` (an email file) or `pdf` (an email printed or exported to PDF). Every Message has exactly one.
_Avoid_: format, origin, type

**PDF Email**:
A Message whose Source is `pdf`: a PDF file in the Mailbox whose first page opens with a Header Block.
_Avoid_: PDF message, scan

**Header Block**:
The lines `From:`, `To:`, `Subject:` and `Date:` at the top of a PDF Email's first page, as mail clients print them. Everything after it is the body. A PDF Email needs at least a parseable From and Date.
_Avoid_: metadata, PDF properties

**Skipped File**:
A file in the Mailbox that could not become a Message (a PDF without a parseable From and Date, or not a readable PDF). It is counted and reported, never fatal.
_Avoid_: error, failure

## Relationships

- A **Digest** covers exactly one **Digest Day** of one **Mailbox**
- A **Digest** ranks many **Senders** and groups **Messages** into **Threads**
- A **Thread** has many **Messages** and shares one **Normalised Subject**
- A **Message** has zero or many **Action Items**
- A **Thread** is in **Needs Reply** depending on who sent its last **Message**
- A **Message** has exactly one **Source**
- A **PDF Email** is a **Message** read from a **Header Block** and the text after it
- A **Digest** reports how many **Skipped Files** the **Mailbox** held

## Modules

- `app` — parsing the Mailbox, building the Digest, and serving it over the web

## Flagged ambiguities

- "date" was used for both the day a Digest covers and a message's timestamp. Resolved: the day is the **Digest Day**; the timestamp belongs to the **Message**.
- "email" for a PDF: a PDF Email is a **Message** with Source `pdf`, not a separate kind of thing; the Digest treats all Messages alike whatever their Source.
