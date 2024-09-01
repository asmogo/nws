package proxy

import (
	"context"
	"net"

	"github.com/asmogo/nws/config"
	"github.com/asmogo/nws/netstr"
	"github.com/asmogo/nws/socks5"
	"github.com/nbd-wtf/go-nostr"
)

type Proxy struct {
	config *config.EntryConfig // the configuration for the gateway
	// a list of nostr relays to publish events to
	pool        *nostr.SimplePool
	socksServer *socks5.Server
}

func New(ctx context.Context, config *config.EntryConfig) *Proxy {
	proxy := &Proxy{
		config: config,
		pool:   nostr.NewSimplePool(ctx),
	}
	socksServer, err := socks5.New(&socks5.Config{
		Resolver: netstr.NewNostrDNS(proxy.pool, config.NostrRelays),
		BindIP:   net.IP{0, 0, 0, 0},
	}, proxy.pool, config)
	if err != nil {
		panic(err)
	}
	proxy.socksServer = socksServer
	return proxy
}

// Start should start the server
func (s *Proxy) Start() error {
	err := s.socksServer.ListenAndServe("tcp", "8882")
	if err != nil {
		panic(err)
	}
	return nil
}
