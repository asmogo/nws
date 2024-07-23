package exit

import (
	"crypto/tls"
	"fmt"
	"github.com/asmogo/nws/config"
	"github.com/asmogo/nws/netstr"
	"github.com/asmogo/nws/protocol"
	"github.com/asmogo/nws/socks5"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip04"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/puzpuzpuz/xsync/v3"
	"golang.org/x/net/context"
	"log/slog"
	"net"
	_ "net/http/pprof"
)

const (
	startingReverseProxyMessage = "starting exit node with https reverse proxy"
	generateKeyMessage          = "Generated new private key. Please set your environment using the new key, otherwise your key will be lost."
)

// Exit represents a structure that holds information related to an exit node.
type Exit struct {

	// pool represents a pool of relays and manages the subscription to incoming events from relays.
	pool *nostr.SimplePool

	// config is a field in the Exit struct that holds information related to exit node configuration.
	config *config.ExitConfig

	// relays represents a slice of *nostr.Relay, which contains information about the relay nodes used by the Exit node.
	// Todo -- check if this is deprecated
	relays []*nostr.Relay

	// nostrConnectionMap is a concurrent map used to store connections for the Exit node.
	// It is used to establish and maintain connections between the Exit node and the backend host.
	nostrConnectionMap *xsync.MapOf[string, *netstr.NostrConnection]

	// mutexMap is a field in the Exit struct that represents a map used for synchronizing access to resources based on a string key.
	mutexMap *MutexMap

	// incomingChannel represents a channel used to receive incoming events from relays.
	incomingChannel chan nostr.IncomingEvent

	nprofile  string
	publicKey string
}

// NewExit creates a new Exit node with the provided context and config.
func NewExit(ctx context.Context, exitNodeConfig *config.ExitConfig) *Exit {
	// generate new private key if it is not set
	if exitNodeConfig.NostrPrivateKey == "" {
		// generate new private key
		exitNodeConfig.NostrPrivateKey = nostr.GeneratePrivateKey()
		slog.Warn(generateKeyMessage, "key", exitNodeConfig.NostrPrivateKey)
	}
	// get public key from private key
	pubKey, err := nostr.GetPublicKey(exitNodeConfig.NostrPrivateKey)
	if err != nil {
		panic(err)
	}
	// encode profile
	profile, err := nip19.EncodeProfile(pubKey,
		exitNodeConfig.NostrRelays)
	if err != nil {
		panic(err)
	}
	// create a new pool
	pool := nostr.NewSimplePool(ctx)

	exit := &Exit{
		nostrConnectionMap: xsync.NewMapOf[string, *netstr.NostrConnection](),
		pool:               pool,
		mutexMap:           NewMutexMap(),
		publicKey:          pubKey,
		nprofile:           profile,
	}
	// start reverse proxy if https port is set
	if exitNodeConfig.HttpsPort != 0 {
		exitNodeConfig.BackendHost = fmt.Sprintf(":%d", exitNodeConfig.HttpsPort)
		go func(cfg *config.ExitConfig) {
			slog.Info(startingReverseProxyMessage, "port", cfg.HttpsPort)
			err := exit.StartReverseProxy(cfg.HttpsTarget, cfg.HttpsPort)
			if err != nil {
				panic(err)
			}
		}(exitNodeConfig)
	}
	// set config
	exit.config = exitNodeConfig
	// add relays to the pool
	for _, relayUrl := range exitNodeConfig.NostrRelays {
		relay, err := exit.pool.EnsureRelay(relayUrl)
		if err != nil {
			fmt.Println(err)
			continue
		}
		exit.relays = append(exit.relays, relay)
		fmt.Printf("added relay connection to %s\n", relayUrl)
	}

	slog.Info("created exit node", "profile", profile)
	// setup subscriptions
	err = exit.setSubscriptions(ctx)
	if err != nil {
		panic(err)
	}
	return exit
}

