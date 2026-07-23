package email

import "context"

// Message is a provider-agnostic single email to send.
type Message struct {
	To       string
	Subject  string
	HTMLBody string
}

// Sender sends a single email. Implementations wrap a specific provider
// (e.g. Resend); swapping providers means writing a new Sender, not
// changing any of its callers.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}
