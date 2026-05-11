#!/usr/bin/env python3
"""Send a test email to a locally-running gofakesmtp server."""

import argparse
import smtplib
import sys
from email.message import EmailMessage


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=2525)
    parser.add_argument("--from", dest="sender", default="sender@example.com")
    parser.add_argument("--to", default="recipient@example.com")
    parser.add_argument("--subject", default="Test email from send-test-email.py")
    parser.add_argument(
        "--body",
        default="Hello from the test script!\n\nThis is a plain-text test message.\n",
    )
    args = parser.parse_args()

    msg = EmailMessage()
    msg["From"] = args.sender
    msg["To"] = args.to
    msg["Subject"] = args.subject
    msg.set_content(args.body)

    try:
        with smtplib.SMTP(args.host, args.port, timeout=5) as s:
            s.send_message(msg)
    except (OSError, smtplib.SMTPException) as e:
        print(f"failed to send: {e}", file=sys.stderr)
        return 1

    print(f"sent test email to {args.host}:{args.port} ({args.sender} -> {args.to})")
    return 0


if __name__ == "__main__":
    sys.exit(main())
