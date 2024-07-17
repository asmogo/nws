package gw

import (
	"context"
	"fmt"
	"github.com/asmogo/nws/config"
	"github.com/asmogo/nws/protocol"
	"github.com/asmogo/nws/socks5"
	"github.com/nbd-wtf/go-nostr"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
)

type Proxy struct {
	config *config.ProxyConfig // the configuration for the gateway
	// a list of nostr relays to publish events to
	relays          []*nostr.Relay
	pool            *protocol.SimplePool
	exitNodeChannel chan protocol.IncomingEvent
	publicKey       string
	socksServer     *socks5.Server
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
	socksServer, err := socks5.New(&socks5.Config{
		AuthMethods: nil,
		Credentials: nil,
		Resolver:    NostrDNS{},
		Rules:       nil,
		Rewriter:    nil,
		BindIP:      net.IP{0, 0, 0, 0},
		Logger:      nil,
		Dial:        nil,
	}, s.pool, config.NostrPrivateKey)
	if err != nil {
		panic(err)
	}
	s.socksServer = socksServer
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

// Start should start the server
func (s *Proxy) Start() error {
	s.subscribe(s.config.NostrRelays)
	// check if the env variable http-proxy is set
	err := s.socksServer.ListenAndServe("tcp", "8882")
	if err != nil {
		panic(err)
	}
	return nil
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
