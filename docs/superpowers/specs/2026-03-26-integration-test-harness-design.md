# Integration Test Harness Design Spec

**Date:** 2026-03-26
**Status:** Approved

## Overview

Add a `//go:build integration`-tagged test file to `internal/smtp/` that tests the running gofakesmtp SMTP server end-to-end. Normal `go test ./...` stays fast; integration tests run explicitly via `go test -tags integration ./...`.

---

## Architecture

A single new file `internal/smtp/integration_test.go` in package `smtp_test`. This shares the existing `startTestServer` helper from `backend_test.go` with no duplication.

```
internal/smtp/
├── email.go
├── backend.go
├── backend_test.go          (existing unit tests + startTestServer helper)
└── integration_test.go      (new — build tag: integration)
```

The Makefile gets a new `test-integration` target.

---

## Test

### `TestIntegration_SendAndReceive`

1. Start a real SMTP listener on a random port via `startTestServer`.
2. Send an email via `net/smtp.SendMail` with:
   - From: `integration@example.com`
   - To: `recipient@example.com`
   - Subject: `Integration test`
   - Body: multi-line text
3. Receive from channel with a 5-second timeout.
4. Assert:
   - `email.From == "integration@example.com"`
   - `len(email.To) == 1 && email.To[0] == "recipient@example.com"`
   - `email.Subject == "Integration test"`
   - `email.Body == <exact body text>`
   - `!email.ReceivedAt.IsZero()`
   - `len(email.Raw) > 0`

---

## Makefile

Add target alongside existing `build`, `test`, `clean`:

```makefile
test-integration:
	go test -tags integration -v ./...
```

---

## Running

```bash
# Unit tests only (fast)
go test ./...

# Unit + integration tests
make test-integration
# or: go test -tags integration -v ./...
```
