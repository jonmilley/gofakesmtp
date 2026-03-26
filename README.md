# gofakesmtp

A fake SMTP server with a terminal UI for testing email in applications. A Go equivalent of [FakeSMTP](https://nilhcem.com/FakeSMTP/).

Run it locally, point your app at `localhost:2525`, and watch emails appear in real time — no mail server, no credentials, nothing sent.

![Terminal UI showing a split-pane layout with an email list on the left and a preview on the right]()

## Features

- Intercepts all SMTP email sent to a local port
- Terminal UI with split-pane layout: email list + message preview
- Keyboard navigation
- Optional auto-save to `.eml` files on disk
- STARTTLS support for apps that require TLS

## Install

```bash
go install github.com/jonmilley/gofakesmtp@latest
```

Or build from source:

```bash
git clone https://github.com/jonmilley/gofakesmtp
cd gofakesmtp
go build -o gofakesmtp .
```

## Usage

```bash
gofakesmtp [flags]
```

Start with defaults (listens on `127.0.0.1:2525`):

```bash
gofakesmtp
```

Custom port:

```bash
gofakesmtp --port 1025
```

Save all received emails to disk as `.eml` files:

```bash
gofakesmtp --output-dir ./received-emails
```

Enable STARTTLS (requires a TLS certificate and key):

```bash
gofakesmtp --tls-cert cert.pem --tls-key key.pem
```

### All flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--port` | `-p` | `2525` | SMTP port to listen on |
| `--addr` | `-a` | `127.0.0.1` | Address to bind to |
| `--output-dir` | `-o` | _(none)_ | Directory to auto-save emails as `.eml` files |
| `--tls-cert` | | _(none)_ | TLS certificate file (enables STARTTLS) |
| `--tls-key` | | _(none)_ | TLS private key file (enables STARTTLS) |

## Keyboard shortcuts

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `d` | Delete selected email |
| `q` / `Ctrl+C` | Quit |

## Configuring your app

Point your app's SMTP settings at:

- **Host:** `127.0.0.1`
- **Port:** `2525` (or whatever you set with `--port`)
- **Auth:** none required
- **TLS:** off by default (use `--tls-cert`/`--tls-key` to enable STARTTLS)

### Example: Go `net/smtp`

```go
smtp.SendMail("127.0.0.1:2525", nil, "from@example.com", []string{"to@example.com"}, msg)
```

### Example: Nodemailer

```js
const transporter = nodemailer.createTransport({ host: "127.0.0.1", port: 2525, secure: false });
```

### Example: Python `smtplib`

```python
with smtplib.SMTP("127.0.0.1", 2525) as s:
    s.sendmail("from@example.com", ["to@example.com"], msg)
```

## License

MIT
