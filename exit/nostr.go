package exit

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/nbd-wtf/go-nostr"
)

func (e *Exit) announceExitNode(ctx context.Context) error {
	if !e.config.Public {
		return nil
	}
	go func() {
		for {
			event := nostr.Event{
				PubKey:    e.publicKey,
				CreatedAt: nostr.Now(),
				Kind:      nostr.KindTextNote,
				Tags: nostr.Tags{
					nostr.Tag{"n", "nws"},
					nostr.Tag{"expiration", strconv.FormatInt(time.Now().Add(time.Second*10).Unix(), 20)},
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
			time.Sleep(time.Second * 10)
		}
	}()
	return nil
}

func (e *Exit) DeleteEvent(ctx context.Context, ev *nostr.Event) error {
	for _, responseRelay := range e.config.NostrRelays {
		var relay *nostr.Relay
		relay, err := e.pool.EnsureRelay(responseRelay)
		if err != nil {
			return err
		}
		event := nostr.Event{
			CreatedAt: nostr.Now(),
			PubKey:    e.publicKey,
			Kind:      nostr.KindDeletion,
			Tags: nostr.Tags{
				nostr.Tag{"e", ev.ID},
			},
		}
		err = event.Sign(e.config.NostrPrivateKey)
		err = relay.Publish(ctx, event)
		if err != nil {
			return err
		}
	}
	return nil
}
