package exit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/asmogo/nws/protocol"
	"github.com/nbd-wtf/go-nostr"
)

const ten = 10

var errNoPublicKey = errors.New("no public key found")

func (e *Exit) announceExitNode(ctx context.Context) error {
	if !e.config.Public {
		return errNoPublicKey
	}
	go func() {
		for {
			event := nostr.Event{
				PubKey:    e.publicKey,
				CreatedAt: nostr.Now(),
				Kind:      protocol.KindAnnouncementEvent,
				Tags: nostr.Tags{
					nostr.Tag{"expiration", strconv.FormatInt(time.Now().Add(time.Second*ten).Unix(), ten)},
				},
			}
			err := event.Sign(e.config.NostrPrivateKey)
			if err != nil {
				slog.Error("could not sign event", "error", err)
				continue
			}
			// publish the event
			for _, relay := range e.relays {
				err = relay.Publish(ctx, event)
				if err != nil {
					slog.Error("could not publish event", "error", err)
					// do not return here, try to publish the event to other relays
				}
			}
			time.Sleep(time.Second * ten)
		}
	}()
	return nil
}

func (e *Exit) DeleteEvent(ctx context.Context, event *nostr.Event) error {
	for _, responseRelay := range e.config.NostrRelays {
		var relay *nostr.Relay
		relay, err := e.pool.EnsureRelay(responseRelay)
		if err != nil {
			return fmt.Errorf("failed to ensure relay: %w", err)
		}
		event := nostr.Event{
			CreatedAt: nostr.Now(),
			PubKey:    e.publicKey,
			Kind:      nostr.KindDeletion,
			Tags: nostr.Tags{
				nostr.Tag{"e", event.ID},
			},
		}
		err = event.Sign(e.config.NostrPrivateKey)
		if err != nil {
			return fmt.Errorf("failed to sign event: %w", err)
		}
		err = relay.Publish(ctx, event)
		if err != nil {
			return fmt.Errorf("failed to publish event: %w", err)
		}
	}
	return nil
}
