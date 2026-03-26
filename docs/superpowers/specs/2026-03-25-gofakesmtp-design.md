# gofakesmtp Design Spec

**Date:** 2026-03-25
**Status:** Approved

## Overview

`gofakesmtp` is a Go CLI tool that runs a local fake SMTP server for development and testing. It intercepts emails sent to a local port, displays them in a terminal UI, and optionally saves them to disk. It is a Go equivalent of [FakeSMTP](https://nilhcem.com/FakeSMTP/).

---

## Architecture

A single binary composed of three internal packages wired together in `main.go`.

```
main.go
├── smtp/       SMTP server backend (emersion/go-smtp)
├── tui/        Bubbletea terminal UI
└── storage/    .eml file writer
```

`main.go` creates a buffered `chan Email` (size 100), starts the SMTP server in a goroutine, and passes the channel to the TUI. It then calls `tea.NewProgram().Run()` which blocks until the user quits.

---

## Packages

### `smtp`

Wraps `emersion/go-smtp`. Implements the `Backend` and `Session` interfaces.

- On `Session.Data()` completion, parses the raw message with `net/mail` into an `Email` struct and sends it onto the channel.
- STARTTLS is enabled when a TLS config is provided (cert + key files). Without them, the server runs plaintext.
- Returns an error on any SMTP protocol violation; `go-smtp` handles the response to the client.

**Email struct:**
```go
type Email struct {
    From      string
    To        []string
    Subject   string
    Body      string
    Raw       []byte
    ReceivedAt time.Time
}
```

### `tui`

Bubbletea application rendering a horizontal split layout:

- **Left pane:** scrollable list of received emails (from, subject, time). Selected item highlighted.
- **Right pane:** preview of the selected email (subject, from, to, body).
- **Status bar:** server address/port, email count, error messages, STARTTLS indicator.

**Key bindings:**
- `↑` / `↓` or `j` / `k` — navigate list
- `d` — delete selected email from list
- `s` — not shown when `--output-dir` is not set (emails are memory-only)
- `q` / `ctrl+c` — quit

The TUI model uses a `waitForEmail()` `tea.Cmd` that blocks on a channel read and returns the email as a `tea.Msg`. On receipt, `Update()` appends it to the email slice and re-issues `waitForEmail()`.

If `--output-dir` is set, `Update()` calls `storage.Save()` automatically on each new email (auto-save mode). The `s` key binding is not shown since emails are already saved on arrival.

### `storage`

Single exported function:

```go
func Save(email smtp.Email, dir string) error
```

Writes the raw email bytes to `<dir>/<RFC3339-timestamp>-<from>.eml` (e.g. `2026-03-25T14-32-00Z-from@example.com.eml`). Colons in the timestamp are replaced with hyphens for filesystem compatibility. Creates the directory if it does not exist. Returns an error on any filesystem failure (surfaced as a TUI status bar message, not a crash).

---

## CLI Flags

```
gofakesmtp [flags]

  -p, --port        SMTP port to listen on (default: 2525)
  -a, --addr        Bind address (default: 127.0.0.1)
  -o, --output-dir  Directory to auto-save received emails as .eml files
      --tls-cert    Path to TLS certificate file (enables STARTTLS)
      --tls-key     Path to TLS private key file (enables STARTTLS)
```

STARTTLS is only active when both `--tls-cert` and `--tls-key` are provided. Providing only one is a fatal startup error with a clear message.

---

## Data Flow

1. A client connects and sends an email over SMTP.
2. `Session.Data()` is called with the raw message reader.
3. The session parses it with `net/mail` into an `Email` struct (from, to, subject, body, raw bytes, timestamp).
4. The email is sent onto the buffered `chan Email`.
5. The TUI's `waitForEmail()` cmd receives it and returns it as a `tea.Msg`.
6. `Update()` appends it to the model's email slice.
7. If auto-save is enabled, `storage.Save()` is called immediately.
8. `waitForEmail()` is re-issued to keep listening for the next email.

---

## Error Handling

- **SMTP protocol errors:** handled by `go-smtp` internally.
- **TLS config errors** (missing/invalid cert or key): fatal at startup with a descriptive error message.
- **Storage errors** (bad dir, disk full): surfaced as a transient message in the TUI status bar. Does not crash.
- **Channel full:** not a realistic concern at buffer size 100 for a dev tool. If it occurs, the SMTP session blocks briefly then times out normally.

---

## Testing

| Package   | Approach |
|-----------|----------|
| `smtp`    | Start a real listener on a random port. Send messages via `net/smtp`. Assert emails arrive on the channel with correct fields. |
| `storage` | Write emails to a `t.TempDir()`. Assert file exists, content matches raw bytes, filename format is correct. |
| `tui`     | Bubbletea models are pure functions. Call `Update()` directly with `tea.Msg` values. Assert resulting model state (email list length, selected index, status bar text). |

---

## Dependencies

| Dependency | Purpose |
|------------|---------|
| `github.com/emersion/go-smtp` | SMTP server implementation |
| `github.com/charmbracelet/bubbletea` | TUI framework |
| `github.com/charmbracelet/lipgloss` | TUI styling |
| `github.com/spf13/cobra` | CLI flag parsing |
