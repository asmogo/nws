package netstr

import (
	"context"
	"fmt"
	"github.com/nbd-wtf/go-nostr"
	"net"
	"strings"
)

// NostrDNS does not resolve anything
type NostrDNS struct {
	Pool        *nostr.SimplePool
	NostrRelays []string
}

func (d NostrDNS) Resolve(ctx context.Context, name string) (context.Context, net.IP, error) {
	if strings.HasSuffix(name, ".nostr") || strings.HasPrefix(name, "npub") || strings.HasPrefix(name, "nprofile") {

		return ctx, nil, nil
	}
	addr, err := net.ResolveIPAddr("ip", name)
	if err != nil {
		return ctx, nil, err
	}
	ev := d.Pool.QuerySingle(ctx, d.NostrRelays, nostr.Filter{
		Tags: nostr.TagMap{"n": []string{"nws"}},
	})
	if ev == nil {
		return ctx, nil, fmt.Errorf("failed to find exit node event")
	}
	ctx = context.WithValue(ctx, "publicKey", ev.PubKey)
	return ctx, addr.IP, err
}
