import unittest
from datetime import date, datetime, timezone

from app.digest import build_digest
from app.mailbox import Message

DAY = date(2025, 1, 7)
OWNER = "owner@example.com"


def msg(sender="a@example.com", subject="Hi", body="", hour=9, day=7):
    return Message(sender, datetime(2025, 1, day, hour, 0, tzinfo=timezone.utc), subject, body)


class TotalTest(unittest.TestCase):
    def test_counts_only_messages_of_the_digest_day_in_utc(self):
        messages = [
            msg(day=6, hour=23), msg(day=7, hour=0), msg(day=7, hour=23), msg(day=8, hour=0),
        ]
        self.assertEqual(build_digest(messages, DAY, OWNER)["total"], 2)

    def test_day_without_mail_is_an_empty_digest(self):
        d = build_digest([], DAY, OWNER)
        self.assertEqual(
            (d["date"], d["total"], d["senders"], d["threads"], d["action_items"]),
            ("2025-01-07", 0, [], [], []))


class SendersTest(unittest.TestCase):
    def test_ranked_by_count_then_alphabetically(self):
        messages = [msg("zed@x.com"), msg("bob@x.com"), msg("amy@x.com"), msg("amy@x.com"),
                    msg("zed@x.com"), msg("cat@x.com", day=8)]
        self.assertEqual(
            build_digest(messages, DAY, OWNER)["senders"],
            [{"address": "amy@x.com", "count": 2}, {"address": "zed@x.com", "count": 2},
             {"address": "bob@x.com", "count": 1}])


class ThreadsTest(unittest.TestCase):
    def subjects(self, messages):
        return [t["subject"] for t in build_digest(messages, DAY, OWNER)["threads"]]

    def test_groups_by_normalised_subject_stripping_markers_repeatedly(self):
        messages = [msg(subject="Plan", hour=8), msg(subject="RE: Plan", hour=9),
                    msg(subject="re: Fwd: RE: plan", hour=10), msg(subject="Other", hour=11)]
        threads = build_digest(messages, DAY, OWNER)["threads"]
        self.assertEqual([(t["subject"], len(t["messages"])) for t in threads],
                         [("Plan", 3), ("Other", 1)])

    def test_messages_in_a_thread_are_ordered_by_date(self):
        messages = [msg(subject="Re: Plan", hour=10, sender="late@x.com"),
                    msg(subject="Plan", hour=8, sender="early@x.com")]
        [t] = build_digest(messages, DAY, OWNER)["threads"]
        self.assertEqual([m["sender"] for m in t["messages"]], ["early@x.com", "late@x.com"])

    def test_needs_reply_when_last_message_is_not_from_owner(self):
        messages = [msg(subject="A", hour=8, sender="x@x.com"),
                    msg(subject="Re: A", hour=9, sender="Owner@Example.com"),
                    msg(subject="B", hour=8, sender="x@x.com"),
                    msg(subject="Re: B", hour=9, sender="x@x.com"),
                    msg(subject="C", hour=8, sender="x@x.com"),
                    msg(subject="Re: C", hour=9, sender=OWNER),
                    msg(subject="Re: C", hour=10, sender="x@x.com")]
        flags = {t["subject"]: t["needs_reply"]
                 for t in build_digest(messages, DAY, OWNER)["threads"]}
        self.assertEqual(flags, {"A": False, "B": True, "C": True})


class ActionItemsTest(unittest.TestCase):
    def items(self, body):
        return build_digest([msg(body=body)], DAY, OWNER)["action_items"]

    def test_detects_requests_and_ignores_plain_statements_and_quotes(self):
        body = ("Thanks for the update. Can you send the report? It was a good day.\n"
                "Please review the draft. The deadline is soon. Send it by Friday.\n"
                "> Could you quote this?\n"
                "We shipped it.")
        self.assertEqual(self.items(body), [
            "Can you send the report?", "Please review the draft.",
            "The deadline is soon.", "Send it by Friday."])

    def test_detects_by_date_and_question_mark(self):
        self.assertEqual(self.items("Need it by 2025-01-10. Ready"
                                    "?"), ["Need it by 2025-01-10.", "Ready?"])

    def test_threads_carry_their_action_items_and_digest_lists_all(self):
        d = build_digest([msg(subject="A", body="Please help."),
                          msg(subject="B", body="Fine.")], DAY, OWNER)
        self.assertEqual([t["action_items"] for t in d["threads"]], [["Please help."], []])
        self.assertEqual(d["action_items"], ["Please help."])


if __name__ == "__main__":
    unittest.main()
