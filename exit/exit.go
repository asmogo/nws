package exit

import (
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"strings"

	"github.com/asmogo/nws/config"
	"github.com/asmogo/nws/netstr"
	"github.com/asmogo/nws/protocol"
	"github.com/asmogo/nws/socks5"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/ekzyis/nip44"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/puzpuzpuz/xsync/v3"
	"golang.org/x/net/context"
)

const (
	startingReverseProxyMessage = "starting exit node with https reverse proxy"
	generateKeyMessage          = "Generated new private key. Please set your environment using the new key, otherwise your key will be lost." //nolint: lll
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
	// mutexMap is a field in the Exit struct  used for synchronizing access to resources based on a string key.
	mutexMap *MutexMap
	// incomingChannel represents a channel used to receive incoming events from relays.
	incomingChannel chan nostr.IncomingEvent
	nprofile        string
	publicKey       string
}

func New(ctx context.Context, exitNodeConfig *config.ExitConfig) *Exit {
	// Generate new private key if needed
	generatePrivateKeyIfNeeded(exitNodeConfig)

	// Create a new exit node
	exit, err := createExitNode(ctx, exitNodeConfig)
	if err != nil {
		panic(err)
	}

	// Setup reverse proxy if HTTPS port is set
	setupReverseProxy(ctx, exit, exitNodeConfig)

	// Add relays to the pool
	addRelaysToPool(exit, exitNodeConfig.NostrRelays)

	if err := exit.setSubscriptions(ctx); err != nil {
		panic(err)
	}

	if err := exit.announceExitNode(ctx); err != nil {
		slog.Error("failed to announce exit node", "error", err)
	}

	printExitNodeInfo(exit, exitNodeConfig)

	return exit
}

func printExitNodeInfo(exit *Exit, exitNodeConfig *config.ExitConfig) {
	// Set up remaining steps for the exit node
	domain, err := exit.getDomain()
	if err != nil {
		panic(err)
	}
	slog.Info("created exit node", "profile", exitNodeConfig.NostrRelays, "domain", domain)
}

func newExit(pool *nostr.SimplePool, pubKey string, profile string) *Exit {
	exit := &Exit{
		nostrConnectionMap: xsync.NewMapOf[string, *netstr.NostrConnection](),
		pool:               pool,
		mutexMap:           NewMutexMap(),
		publicKey:          pubKey,
		nprofile:           profile,
	}
	return exit
}

func generatePrivateKeyIfNeeded(cfg *config.ExitConfig) {
	if cfg.NostrPrivateKey == "" {
		cfg.NostrPrivateKey = nostr.GeneratePrivateKey()
		slog.Warn(generateKeyMessage, "key", cfg.NostrPrivateKey)
	}
}

func createExitNode(ctx context.Context, cfg *config.ExitConfig) (*Exit, error) {
	pubKey, err := nostr.GetPublicKey(cfg.NostrPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get public key: %w", err)
	}
	slog.Info("using public key", "key", pubKey)

	profile, err := nip19.EncodeProfile(pubKey, cfg.NostrRelays)
	if err != nil {
		return nil, fmt.Errorf("failed to encode profile: %w", err)
	}

	pool := nostr.NewSimplePool(ctx)
	exit := newExit(pool, pubKey, profile)
	exit.config = cfg

	return exit, nil
}

func setupReverseProxy(ctx context.Context, exit *Exit, cfg *config.ExitConfig) {
	if cfg.HttpsPort != 0 {
		cfg.BackendHost = fmt.Sprintf(":%d", cfg.HttpsPort)
		go func(ctx context.Context, cfg *config.ExitConfig) {
			slog.Info(startingReverseProxyMessage, "port", cfg.HttpsPort)
			err := exit.StartReverseProxy(ctx, cfg.HttpsTarget, cfg.HttpsPort)
			if err != nil {
				panic(err)
			}
		}(ctx, cfg)
	}
}

func addRelaysToPool(exit *Exit, relays []string) {
	for _, relayURL := range relays {
		relay, err := exit.pool.EnsureRelay(relayURL)
		if err != nil {
			slog.Error("failed to ensure relay", "url", relayURL, "error", err)
			continue
		}
		exit.relays = append(exit.relays, relay)
		slog.Info("added relay connection", "url", relayURL)
	}
}

// getDomain returns the domain string used by the Exit node for communication with the Nostr relays.
// It concatenates the relay URLs using base32 encoding with no padding, separated by dots.
// The domain is then appended with the base32 encoded public key obtained using the configured Nostr private key.
// The final domain string is converted to lowercase and returned.
func (e *Exit) getDomain() (string, error) {
	var domain string
	// first lets build the subdomains
	for _, relayURL := range e.config.NostrRelays {
		if domain == "" {
			domain = base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(relayURL))
		} else {
			domain = fmt.Sprintf("%s.%s",
				domain, base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(relayURL)))
		}
	}
	// create base32 encoded public key
	decoded, err := GetPublicKeyBase32(e.config.NostrPrivateKey)
	if err != nil {
		return "", err
	}
	// use public key as host. add TLD
	domain = strings.ToLower(fmt.Sprintf("%s.%s.nostr", domain, decoded))
	return domain, nil
}

