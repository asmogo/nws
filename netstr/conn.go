package netstr

import (
	"bytes"
	"context"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/asmogo/nws/protocol"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/google/uuid"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip04"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/samber/lo"
	"log/slog"
	"net"
	"strings"
	"time"
)

// NostrConnection implements the net.Conn interface.
// It is used to establish a connection to Nostr relays.
// It provides methods for reading and writing data.
type NostrConnection struct {
	// uuid is a field of type uuid.UUID in the NostrConnection struct.
	uuid uuid.UUID
	// ctx is a field of type context.Context in the NostrConnection struct.
	ctx context.Context
	// cancel is a field of type context.CancelFunc in the NostrConnection struct.
	cancel context.CancelFunc

	// readBuffer is a field of type `bytes.Buffer` in the `NostrConnection` struct.
	// It is used to store the decrypted message from incoming events.
	// The `handleSubscription` method continuously listens for events on the subscription channel,
	// decrypts the event content, and writes the decrypted message to the `readBuffer`.
	readBuffer bytes.Buffer

	// private key of the connection
	privateKey string

	// NostrConnection represents a connection object.
	pool *nostr.SimplePool

	// dst is a field that represents the destination address for the Nostr connection configuration.
	dst string

	// subscriptionChan is a channel of type protocol.IncomingEvent.
	// It is used to write incoming events which will be read and processed by the Read method.
	subscriptionChan chan nostr.IncomingEvent

	// readIds represents the list of event IDs that have been read by the NostrConnection object.
	readIds []string

	// writeIds is a field of type []string in the NostrConnection struct.
	// It stores the IDs of the events that have been written to the connection.
	// This field is used to check if an event has already been written and avoid duplicate writes.
	writeIds []string

	// sentBytes is a field that stores the bytes of data that have been sent by the connection.
	sentBytes [][]byte

	// sub represents a boolean value indicating if a connection should subscribe to a response when writing.
	sub             bool
	defaultRelays   []string
	targetPublicKey string
}

// WriteNostrEvent writes the incoming event to the subscription channel of the NostrConnection.
// The subscription channel is used by the Read method to read events and handle them.
// Parameters:
// - event: The incoming event to be written to the subscription channel.
func (nc *NostrConnection) WriteNostrEvent(event nostr.IncomingEvent) {
	nc.subscriptionChan <- event
}

// NewConnection creates a new NostrConnection object with the provided context and options.
// It initializes the config with default values, processes the options to customize the config,
// and creates a new NostrConnection object using the config.
// The NostrConnection object includes the privateKey, dst, pool, ctx, cancel, sub, subscriptionChan, readIds, and sentBytes fields.
// If an uuid is provided in the options, it is assigned to the NostrConnection object.
// The NostrConnection object is then returned.
func NewConnection(ctx context.Context, opts ...NostrConnOption) *NostrConnection {
	ctx, c := context.WithCancel(ctx)
	nostrConnection := &NostrConnection{
		pool:             nostr.NewSimplePool(ctx),
		ctx:              ctx,
		cancel:           c,
		subscriptionChan: make(chan nostr.IncomingEvent),
		readIds:          make([]string, 0),
		sentBytes:        make([][]byte, 0),
	}
	for _, opt := range opts {
		opt(nostrConnection)
	}

	return nostrConnection
}

// Read reads data from the connection. The data is decrypted and returned in the provided byte slice.
// If there is no data available, Read blocks until data arrives or the context is canceled.
// If the context is canceled before data is received, Read returns an error.
//
// The number of bytes read is returned as n and any error encountered is returned as err.
// The content of the decrypted message is then copied to the provided byte slice b.
func (nc *NostrConnection) Read(b []byte) (n int, err error) {
	return nc.handleNostrRead(b, n)
}

