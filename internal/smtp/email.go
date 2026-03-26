// internal/smtp/email.go
package smtp

import "time"

// Email holds a parsed SMTP message received by the server.
type Email struct {
	From       string
	To         []string
	Subject    string
	Body       string
	Raw        []byte
	ReceivedAt time.Time
}
