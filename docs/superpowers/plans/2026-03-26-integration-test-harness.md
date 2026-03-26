# Integration Test Harness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `//go:build integration`-tagged test file that sends real SMTP email to a live in-process server and asserts all fields are received correctly.

**Architecture:** A single new file `internal/smtp/integration_test.go` in `package smtp_test` reuses the existing `startTestServer` helper from `backend_test.go`. A new `test-integration` Makefile target runs both unit and integration tests together.

**Tech Stack:** Go stdlib `net/smtp`, `github.com/emersion/go-smtp`, existing `internal/smtp` package.

---

## File Map

| File | Change |
|------|--------|
| `internal/smtp/integration_test.go` | Create: integration test with build tag |
| `Makefile` | Modify: add `test-integration` target and update `.PHONY` |

---

## Task 1: Integration test file

**Files:**
- Create: `internal/smtp/integration_test.go`

- [ ] **Step 1: Confirm the existing unit tests still pass (baseline)**

```bash
go test ./internal/smtp/... -v
```

Expected:
```
--- PASS: TestBackend_ReceivesEmail
--- PASS: TestBackend_MultipleRecipients
PASS
```

- [ ] **Step 2: Verify the integration test is invisible without the build tag**

Create `internal/smtp/integration_test.go` with just the build tag and package declaration — no test functions yet:

```go
//go:build integration

package smtp_test
```

Run without the tag — confirms the file is ignored:

```bash
go test ./internal/smtp/... -v
```

Expected: same two PASS lines as above, no mention of integration.

Run with the tag — confirms the file compiles:

```bash
go test -tags integration ./internal/smtp/... -v
```

Expected: same two PASS lines (no new tests yet, just confirms the tagged file compiles cleanly alongside the untagged files).

- [ ] **Step 3: Add the integration test**

Replace the contents of `internal/smtp/integration_test.go` with:

```go
//go:build integration

package smtp_test

import (
	"net/smtp"
	"strings"
	"testing"
	"time"

	oursmtp "github.com/jonmilley/gofakesmtp/internal/smtp"
)

func TestIntegration_SendAndReceive(t *testing.T) {
	ch := make(chan oursmtp.Email, 10)
	addr := startTestServer(t, ch)

	body := "Line one of the integration test body.\r\nLine two."
	msg := strings.Join([]string{
		"From: integration@example.com",
		"To: recipient@example.com",
		"Subject: Integration test",
		"",
		body,
	}, "\r\n")

	if err := smtp.SendMail(addr, nil, "integration@example.com", []string{"recipient@example.com"}, []byte(msg)); err != nil {
		t.Fatalf("SendMail failed: %v", err)
	}

	select {
	case email := <-ch:
		if email.From != "integration@example.com" {
			t.Errorf("From: got %q, want %q", email.From, "integration@example.com")
		}
		if len(email.To) != 1 || email.To[0] != "recipient@example.com" {
			t.Errorf("To: got %v, want [recipient@example.com]", email.To)
		}
		if email.Subject != "Integration test" {
			t.Errorf("Subject: got %q, want %q", email.Subject, "Integration test")
		}
		wantBody := "Line one of the integration test body.\r\nLine two."
		if email.Body != wantBody {
			t.Errorf("Body: got %q, want %q", email.Body, wantBody)
		}
		if email.ReceivedAt.IsZero() {
			t.Error("ReceivedAt should not be zero")
		}
		if len(email.Raw) == 0 {
			t.Error("Raw should not be empty")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for email")
	}
}
```

- [ ] **Step 4: Run the integration test and confirm it passes**

```bash
go test -tags integration -v ./internal/smtp/...
```

Expected:
```
--- PASS: TestBackend_ReceivesEmail
--- PASS: TestBackend_MultipleRecipients
--- PASS: TestIntegration_SendAndReceive
PASS
```

- [ ] **Step 5: Confirm unit tests are unaffected**

```bash
go test ./internal/smtp/... -v
```

Expected: only `TestBackend_ReceivesEmail` and `TestBackend_MultipleRecipients` — no `TestIntegration_SendAndReceive`.

- [ ] **Step 6: Commit**

```bash
git add internal/smtp/integration_test.go
git commit -m "test: add integration test for SMTP send/receive"
```

---

## Task 2: Makefile target

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Add the `test-integration` target**

Open `Makefile`. The current contents are:

```makefile
BINARY := gofakesmtp
LDFLAGS := -ldflags="-s -w"

.PHONY: build build-small clean test

build:
	go build $(LDFLAGS) -o $(BINARY) .

# Further compress the binary with UPX (~60% smaller).
# Requires: brew install upx
build-small: build
	upx --best --lzma $(BINARY)

clean:
	rm -f $(BINARY)

test:
	go test ./...
```

Replace with:

```makefile
BINARY := gofakesmtp
LDFLAGS := -ldflags="-s -w"

.PHONY: build build-small clean test test-integration

build:
	go build $(LDFLAGS) -o $(BINARY) .

# Further compress the binary with UPX (~60% smaller).
# Requires: brew install upx
build-small: build
	upx --best --lzma $(BINARY)

clean:
	rm -f $(BINARY)

test:
	go test ./...

test-integration:
	go test -tags integration -v ./...
```

- [ ] **Step 2: Verify both targets work**

```bash
make test
```

Expected: all unit tests pass, no integration tests run.

```bash
make test-integration
```

Expected: unit tests + `TestIntegration_SendAndReceive` all pass.

- [ ] **Step 3: Commit**

```bash
git add Makefile
git commit -m "chore: add test-integration Makefile target"
```

---

## Task 3: Push

- [ ] **Step 1: Push to remote**

```bash
git push
```

Expected: commits pushed to `origin/main`.
