package netstr

import (
	"context"
	"fmt"
	"github.com/asmogo/nws/config"
	"github.com/asmogo/nws/protocol"
	"github.com/google/uuid"
	"github.com/nbd-wtf/go-nostr"
	"log/slog"
	"net"
)

type DialOptions struct {
	Pool            *nostr.SimplePool
	PublicAddress   string
	ConnectionID    uuid.UUID
	MessageType     protocol.MessageType
	TargetPublicKey string
}

// DialSocks connects to a destination using the provided SimplePool and returns a Dialer function.
// It creates a new Connection using the specified context, private key, destination address,
// It parses the destination address to get the public key and relays.
// It creates a signed event using the private key, public key, and destination address.
// It ensures that the relays are available in the pool and publishes the signed event to each relay.
// Finally, it returns the Connection and nil error. If there are any errors, nil connection and the error are returned.
func DialSocks(options DialOptions, config *config.EntryConfig) func(ctx context.Context, net_, addr string) (net.Conn, error) {
	return func(ctx context.Context, net_, addr string) (net.Conn, error) {
		key := nostr.GeneratePrivateKey()
		connection := NewConnection(ctx,
			WithPrivateKey(key),
			WithDst(addr),
			WithSub(),
			WithDefaultRelays(config.NostrRelays),
			WithTargetPublicKey(options.TargetPublicKey),
			WithUUID(options.ConnectionID))

		var publicKey string
		var relays []string
		var err error
		if options.TargetPublicKey != "" {
			publicKey, relays = options.TargetPublicKey, config.NostrRelays
		} else {
			publicKey, relays, err = connection.parseDestination()
			if err != nil {
				slog.Error("error parsing host", err)
				return nil, fmt.Errorf("error parsing host: %w", err)
			}
		}
		// create nostr signed event
		signer, err := protocol.NewEventSigner(key)
		if err != nil {
			return nil, err
		}
		opts := []protocol.MessageOption{
			protocol.WithType(options.MessageType),
			protocol.WithUUID(options.ConnectionID),
		}
		if options.PublicAddress != "" {
			opts = append(opts, protocol.WithEntryPublicAddress(options.PublicAddress))
		}
		opts = append(opts, protocol.WithDestination(addr))

		ev, err := signer.CreateSignedEvent(publicKey, protocol.KindEphemeralEvent,
			nostr.Tags{nostr.Tag{"p", publicKey}},
			opts...)

		for _, relayUrl := range relays {
			relay, err := options.Pool.EnsureRelay(relayUrl)
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
