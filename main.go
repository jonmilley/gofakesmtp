package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	gosmtp "github.com/emersion/go-smtp"
	"github.com/spf13/cobra"

	oursmtp "github.com/jonmilley/gofakesmtp/internal/smtp"
	"github.com/jonmilley/gofakesmtp/internal/tui"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	var (
		port        int
		addr        string
		outputDir   string
		tlsCert     string
		tlsKey      string
		smtpUser    string
		smtpPass    string
		smtpAuthAny bool
		sessionLog  string
	)

	cmd := &cobra.Command{
		Use:   "gofakesmtp",
		Short: "A fake SMTP server with a terminal UI for testing email",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate TLS flags
			if (tlsCert == "") != (tlsKey == "") {
				return fmt.Errorf("--tls-cert and --tls-key must both be provided to enable STARTTLS")
			}

			// Validate auth flags
			if (smtpUser == "") != (smtpPass == "") {
				return fmt.Errorf("--smtp-user and --smtp-pass must both be provided to require authentication")
			}
			if smtpAuthAny && (smtpUser != "" || smtpPass != "") {
				return fmt.Errorf("--smtp-auth-any cannot be combined with --smtp-user/--smtp-pass")
			}

			var logWriter io.Writer
			if sessionLog != "" {
				f, err := os.OpenFile(sessionLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					return fmt.Errorf("open session log: %w", err)
				}
				defer f.Close()
				logWriter = f
			}

			ch := make(chan oursmtp.Email, 100)

			// Configure and start SMTP server
			backend := oursmtp.NewBackend(ch, oursmtp.Config{
				Username:     smtpUser,
				Password:     smtpPass,
				AllowAnyAuth: smtpAuthAny,
				SessionLog:   logWriter,
			})
			smtpServer := gosmtp.NewServer(backend)
			smtpServer.Addr = fmt.Sprintf("%s:%d", addr, port)
			smtpServer.Domain = "localhost"
			smtpServer.AllowInsecureAuth = true
			smtpServer.Debug = logWriter

			if tlsCert != "" {
				cert, err := tls.LoadX509KeyPair(tlsCert, tlsKey)
				if err != nil {
					return fmt.Errorf("load TLS key pair: %w", err)
				}
				smtpServer.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
			}

			go func() {
				if err := smtpServer.ListenAndServe(); err != nil {
					// Server closed on quit — not an error worth logging.
				}
			}()

			// Start TUI
			model := tui.New(ch, outputDir, 0, 0)
			p := tea.NewProgram(model, tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				return fmt.Errorf("TUI error: %w", err)
			}

			smtpServer.Close()
			return nil
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 2525, "SMTP port to listen on")
	cmd.Flags().StringVarP(&addr, "addr", "a", "127.0.0.1", "Address to bind to")
	cmd.Flags().StringVarP(&outputDir, "output-dir", "o", "", "Directory to auto-save received emails as .eml files")
	cmd.Flags().StringVar(&tlsCert, "tls-cert", "", "Path to TLS certificate file (enables STARTTLS)")
	cmd.Flags().StringVar(&tlsKey, "tls-key", "", "Path to TLS private key file (enables STARTTLS)")
	cmd.Flags().StringVar(&smtpUser, "smtp-user", "", "Username for SMTP AUTH PLAIN (requires --smtp-pass; when set, auth is required)")
	cmd.Flags().StringVar(&smtpPass, "smtp-pass", "", "Password for SMTP AUTH PLAIN (requires --smtp-user)")
	cmd.Flags().BoolVar(&smtpAuthAny, "smtp-auth-any", false, "Advertise AUTH PLAIN and accept any credentials (does not gate MAIL/RCPT/DATA)")
	cmd.Flags().StringVar(&sessionLog, "session-log", "", "Path to append the full SMTP session wire log")

	return cmd
}
