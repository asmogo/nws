package protocol

import (
	"bytes"
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip04"
	"log/slog"
	"net"
	"sync"
	"time"
)

const KindEphemeralEvent int = 38333

type EventSigner struct {
	PublicKey  string
	privateKey string
	wg         *sync.WaitGroup
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
		wg:         &sync.WaitGroup{},
	}
	return signer, nil
}
func (s *EventSigner) CreateEvent(tags nostr.Tags) nostr.Event {
	return nostr.Event{
		PubKey:    s.PublicKey,
		CreatedAt: nostr.Now(),
		Kind:      KindEphemeralEvent,
		Tags:      tags,
	}
}

func (s *EventSigner) CreateSignedEvent(targetPublicKey string, tags nostr.Tags, opts ...MessageOption) (nostr.Event, error) {
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
	ev := s.CreateEvent(tags)
	ev.Content = encryptedMessage
	// calling Sign sets the event ID field and the event Sig field
	err = ev.Sign(s.privateKey)
	if err != nil {
		return nostr.Event{}, err
	}
	return ev, nil
}

func (s *EventSigner) DecryptAndWrite(
	ctx context.Context,
	exitNodeSub chan IncomingEvent,
	w net.Conn,
	outChannel chan nostr.Event,
	sessionID uuid.UUID) {
	for {
		select {
		default:
			time.Sleep(time.Millisecond * 100)

		case <-ctx.Done():
			slog.Info("context done")
			return
		case outgoingEvent := <-outChannel:
			slog.Info("received outgoing", "id", outgoingEvent.ID)

		case ev := <-exitNodeSub:
			slog.Info("received response from exit node", "id", ev.ID)

			/*	if !ev.Tags.ContainsAny("e", []string{ev.ID, ev.PubKey}) {
				slog.Info("skipping event", ev)
				continue
			}*/
			// decrypt the message
			sharedKey, err := nip04.ComputeSharedSecret(ev.PubKey, s.privateKey)
			if err != nil {
				fmt.Println(err)
				continue
			}
			decryptedMessage, err := nip04.Decrypt(ev.Content, sharedKey)
			if err != nil {
				fmt.Println(err)
				continue
			}
			message, err := UnmarshalJSON([]byte(decryptedMessage))
			if err != nil {
				slog.Error("error unmarshalling message", err)
				continue
			}
			if message.Key != sessionID {
				slog.Info("skipping message", "id", message.Key)
				continue
			}
			// print the message
			// find first null byte in message data and truncate it
			nullByteIndex := bytes.IndexByte(message.Data, 0)
			if nullByteIndex != -1 {
				message.Data = message.Data[:nullByteIndex]
			}
			_, err = w.Write(message.Data)
			if err != nil {
				slog.Error("error writing to client", err)
				return
			}
			return
		}
	}
}
