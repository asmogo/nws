package gw

import (
	"bufio"
	"context"
	"fmt"
	"github.com/asmogo/nws/config"
	"github.com/asmogo/nws/protocol"
	"github.com/asmogo/nws/socks5"
	"github.com/google/uuid"
	"github.com/nbd-wtf/go-nostr"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	_ "net/http/pprof"
	"net/url"
	"os"
)

type Proxy struct {
	config *config.ProxyConfig // the configuration for the gateway
	// a list of nostr relays to publish events to
	relays          []*nostr.Relay
	pool            *protocol.SimplePool
	exitNodeChannel chan protocol.IncomingEvent
	publicKey       string
}

func NewProxy(ctx context.Context, config *config.ProxyConfig) *Proxy {
	// we need a webserver to get the pprof webserver
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	s := &Proxy{
		config: config,
		pool:   protocol.NewSimplePool(ctx),
	}
	go func() {
		err := startSocks5(s.pool, config)
		if err != nil {
			slog.Error("error starting socks5", "err", err)
		}
	}()

	// publish the event to two relays
	for _, relayUrl := range config.NostrRelays {

		relay, err := s.pool.EnsureRelay(relayUrl)
		if err != nil {
			fmt.Println(err)
			continue
		}
		s.relays = append(s.relays, relay)
		fmt.Printf("added relay connection to %s\n", relayUrl)
	}
	pub, err := nostr.GetPublicKey(config.NostrPrivateKey)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	s.publicKey = pub

	return s
}

// NostrDNS does not resolve anything
type NostrDNS struct{}

func (d NostrDNS) Resolve(ctx context.Context, name string) (context.Context, net.IP, error) {
	return ctx, net.IP{0, 0, 0, 0}, nil
}

func startSocks5(pool *protocol.SimplePool, cfg *config.ProxyConfig) error {
	socksServer, err := socks5.New(&socks5.Config{
		AuthMethods: nil,
		Credentials: nil,
		Resolver:    NostrDNS{},
		Rules:       nil,
		Rewriter:    nil,
		BindIP:      net.IP{0, 0, 0, 0},
		Logger:      nil,
		Dial:        nil,
	}, pool, cfg.NostrPrivateKey)
	if err != nil {
		return err
	}
	err = socksServer.ListenAndServe("tcp", "8882")
	if err != nil {
		return err
	}
	return nil
}

// Start should start the server
func (s *Proxy) Start() error {
	s.subscribe(s.config.NostrRelays)
	// check if the env variable http-proxy is set
	port, err := getListeningPort()
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", port)
	if err != nil {
		return err
	}
	for {
		var conn net.Conn
		conn, err = ln.Accept()
		if err != nil {
			// handle error
			slog.Error("error accepting connection", err)
			continue
		}
		go s.handleConnection(conn.(*net.TCPConn))
	}
}

func getListeningPort() (string, error) {
	port := ":8881"
	if os.Getenv("http-proxy") != "" {
		proxyUrl, err := url.Parse(os.Getenv("http-proxy"))
		if err != nil {
			return "", err
		}
		// get th port from the proxy url
		port = ":" + proxyUrl.Port()

	}
	return port, nil
}

func (s *Proxy) subscribe(relays []string) {
	now := nostr.Now()
	incomingEventChannel := s.pool.SubMany(context.Background(), relays,
		nostr.Filters{
			{
				Kinds: []int{nostr.KindEncryptedDirectMessage},
				Since: &now,
				Tags: nostr.TagMap{
					"p": []string{s.publicKey},
				},
			},
		})
	s.exitNodeChannel = incomingEventChannel
}

func (s *Proxy) handleConnection(conn *net.TCPConn) {
	defer conn.Close()

	ctx := context.Background()
	signer, err := protocol.NewEventSigner(s.config.NostrPrivateKey)
	if err != nil {
		return
	}
	sessionID := uuid.New()
	outChannel := make(chan nostr.Event)
	// create a close channel
	go signer.DecryptAndWrite(ctx, s.exitNodeChannel, conn, outChannel, sessionID)
	for {
		select {
		case <-ctx.Done():
			slog.Info("context done")
			return
		default:
			reader := bufio.NewReader(conn)
			// use http.ReadRequest to read http request
			req, err := http.ReadRequest(reader)
			if err != nil {
				slog.Error("error parsing http request", err)
				return
			}
			// read the host from the request
			host := req.Host
			// parse the host using nip19
			publicKey, relays, err := socks5.ParseDestination(host)
			if err != nil {
				slog.Error("error parsing host", err)
				return
			}
			if len(relays) == 0 {
				relays = s.config.NostrRelays
			}
			var ev nostr.Event
			// dump the request as string
			var requestData []byte
			requestData, err = httputil.DumpRequest(req, true)
			if err != nil {
				slog.Error("error dumping request", err)
				return
			}
			opts := []protocol.MessageOption{
				protocol.WithUUID(sessionID),
				protocol.WithType(protocol.MessageTypeHttp),
				protocol.WithData(requestData),
			}
			ev, err = signer.CreateSignedEvent(publicKey, nostr.Tags{nostr.Tag{"p", publicKey}}, opts...)
			if err != nil {
				return
			}
			// publish the event to all relays
			err = s.RunOnRelays(relays, func(relay *nostr.Relay, ctx context.Context) error {
				return relay.Publish(ctx, ev)
			}, ctx)
			if err != nil {
				slog.Error("error publishing event", err)
				return
			}
			slog.Info("sent request")
		}
	}
}

func (s *Proxy) RunOnRelays(relays []string, fn func(*nostr.Relay, context.Context) error, ctx context.Context) error {
	s.subscribe(relays)
	nostrRelays := make([]*nostr.Relay, 0)
	for _, relayUrl := range relays {
		relay, err := s.pool.EnsureRelay(relayUrl)
		if err != nil {
			slog.Error("error creating relay", err)
			continue
		}
		nostrRelays = append(nostrRelays, relay)
	}
	return RunOnAllRelays(nostrRelays, fn, ctx)
}

func RunOnAllRelays(relays []*nostr.Relay, fn func(*nostr.Relay, context.Context) error, ctx context.Context) error {
	for _, relay := range relays {
		if err := fn(relay, ctx); err != nil {
			slog.Error("error running on relay", err)
			continue
		}
	}
	return nil
}