// setSubscriptions sets up subscriptions for the Exit node to receive incoming events from the specified relays.
// It first obtains the public key using the configured Nostr private key.
// Then it calls the `handleSubscription` method to open a subscription to the relays with the specified filters.
// This method runs in a separate goroutine and continuously handles the incoming events by calling the `processMessage` method.
// If the context is canceled before the subscription is established, it returns the context error.
// If any errors occur during the process, they are returned.
// This method should be called once when starting the Exit node.
func (e *Exit) setSubscriptions(ctx context.Context) error {
	pubKey, err := nostr.GetPublicKey(e.config.NostrPrivateKey)
	if err != nil {
		return err
	}
	now := nostr.Now()
	if err = e.handleSubscription(ctx, pubKey, now); err != nil {
		return err
	}
	return nil

}

// handleSubscription handles the subscription to incoming events from relays based on the provided filters.
// It sets up the incoming event channel and starts a goroutine to handle the events.
// It returns an error if there is any issue with the subscription.
func (e *Exit) handleSubscription(ctx context.Context, pubKey string, since nostr.Timestamp) error {
	incomingEventChannel := e.pool.SubMany(ctx, e.config.NostrRelays, nostr.Filters{
		{Kinds: []int{protocol.KindEphemeralEvent},
			Since: &since,
			Tags: nostr.TagMap{
				"p": []string{pubKey},
			}},
	})
	e.incomingChannel = incomingEventChannel
	return nil
}

// ListenAndServe handles incoming events from the subscription channel.
// It processes each event by calling the processMessage method, as long as the event is not nil.
// If the context is canceled (ctx.Done() receives a value), the method returns.
func (e *Exit) ListenAndServe(ctx context.Context) {
	for {
		select {
		case event := <-e.incomingChannel:
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

// processMessage decrypts and unmarshals the incoming event message, and then
// routes the message to the appropriate handler based on its protocol type.
func (e *Exit) processMessage(ctx context.Context, msg nostr.IncomingEvent) {
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
		e.handleSocks5ProxyMessage(msg, protocolMessage)
	}
}

// handleConnect handles the connection for the given message and protocol message.
// It locks the mutex for the protocol message key, encodes the receiver's profile,
// creates a new connection with the provided context and options, and establishes
// a connection to the backend host.
// If the connection cannot be established, it logs an error and returns.
// It then stores the connection in the nostrConnectionMap and creates two goroutines
// to proxy the data between the connection and the backend.
func (e *Exit) handleConnect(ctx context.Context, msg nostr.IncomingEvent, protocolMessage *protocol.Message, isTLS bool) {
	e.mutexMap.Lock(protocolMessage.Key.String())
	defer e.mutexMap.Unlock(protocolMessage.Key.String())
	receiver, err := nip19.EncodeProfile(msg.PubKey, []string{msg.Relay.String()})
	if err != nil {
		return
	}
	connection := netstr.NewConnection(
		ctx,
		netstr.WithPrivateKey(e.config.NostrPrivateKey),
		netstr.WithDst(receiver),
		netstr.WithUUID(protocolMessage.Key),
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

// handleSocks5ProxyMessage handles the SOCKS5 proxy message by writing it to the destination connection.
// If the destination connection does not exist, the function returns without doing anything.
//
// Parameters:
// - msg: The incoming event containing the SOCKS5 proxy message.
// - protocolMessage: The protocol message associated with the incoming event.
func (e *Exit) handleSocks5ProxyMessage(
	msg nostr.IncomingEvent,
	protocolMessage *protocol.Message,
) {
	e.mutexMap.Lock(protocolMessage.Key.String())
	defer e.mutexMap.Unlock(protocolMessage.Key.String())
	dst, ok := e.nostrConnectionMap.Load(protocolMessage.Key.String())
	if !ok {
		return
	}
	dst.WriteNostrEvent(msg)
}
