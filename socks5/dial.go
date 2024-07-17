package socks5

import (
	"context"
	"fmt"
	"github.com/asmogo/nws/protocol"
	"github.com/google/uuid"
	"github.com/nbd-wtf/go-nostr"
	"log/slog"
	"net"
	"strings"
)

func DialSocks(pool *protocol.SimplePool) func(ctx context.Context, net_, addr string) (net.Conn, error) {
	return func(ctx context.Context, net_, addr string) (net.Conn, error) {
		addr = strings.ReplaceAll(addr, ".", "")
		connectionID := uuid.New()
		key := nostr.GeneratePrivateKey()
		connection := NewConnection(ctx,
			WithPrivateKey(key),
			WithDst(addr),
			WithSub(),
			WithUUID(connectionID))
		go connection.HandleSubscription()

		publicKey, relays, err := ParseDestination(addr)
		if err != nil {
			slog.Error("error parsing host", err)
			return nil, fmt.Errorf("error parsing host: %w", err)
		}
		// create nostr signed event
		signer, err := protocol.NewEventSigner(key)
		if err != nil {
			return nil, err
		}
		opts := []protocol.MessageOption{
			protocol.WithType(protocol.MessageConnect),
			protocol.WithUUID(connectionID),
			protocol.WithDestination(addr),
		}
		ev, err := signer.CreateSignedEvent(publicKey,
			nostr.Tags{nostr.Tag{"p", publicKey}},
			opts...)

		for _, relayUrl := range relays {
			relay, err := pool.EnsureRelay(relayUrl)
			if err != nil {
				slog.Error("error creating relay", err)
				continue
			}
			err = relay.Publish(ctx, ev)
			if err != nil {
				return nil, err
			}
		}
		return connection, nil
	}
}
