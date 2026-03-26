package storage

import (
	"fmt"
	"os"
	"strings"

	smtppkg "github.com/jonmilley/gofakesmtp/internal/smtp"
)

// Save writes the raw bytes of email to a .eml file in dir.
// The filename format is: <RFC3339-timestamp>-<from>.eml
// with colons replaced by hyphens for filesystem compatibility.
// dir is created if it does not exist.
func Save(email smtppkg.Email, dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	ts := strings.ReplaceAll(email.ReceivedAt.UTC().Format("2006-01-02T15-04-05Z"), ":", "-")
	filename := fmt.Sprintf("%s-%s.eml", ts, email.From)
	path := fmt.Sprintf("%s/%s", dir, filename)

	if err := os.WriteFile(path, email.Raw, 0644); err != nil {
		return fmt.Errorf("write email file: %w", err)
	}
	return nil
}
