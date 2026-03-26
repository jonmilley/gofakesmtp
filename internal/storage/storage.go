package storage

import (
	"fmt"
	"os"
	"path/filepath"
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
	filename := fmt.Sprintf("%s-%s.eml", ts, sanitizeFilename(email.From))
	path := filepath.Join(dir, filename)

	if err := os.WriteFile(path, email.Raw, 0644); err != nil {
		return fmt.Errorf("write email file: %w", err)
	}
	return nil
}

// sanitizeFilename replaces filesystem-unsafe characters with underscores.
func sanitizeFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '.' || r == '-' || r == '_' || r == '@' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}
