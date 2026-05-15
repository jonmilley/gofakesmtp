# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

- `make build` — produces the `gofakesmtp` binary with `-s -w` linker flags.
- `make test` — runs unit tests (`go test ./...`).
- `make test-integration` — runs the integration suite, gated behind `//go:build integration`.
- `go test ./internal/smtp -run TestBackend_AuthRequired_AcceptsValidCredentials` — pattern for running a single test.
- `go vet ./...` — run before declaring work done; CI is not configured, so `go build`, `go vet`, and `go test ./...` are the standard local gate.

## Architecture

The runtime is a single process that bridges an SMTP server and a Bubbletea TUI through a buffered channel. `main.go` wires it together:

1. `chan oursmtp.Email` (buffer 100) is created.
2. `internal/smtp.NewBackend(ch, Config{...})` is wrapped in `gosmtp.NewServer` and started in a goroutine.
3. `internal/tui.New(ch, outputDir, ...)` is run as the foreground Bubbletea program.
4. When the TUI exits, `smtpServer.Close()` is called.

The TUI's `Init` returns a `tea.Cmd` that blocks reading from `ch`; on each `EmailReceivedMsg` it re-arms itself, optionally calls `internal/storage.Save`, and updates the model. Storage failures surface as a `statusMessage` in the bottom bar rather than aborting.

The `Email` value carries both parsed fields and the original `Raw` bytes, because `storage.Save` writes the original `.eml` to disk and downstream consumers may want either form.

### Backend configuration (auth + session log)

`internal/smtp.Backend` is configured via `Config`. The auth modes are mutually exclusive and chosen by `main.go` flag validation:

| Flags | Mode | `AuthMechanisms()` | Gate on `MAIL`/`RCPT`/`DATA` |
|-------|------|--------------------|------------------------------|
| (none) | open | `nil` (AUTH not advertised) | none |
| `--smtp-user U --smtp-pass P` | strict | `[PLAIN]` | `ErrAuthRequired` until creds match |
| `--smtp-auth-any` | permissive | `[PLAIN]` | none; any creds accepted |

`go-smtp` v0.24 picks up auth via the `AuthSession` interface (`AuthMechanisms` + `Auth`), not the older `AuthPlain` method. Returning `nil` from `AuthMechanisms` causes `go-smtp` to omit the `AUTH` capability from the EHLO response entirely. Auth failures must return `gosmtp.ErrAuthFailed` (or another `*SMTPError`); a plain `errors.New(...)` gets coerced to a generic `454`, not `535`.

The `--session-log` writer is plumbed two ways for a single file: `Server.Debug` receives the wire bytes (via `go-smtp`'s built-in `TeeReader`/`MultiWriter`), and `Backend.cfg.SessionLog` receives `--- <ts> connect <addr> ---` / `disconnect` markers from `NewSession`/`Logout`. There is no per-connection synchronization, so concurrent connections can interleave at byte boundaries; this is acceptable for the intended use as a developer test server.

Watch for the typed-nil-interface gotcha when constructing a `Config`: assign the file to a local `var w io.Writer` and pass `w`. Passing a typed nil pointer (`*os.File(nil)`) yields a non-nil `io.Writer`, and the marker writes will panic.

### Test layout

`internal/smtp/backend_test.go` shares a `startTestServerOpts` helper that listens on `127.0.0.1:0` and spins up a real `go-smtp` server against the backend. Tests use `net/smtp` as the client. The `syncBuffer` type wraps `bytes.Buffer` with a mutex because `Server.Debug` is written from connection goroutines. `internal/smtp/integration_test.go` is build-tagged `integration`.
