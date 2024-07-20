package socks5

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"github.com/asmogo/nws/protocol"
	"github.com/google/uuid"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip04"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/samber/lo"
	"log/slog"
	"net"
	"time"
)

type Conn struct {
	uuid             uuid.UUID
	ctx              context.Context
	cancel           context.CancelFunc
	readBuffer       bytes.Buffer
	privateKey       string
	pool             *protocol.SimplePool
	dst              string
	subscriptionChan chan protocol.IncomingEvent
	readIds          []string
	writeIds         []string
	sentBytes        [][]byte
	sub              bool
}

func (nc *Conn) Chan() chan protocol.IncomingEvent {
	return nc.subscriptionChan
}
func (nc *Conn) WriteNostrEvent(event protocol.IncomingEvent) {
	nc.subscriptionChan <- event
}

func NewConnection(ctx context.Context, opts ...NostrConnOption) *Conn {
	config := &NostrConnConfig{}
	for _, opt := range opts {
		opt(config)
	}
	ctx, c := context.WithCancel(ctx)
	nostrConnection := &Conn{
		privateKey:       config.privateKey,
		dst:              config.dst,
		pool:             protocol.NewSimplePool(ctx),
		ctx:              ctx,
		cancel:           c,
		sub:              config.sub,
		subscriptionChan: make(chan protocol.IncomingEvent),
		readIds:          make([]string, 0),
		sentBytes:        make([][]byte, 0),
	}

	if config.uuid != uuid.Nil {
		nostrConnection.uuid = config.uuid
	}

	return nostrConnection
}

func (nc *Conn) Read(b []byte) (n int, err error) {
	return nc.handleNostrRead(b, n)
}

func (nc *Conn) handleNostrRead(b []byte, n int) (int, error) {
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
			slog.Info("reading", slog.String("event", event.ID), slog.String("content", base64.StdEncoding.EncodeToString(message.Data)))
			n = copy(b, message.Data)
			return n, nil
		case <-nc.ctx.Done():
			return 0, fmt.Errorf("context canceled")
		default:
			time.Sleep(time.Millisecond * 100)
		}
	}
}

func (nc *Conn) Write(b []byte) (n int, err error) {
	return nc.handleNostrWrite(b, err)
}
func ParseDestination(destination string) (string, []string, error) {
	// destination can be npub or nprofile
	prefix, pubKey, err := nip19.Decode(destination)

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
func (nc *Conn) handleNostrWrite(b []byte, err error) (int, error) {
	// check if we have already sent this event

	publicKey, relays, err := ParseDestination(nc.dst)
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
		protocol.WithData(b),
	}
	ev, err := signer.CreateSignedEvent(publicKey, nostr.Tags{nostr.Tag{"p", publicKey}}, opts...)
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
				{Kinds: []int{protocol.KindEphemeralEvent},
					Authors: []string{publicKey},
					Since:   &now,
					Tags: nostr.TagMap{
						"p": []string{ev.PubKey},
					}}})
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
	slog.Info("writing", slog.String("event", ev.ID), slog.String("content", base64.StdEncoding.EncodeToString(b)))
	return len(b), nil
}

func (nc *Conn) Close() error {
	nc.cancel()
	return nil
}

func (nc *Conn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9333}
}

func (nc *Conn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
}

/*
	func (c *Conn) LocalAddr() net.Addr {
		return &protocol.NostrAddress{Nprofile: c.dst}
	}

	func (c *Conn) RemoteAddr() net.Addr {
		return &protocol.NostrAddress{Nprofile: c.dst}
	}
*/
func (nc *Conn) SetDeadline(t time.Time) error {
	return nil
}

func (nc *Conn) SetReadDeadline(t time.Time) error {
	return nil
}

func (nc *Conn) SetWriteDeadline(t time.Time) error {
	return nil
}

type NostrConnConfig struct {
	privateKey string
	dst        string
	uuid       uuid.UUID
	sub        bool
}

type NostrConnOption func(*NostrConnConfig)

func WithPrivateKey(privateKey string) NostrConnOption {
	return func(config *NostrConnConfig) {
		config.privateKey = privateKey
	}
}
func WithSub(...bool) NostrConnOption {
	return func(config *NostrConnConfig) {
		config.sub = true
	}
}
func WithDst(dst string) NostrConnOption {
	return func(config *NostrConnConfig) {
		config.dst = dst
	}
}

func WithUUID(uuid uuid.UUID) NostrConnOption {
	return func(config *NostrConnConfig) {
		config.uuid = uuid
	}
}

func (nc *Conn) HandleSubscription() {
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
