package exit

import (
	"bufio"
	"bytes"
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
	"net/http/httputil"
	_ "net/http/pprof"
	"strconv"
	"strings"
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
		{Kinds: []int{nostr.KindEncryptedDirectMessage},
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
	case protocol.MessageTypeHttp:
		e.handleHttpProxyMessage(ctx, msg, protocolMessage, sharedKey)
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

// deprecated
func (e *Exit) handleHttpProxyMessage(
	ctx context.Context,
	msg protocol.IncomingEvent,
	protocolMessage *protocol.Message,
	sharedKey []byte,
) {
	dst, ok := e.connectionMap.Load(protocolMessage.Key.String())
	if !ok {
		var err error
		dst, err = net.Dial("tcp", e.config.BackendHost)
		if err != nil {
			return
		}
		e.connectionMap.Store(protocolMessage.Key.String(), dst)
		go func() {
			// Use bufio to read the response
			reader := bufio.NewReader(dst)

			// Read the status line
			statusLine, err := reader.ReadString('\n')
			if err != nil {
				panic(err)
			}
			fmt.Println("Status Line:", statusLine)

			// Read and store the headers
			headersBuffer := &bytes.Buffer{}
			headers := make(http.Header)
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					panic(err)
				}
				headersBuffer.WriteString(line)
				if line == "\r\n" {
					break
				}
				parts := strings.SplitN(line, ": ", 2)
				if len(parts) == 2 {
					headers.Add(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
				}
			}
			fmt.Println("Headers:", headersBuffer.String())

			// Read the body
			var body bytes.Buffer
			// calculate content length from header
			contentLength, err := strconv.Atoi(headers.Get("Content-Length"))
			if err != nil {
				panic(err)
			}

			chunk := make([]byte, contentLength)
			n, err := reader.Read(chunk)
			body.Write(chunk[:n])

			if err != nil {
				panic(err)
			}
			e.publishResponse(ctx, msg, append([]byte(statusLine), append(headersBuffer.Bytes(), body.Bytes()...)...), protocolMessage)
		}()
	} else {
		slog.Info("reusing connection")
	}
	// parse message.data to http request
	reader := bufio.NewReader(bytes.NewReader(protocolMessage.Data))
	request, err := http.ReadRequest(reader)
	if err != nil {
		slog.Error("could not parse request", "error", err)
		return
	}
	request.Host = e.config.BackendHost
	request.URL.Host = request.Host
	request.URL.Scheme = e.config.BackendScheme
	request.RequestURI = ""
	request.Header.Del("Proxy-Connection")
	// dump the request as string
	dump, err := httputil.DumpRequest(request, true)
	if err != nil {
		slog.Error("could not dump request", "error", err)
		return
	}
	slog.Info("sending request", "request", string(dump))
	_, err = dst.Write(dump)
	if err != nil {
		slog.Error("error writing to pipe", "error", err)
		return
	}
}

// func (e *Exit)  messageWriter(ctx context.Context, dataChannel chan []byte, incomingMessage nostr.IncomingEvent)
func (e *Exit) publishResponse(ctx context.Context, msg protocol.IncomingEvent, message []byte, receivedMessage *protocol.Message) {
	signer, err := protocol.NewEventSigner(e.config.NostrPrivateKey)
	if err != nil {
		return
	}
	opts := []protocol.MessageOption{
		protocol.WithUUID(receivedMessage.Key),
		protocol.WithType(receivedMessage.Type),
		protocol.WithData(message),
	}
	ev, err := signer.CreateSignedEvent(msg.PubKey, nostr.Tags{nostr.Tag{"p", msg.PubKey}}, opts...)
	if err != nil {
		slog.Error("could not create event", "error", err)
		return
	}
	// publish the event to all relays
	for _, responseRelay := range e.relays {
		var relay *nostr.Relay
		relay, err = e.pool.EnsureRelay(responseRelay.URL)
		if err != nil {
			return
		}
		err = relay.Publish(ctx, ev)
		if err != nil {
			return
		}
	}
}