// handleNostrRead reads the incoming events from the subscription channel and processes them.
// It checks if the event has already been read, decrypts the content using the shared key,
// unmarshals the decoded message and copies the content into the provided byte slice.
// It returns the number of bytes copied and any error encountered.
// If the context is canceled, it returns an error with "context canceled" message.
func (nc *NostrConnection) handleNostrRead(b []byte, n int) (int, error) {
	for {
		select {
		case event := <-nc.subscriptionChan:
			if event.Relay == nil {
				return 0, nil
			}
			// check if we have already read this event
			if lo.Contains(nc.readIds, event.ID) {
				continue
			}
			nc.readIds = append(nc.readIds, event.ID)
			sharedKey, err := nip04.ComputeSharedSecret(event.PubKey, nc.privateKey)
			if err != nil {
				return 0, err
			}
			decodedMessage, err := nip04.Decrypt(event.Content, sharedKey)
			if err != nil {
				return 0, err
			}
			message, err := protocol.UnmarshalJSON([]byte(decodedMessage))
			if err != nil {
				return 0, err
			}
			slog.Debug("reading", slog.String("event", event.ID), slog.String("content", base64.StdEncoding.EncodeToString(message.Data)))
			n = copy(b, message.Data)
			return n, nil
		case <-nc.ctx.Done():
			return 0, fmt.Errorf("context canceled")
		default:
			time.Sleep(time.Millisecond * 100)
		}
	}
}

// Write writes data to the connection.
// It delegates the writing logic to handleNostrWrite method.
// The number of bytes written and error (if any) are returned.
func (nc *NostrConnection) Write(b []byte) (int, error) {
	return nc.handleNostrWrite(b)
}

// handleNostrWrite handles the writing of a Nostr event.
// It checks if the event has already been sent, parses the destination,
// creates a message signer, creates message options, signs the event,
// publishes the event to relays, and appends the sent bytes to the connection's sentBytes array.
// The method returns the number of bytes written and any error that occurred.
func (nc *NostrConnection) handleNostrWrite(b []byte) (int, error) {
	// check if we have already sent this event
	publicKey, relays, err := nc.parseDestination()
	if err != nil {
		return 0, err
	}
	signer, err := protocol.NewEventSigner(nc.privateKey)
	if err != nil {
		return 0, err
	}
	// create message options
	opts := []protocol.MessageOption{
		protocol.WithUUID(nc.uuid),
		protocol.WithType(protocol.MessageTypeSocks5),
		protocol.WithDestination(nc.dst),
		protocol.WithData(b),
	}
	ev, err := signer.CreateSignedEvent(
		publicKey,
		protocol.KindEphemeralEvent,
		nostr.Tags{nostr.Tag{"p", publicKey}},
		opts...,
	)
	if err != nil {
		return 0, err
	}
	if lo.Contains(nc.writeIds, ev.ID) {
		slog.Info("event already sent", slog.String("event", ev.ID))
		return 0, nil
	}
	nc.writeIds = append(nc.writeIds, ev.ID)

	if nc.sub {
		nc.sub = false
		now := nostr.Now()
		incomingEventChannel := nc.pool.SubMany(nc.ctx, relays,
			nostr.Filters{
				{
					Kinds:   []int{protocol.KindEphemeralEvent},
					Authors: []string{publicKey},
					Since:   &now,
					Tags: nostr.TagMap{
						"p": []string{ev.PubKey},
					},
				},
			},
		)
		nc.subscriptionChan = incomingEventChannel
	}
	for _, responseRelay := range relays {
		var relay *nostr.Relay
		relay, err = nc.pool.EnsureRelay(responseRelay)
		if err != nil {
			return 0, err
		}
		err = relay.Publish(nc.ctx, ev)
		if err != nil {
			return 0, err
		}
	}
	nc.sentBytes = append(nc.sentBytes, b)
	slog.Debug("writing", slog.String("event", ev.ID), slog.String("content", base64.StdEncoding.EncodeToString(b)))
	return len(b), nil
}

// parseDestination takes a destination string and returns a public key and relays.
// The destination can be "npub" or "nprofile".
// If the prefix is "npub", the public key is extracted.
// If the prefix is "nprofile", the public key and relays are extracted.
// Returns the public key, relays (if any), and any error encountered.
func (nc *NostrConnection) parseDestination() (string, []string, error) {
	// check if destination ends with .nostr
	if strings.HasPrefix(nc.dst, "npub") || strings.HasPrefix(nc.dst, "nprofile") {
		// destination can be npub or nprofile
		prefix, pubKey, err := nip19.Decode(nc.dst)

		if err != nil {
			return "", nil, err
		}

		var relays []string
		var publicKey string

		switch prefix {
		case "npub":
			publicKey = pubKey.(string)
		case "nprofile":
			profilePointer := pubKey.(nostr.ProfilePointer)
			publicKey = profilePointer.PublicKey
			relays = profilePointer.Relays
		}
		return publicKey, relays, nil
	}
	return nc.parseDestinationDomain()
}

