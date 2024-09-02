package protocol

import (
	"fmt"

	"github.com/ekzyis/nip44"
	"github.com/nbd-wtf/go-nostr"
)

// KindEphemeralEvent represents the unique identifier for ephemeral events.
const KindEphemeralEvent int = 28333

// KindAnnouncementEvent represents the unique identifier for announcement events.
const KindAnnouncementEvent int = 38333

// KindCertificateEvent represents the unique identifier for certificate events.
const KindCertificateEvent int = 38334

// KindPrivateKeyEvent represents the unique identifier for private key events.
const KindPrivateKeyEvent int = 38335

// EventSigner represents a signer that can create and sign events.
//
// EventSigner provides methods for creating unsigned events, creating signed events.
type EventSigner struct {
	PublicKey  string
	privateKey string
}

// NewEventSigner creates a new EventSigner.
func NewEventSigner(privateKey string) (*EventSigner, error) {
	myPublicKey, err := nostr.GetPublicKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("could not generate public key: %w", err)
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
func (s *EventSigner) CreateSignedEvent(
	targetPublicKey string,
	kind int,
	tags nostr.Tags,
	opts ...MessageOption,
) (nostr.Event, error) {
	privateKeyBytes, targetPublicKeyBytes, err := GetEncryptionKeys(s.privateKey, targetPublicKey)
	if err != nil {
		return nostr.Event{}, fmt.Errorf("could not get encryption keys: %w", err)
	}
	sharedKey, err := nip44.GenerateConversationKey(privateKeyBytes, targetPublicKeyBytes)
	if err != nil {
		return nostr.Event{}, fmt.Errorf("could not compute shared key: %w", err)
	}
	message := NewMessage(
		opts...,
	)
	messageJSON, err := MarshalJSON(message)
	if err != nil {
		return nostr.Event{}, fmt.Errorf("could not marshal message: %w", err)
	}
	encryptedMessage, err := nip44.Encrypt(sharedKey, string(messageJSON), &nip44.EncryptOptions{
		Salt:    nil,
		Version: 0,
	})
	if err != nil {
		return nostr.Event{}, fmt.Errorf("could not encrypt message: %w", err)
	}
	event := s.CreateEvent(kind, tags)
	event.Content = encryptedMessage
	// calling Sign sets the event ID field and the event Sig field
	err = event.Sign(s.privateKey)
	if err != nil {
		return nostr.Event{}, fmt.Errorf("could not sign event: %w", err)
	}
	return event, nil
}
