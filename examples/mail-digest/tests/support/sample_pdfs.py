from tests.support.pdf import write_pdf

SAMPLE_PDFS = {
    "11.pdf": [
        "From: Mo Chen <mo@acme.example>",
        "To: owner@example.com",
        "Subject: Security audit",
        "Date: Mon, 10 Mar 2025 13:30:00 +0000",
        "",
        "Hello,",
        "",
        "The annual security audit starts next week.",
        "Can you confirm which systems are in scope?",
    ],
    "12.pdf": [
        "From: Jo Kim <jo@acme.example>",
        "To: owner@example.com",
        "Subject: Re: Q1 budget review",
        "Date: Tue, 11 Mar 2025 13:00:00 +0000",
        "",
        "Hi,",
        "",
        "I added the travel lines to the Q1 budget.",
        "Please check the totals before Friday.",
    ],
    "13.pdf": [
        "From: Mo Chen <mo@acme.example>",
        "To: owner@example.com",
        "Subject: Security audit schedule",
        "Date: Wed, 12 Mar 2025 15:45:00 +0000",
        "",
        "Hello,",
        "",
        "The audit schedule is attached to the invite.",
    ],
}


def write_sample_pdfs(directory):
    for name, lines in SAMPLE_PDFS.items():
        write_pdf(directory / name, [lines])
