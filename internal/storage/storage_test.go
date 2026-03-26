package storage_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	smtppkg "github.com/jonmilley/gofakesmtp/internal/smtp"
	"github.com/jonmilley/gofakesmtp/internal/storage"
)

func TestSave_WritesFileWithCorrectContent(t *testing.T) {
	dir := t.TempDir()
	email := smtppkg.Email{
		From:       "sender@example.com",
		To:         []string{"receiver@example.com"},
		Subject:    "Hello",
		Body:       "World",
		Raw:        []byte("raw email bytes"),
		ReceivedAt: time.Date(2026, 3, 25, 14, 32, 0, 0, time.UTC),
	}

	err := storage.Save(email, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file, got %d", len(entries))
	}

	filename := entries[0].Name()
	if !strings.HasSuffix(filename, ".eml") {
		t.Errorf("expected .eml extension, got %q", filename)
	}
	if !strings.Contains(filename, "2026-03-25T14-32-00Z") {
		t.Errorf("expected RFC3339 timestamp in filename, got %q", filename)
	}
	if !strings.Contains(filename, "sender@example.com") {
		t.Errorf("expected from address in filename, got %q", filename)
	}

	content, err := os.ReadFile(filepath.Join(dir, filename))
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != "raw email bytes" {
		t.Errorf("expected raw bytes in file, got %q", string(content))
	}
}

func TestSave_CreatesDirectoryIfNotExists(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "subdir", "emails")
	email := smtppkg.Email{
		From:       "a@b.com",
		Raw:        []byte("data"),
		ReceivedAt: time.Now(),
	}

	err := storage.Save(email, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("expected dir to be created: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 file, got %d", len(entries))
	}
}