// GetPublicKeyBase32 decodes the private key string from hexadecimal format
// and returns the base32 encoded public key obtained using the provided private key.
// The base32 encoding has no padding. If there is an error decoding the private key
// or generating the public key, an error is returned.
func GetPublicKeyBase32(sk string) (string, error) {
	b, err := hex.DecodeString(sk)
	if err != nil {
		return "", fmt.Errorf("failed to decode private key: %w", err)
	}
	_, pk := btcec.PrivKeyFromBytes(b)
	return base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString(schnorr.SerializePubKey(pk)), nil
}

// setSubscriptions sets up subscriptions for the Exit node to receive incoming events from the specified relays.
// It first obtains the public key using the configured Nostr private key.
// Then it calls the `handleSubscription` method to open a subscription to the relays with the specified filters.
// This method runs in a separate goroutine and continuously handles the incoming events by calling `processMessage`
// If the context is canceled before the subscription is established, it returns the context error.
// If any errors occur during the process, they are returned.
// This method should be called once when starting the Exit node.
func (e *Exit) setSubscriptions(ctx context.Context) error {
	pubKey, err := nostr.GetPublicKey(e.config.NostrPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to get public key: %w", err)
	}
	if err = e.handleSubscription(ctx, pubKey, nostr.Now()); err != nil {
		return fmt.Errorf("failed to handle subscription: %w", err)
	}
	return nil
}

// handleSubscription handles the subscription to incoming events from relays based on the provided filters.
// It sets up the incoming event channel and starts a goroutine to handle the events.
// It returns an error if there is any issue with the subscription.
func (e *Exit) handleSubscription(ctx context.Context, pubKey string, since nostr.Timestamp) error {
	incomingEventChannel := e.pool.SubMany(ctx, e.config.NostrRelays, nostr.Filters{
		{
			Kinds: []int{protocol.KindEphemeralEvent},
			Since: &since,
			Tags: nostr.TagMap{
				"p": []string{pubKey},
			},
		},
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
	// hex decode the target public key
	privateKeyBytes, targetPublicKeyBytes, err := protocol.GetEncryptionKeys(e.config.NostrPrivateKey, msg.PubKey)
	if err != nil {
		slog.Error("could not get encryption keys", "error", err)
		return
	}
	sharedKey, err := nip44.GenerateConversationKey(privateKeyBytes, targetPublicKeyBytes)
	if err != nil {
		slog.Error("could not compute shared key", "error", err)
		return
	}
	decodedMessage, err := nip44.Decrypt(sharedKey, msg.Content)
	if err != nil {
		slog.Error("could not decrypt message", "error", err)
		return
	}
	protocolMessage, err := protocol.UnmarshalJSON([]byte(decodedMessage))
	if err != nil {
		slog.Error("could not unmarshal message", "error", err)
		return
	}
	destination, err := protocol.Parse(protocolMessage.Destination)
	if err != nil {
		slog.Error("could not parse destination", "error", err)
		return
	}
	if destination.TLD == "nostr" {
		protocolMessage.Destination = e.config.BackendHost
	}
	switch protocolMessage.Type {
	case protocol.MessageConnect:
		e.handleConnect(ctx, msg, protocolMessage)
	case protocol.MessageConnectReverse:
		e.handleConnectReverse(protocolMessage)
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
func (e *Exit) handleConnect(
	ctx context.Context,
	msg nostr.IncomingEvent,
	protocolMessage *protocol.Message,
) {
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
	dst, err = net.Dial("tcp", protocolMessage.Destination)
	if err != nil {
		slog.Error("could not connect to backend", "error", err)
		err = connection.Close()
		if err != nil {
			slog.Error("could not close connection", "error", err)
		}
		return
	}

	e.nostrConnectionMap.Store(protocolMessage.Key.String(), connection)
	slog.Info("connected to backend", "key", protocolMessage.Key)
	go socks5.Proxy(dst, connection, nil)
	go socks5.Proxy(connection, dst, nil)
}

func (e *Exit) handleConnectReverse(protocolMessage *protocol.Message) {
	e.mutexMap.Lock(protocolMessage.Key.String())
	defer e.mutexMap.Unlock(protocolMessage.Key.String())
	connection, err := net.Dial("tcp", protocolMessage.EntryPublicAddress)
	if err != nil {
		slog.Error("could not connect to entry", "error", err)
		return
	}

	_, err = connection.Write([]byte(protocolMessage.Key.String()))
	if err != nil {
		return
	}
	// read single byte from the connection
	readbuffer := make([]byte, 1)
	_, err = connection.Read(readbuffer)
	if err != nil {
		slog.Error("could not read from connection", "error", err)
		return
	}
	if readbuffer[0] != 1 {
		return
	}
	var dst net.Conn
	dst, err = net.Dial("tcp", protocolMessage.Destination)
	if err != nil {
		slog.Error("could not connect to backend", "error", err)
		connection.Close()
		return
	}
	slog.Info("connected to entry", "key", protocolMessage.Key)
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
	slog.Info("wrote event to backend", "key", protocolMessage.Key)
}