func (nc *NostrConnection) parseDestinationDomain() (string, []string, error) {
	url, err := protocol.Parse(nc.dst)
	if err != nil {
		return "", nil, err
	}
	if !url.IsDomain {
		// try to parse as ip
		ip := net.ParseIP(url.Name)
		if ip != nil {
			return nc.targetPublicKey, nc.defaultRelays, nil
		}
		return "", nil, fmt.Errorf("destination is not a domain")

	}
	if url.TLD != "nostr" {
		// parse public key
		/*pubKey,err := nostr.GetPublicKey(nc.privateKey)
		if err != nil {
			return "", nil, err
		}*/
		return nc.targetPublicKey, nc.defaultRelays, nil
	}
	var subdomains []string
	split := strings.Split(url.SubName, ".")
	for _, subdomain := range split {
		decodedSubDomain, err := base32.HexEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(subdomain))
		if err != nil {
			continue
		}
		subdomains = append(subdomains, string(decodedSubDomain))
	}

	// base32 decode the subdomain
	decodedPubKey, err := base32.HexEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(url.Name))

	if err != nil {
		return "", nil, err
	}
	pk, err := schnorr.ParsePubKey(decodedPubKey)
	if err != nil {
		return "", nil, err
	}
	// todo -- check if this is correct
	return hex.EncodeToString(pk.SerializeCompressed())[2:], subdomains, nil
}

func (nc *NostrConnection) Close() error {
	nc.cancel()
	return nil
}

func (nc *NostrConnection) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9333}
}

func (nc *NostrConnection) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
}

func (nc *NostrConnection) SetDeadline(t time.Time) error {
	return nil
}

func (nc *NostrConnection) SetReadDeadline(t time.Time) error {
	return nil
}

func (nc *NostrConnection) SetWriteDeadline(t time.Time) error {
	return nil
}

// NostrConnOption is a functional option type for configuring NostrConnConfig.
type NostrConnOption func(*NostrConnection)

// WithPrivateKey sets the private key for the NostrConnConfig.
func WithPrivateKey(privateKey string) NostrConnOption {
	return func(config *NostrConnection) {
		config.privateKey = privateKey
	}
}

// WithPrivateKey sets the private key for the NostrConnConfig.
func WithDefaultRelays(defaultRelays []string) NostrConnOption {
	return func(config *NostrConnection) {
		config.defaultRelays = defaultRelays
	}
}

// WithTargetPublicKey sets the private key for the NostrConnConfig.
func WithTargetPublicKey(pubKey string) NostrConnOption {
	return func(config *NostrConnection) {
		config.targetPublicKey = pubKey
	}
}

// WithSub is a function that returns a NostrConnOption. When this option is applied
// to a NostrConnConfig, it sets the 'sub' field to true, indicating that
// the connection will handle subscriptions.
func WithSub(...bool) NostrConnOption {
	return func(connection *NostrConnection) {
		connection.sub = true
		go connection.handleSubscription()
	}
}

// WithDst is a NostrConnOption function that sets the destination address for the Nostr connection configuration.
// It takes a string parameter `dst` and updates the `config.dst` field accordingly.
func WithDst(dst string) NostrConnOption {
	return func(connection *NostrConnection) {
		connection.dst = dst
	}
}

// WithUUID sets the UUID option for creating a NostrConnConfig.
// It assigns the provided UUID to the config's uuid field.
func WithUUID(uuid uuid.UUID) NostrConnOption {
	return func(connection *NostrConnection) {
		connection.uuid = uuid
	}
}

// handleSubscription handles the subscription channel for incoming events.
// It continuously listens for events on the subscription channel and performs necessary operations.
// If the event has a valid relay, it computes the shared key and decrypts the event content.
// The decrypted message is then written to the read buffer.
// If the context is canceled, the method returns.
func (nc *NostrConnection) handleSubscription() {
	for {
		select {
		case event := <-nc.subscriptionChan:
			if event.Relay == nil {
				continue
			}
			sharedKey, err := nip04.ComputeSharedSecret(event.PubKey, nc.privateKey)
			if err != nil {
				continue
			}
			decodedMessage, err := nip04.Decrypt(event.Content, sharedKey)
			if err != nil {
				continue
			}
			nc.readBuffer.Write([]byte(decodedMessage))
		case <-nc.ctx.Done():
			return
		}
	}
}
