package proxy

import (
	"context"
	"fmt"
	"github.com/asmogo/nws/config"
	"github.com/asmogo/nws/netstr"
	"github.com/asmogo/nws/socks5"
	"github.com/nbd-wtf/go-nostr"
	"net"
)

type Proxy struct {
	config *config.EntryConfig // the configuration for the gateway
	// a list of nostr relays to publish events to
	relays      []*nostr.Relay // deprecated -- should be used for default relay configuration
	pool        *nostr.SimplePool
	socksServer *socks5.Server
}

func New(ctx context.Context, config *config.EntryConfig) *Proxy {
	s := &Proxy{
		config: config,
		pool:   nostr.NewSimplePool(ctx),
	}
	socksServer, err := socks5.New(&socks5.Config{
		AuthMethods: nil,
		Credentials: nil,
		Resolver:    netstr.NostrDNS{},
		Rules:       nil,
		Rewriter:    nil,
		BindIP:      net.IP{0, 0, 0, 0},
		Logger:      nil,
		Dial:        nil,
	}, s.pool, config)
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
	return s
}

// Start should start the server
func (s *Proxy) Start() error {
	err := s.socksServer.ListenAndServe("tcp", "8882")
	if err != nil {
		panic(err)
	}
	return nil
}
