package protocol

import (
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip04"
	"log/slog"
)

// KindEphemeralEvent represents the unique identifier for ephemeral events.
const KindEphemeralEvent int = 38333

// EventSigner represents a signer that can create and sign events.
//
// EventSigner provides methods for creating unsigned events, creating signed events
type EventSigner struct {
	PublicKey  string
	privateKey string
}

// NewEventSigner creates a new EventSigner
func NewEventSigner(privateKey string) (*EventSigner, error) {
	myPublicKey, err := nostr.GetPublicKey(privateKey)
	if err != nil {
		slog.Error("could not generate pubkey")
		return nil, err
	}
	signer := &EventSigner{
		privateKey: privateKey,
		PublicKey:  myPublicKey,
	}
	return signer, nil
}

// CreateEvent creates a new Event with the provided tags. The Public Key and the
// current timestamp are set automatically. The Kind is set to KindEphemeralEvent.
func (s *EventSigner) CreateEvent(kind int, tags nostr.Tags) nostr.Event {
	return nostr.Event{
		PubKey:    s.PublicKey,
		CreatedAt: nostr.Now(),
		Kind:      kind,
		Tags:      tags,
	}
}

// CreateSignedEvent creates a signed Nostr event with the provided target public key, tags, and options.
// It computes the shared key between the target public key and the private key of the EventSigner.
// Then, it creates a new message with the provided options.
// The message is serialized to JSON and encrypted using the shared key.
// The method then calls CreateEvent to create a new unsigned event with the provided tags.
// The encrypted message is set as the content of the event.
// Finally, the event is signed with the private key of the EventSigner, setting the event ID and event Sig fields.
// The signed event is returned along with any error that occurs.
func (s *EventSigner) CreateSignedEvent(targetPublicKey string, kind int, tags nostr.Tags, opts ...MessageOption) (nostr.Event, error) {
	sharedKey, err := nip04.ComputeSharedSecret(targetPublicKey, s.privateKey)
	if err != nil {
		return nostr.Event{}, err
	}
	message := NewMessage(
		opts...,
	)
	messageJson, err := MarshalJSON(message)
	if err != nil {
		return nostr.Event{}, err
	}
	encryptedMessage, err := nip04.Encrypt(string(messageJson), sharedKey)
	ev := s.CreateEvent(kind, tags)
	ev.Content = encryptedMessage
	// calling Sign sets the event ID field and the event Sig field
	err = ev.Sign(s.privateKey)
	if err != nil {
		return nostr.Event{}, err
	}
	return ev, nil
}
