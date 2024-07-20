package exit

import (
	"crypto/tls"
	"fmt"
	"github.com/asmogo/nws/config"
	"github.com/asmogo/nws/protocol"
	"github.com/asmogo/nws/socks5"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip04"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/puzpuzpuz/xsync/v3"
	"golang.org/x/net/context"
	"log"
	"log/slog"
	"net"
	"net/http"
	_ "net/http/pprof"
)

type Exit struct {
	pool               *protocol.SimplePool
	config             *config.ExitConfig
	relays             []*nostr.Relay
	connectionMap      *xsync.MapOf[string, net.Conn]
	nostrConnectionMap *xsync.MapOf[string, *socks5.Conn]
	mutexMap           *MutexMap
	incomingChannel    chan protocol.IncomingEvent
}

func NewExit(ctx context.Context, config *config.ExitConfig) *Exit {
	// todo -- this is for debugging purposes only and should be removed
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	pool := protocol.NewSimplePool(ctx)

	exit := &Exit{
		connectionMap:      xsync.NewMapOf[string, net.Conn](),
		nostrConnectionMap: xsync.NewMapOf[string, *socks5.Conn](),
		config:             config,
		pool:               pool,
		mutexMap:           NewMutexMap(),
	}

	for _, relayUrl := range config.NostrRelays {
		relay, err := exit.pool.EnsureRelay(relayUrl)
		if err != nil {
			fmt.Println(err)
			continue
		}
		exit.relays = append(exit.relays, relay)
		fmt.Printf("added relay connection to %s\n", relayUrl)
	}
	pubKey, err := nostr.GetPublicKey(config.NostrPrivateKey)
	if err != nil {
		panic(err)
	}
	profile, err := nip19.EncodeProfile(pubKey,
		config.NostrRelays)
	if err != nil {
		panic(err)
	}
	slog.Info("created exit node", "profile", profile)

	return exit
}

func (e *Exit) SetSubscriptions(ctx context.Context) error {
	pubKey, err := nostr.GetPublicKey(e.config.NostrPrivateKey)
	if err != nil {
		return err
	}
	now := nostr.Now()
	if err := e.handleSubscription(ctx, pubKey, now); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (e *Exit) handleSubscription(ctx context.Context, pubKey string, since nostr.Timestamp) error {
	incomingEventChannel := e.pool.SubMany(ctx, e.config.NostrRelays, nostr.Filters{
		{Kinds: []int{protocol.KindEphemeralEvent},
			Since: &since,
			Tags: nostr.TagMap{
				"p": []string{pubKey},
			}},
	})
	e.incomingChannel = incomingEventChannel
	go e.handleEvents(ctx, incomingEventChannel)
	return nil
}

func (e *Exit) handleEvents(ctx context.Context, subscription chan protocol.IncomingEvent) {
	for {
		select {
		case event := <-subscription:
			slog.Debug("received event", "event", event)
			if event.Relay == nil {
				continue
			}
			go e.processMessage(ctx, event)
		case <-ctx.Done():
			return
		}
	}
}

func (e *Exit) processMessage(ctx context.Context, msg protocol.IncomingEvent) {
	sharedKey, err := nip04.ComputeSharedSecret(msg.PubKey, e.config.NostrPrivateKey)
	if err != nil {
		return
	}
	decodedMessage, err := nip04.Decrypt(msg.Content, sharedKey)
	if err != nil {
		return
	}
	protocolMessage, err := protocol.UnmarshalJSON([]byte(decodedMessage))
	if err != nil {
		slog.Error("could not unmarshal message")
		return
	}
	switch protocolMessage.Type {
	case protocol.MessageConnect:
		e.handleConnect(ctx, msg, protocolMessage, false)
	case protocol.MessageTypeSocks5:
		e.handleSocks5ProxyMessage(ctx, msg, protocolMessage, sharedKey)
	}
}

// handleConnect will create a new socks5 connection
func (e *Exit) handleConnect(ctx context.Context, msg protocol.IncomingEvent, protocolMessage *protocol.Message, isTLS bool) {
	e.mutexMap.Lock(protocolMessage.Key.String())
	defer e.mutexMap.Unlock(protocolMessage.Key.String())
	receiver, err := nip19.EncodeProfile(msg.PubKey, []string{msg.Relay.String()})
	if err != nil {
		return
	}
	connection := socks5.NewConnection(
		ctx,
		socks5.WithPrivateKey(e.config.NostrPrivateKey),
		socks5.WithDst(receiver),
		socks5.WithUUID(protocolMessage.Key),
	)
	var dst net.Conn
	if isTLS {
		conf := tls.Config{InsecureSkipVerify: true}
		dst, err = tls.Dial("tcp", e.config.BackendHost, &conf)
	} else {
		dst, err = net.Dial("tcp", e.config.BackendHost)
	}
	if err != nil {
		slog.Error("could not connect to backend", "error", err)
		return
	}

	e.nostrConnectionMap.Store(protocolMessage.Key.String(), connection)

	go socks5.Proxy(dst, connection, nil)
	go socks5.Proxy(connection, dst, nil)
}

func (e *Exit) handleSocks5ProxyMessage(
	ctx context.Context,
	msg protocol.IncomingEvent,
	protocolMessage *protocol.Message,
	sharedKey []byte,
) {
	e.mutexMap.Lock(protocolMessage.Key.String())
	defer e.mutexMap.Unlock(protocolMessage.Key.String())
	dst, ok := e.nostrConnectionMap.Load(protocolMessage.Key.String())
	if !ok {
		return
	}
	dst.WriteNostrEvent(msg)
}
