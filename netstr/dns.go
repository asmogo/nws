package netstr

import (
	"context"
	"fmt"
	"github.com/nbd-wtf/go-nostr"
	"net"
	"strings"
	"time"
)

// NostrDNS does not resolve anything
type NostrDNS struct {
	pool        *nostr.SimplePool
	nostrRelays []string
}

func NewNostrDNS(pool *nostr.SimplePool, nostrRelays []string) *NostrDNS {
	return &NostrDNS{
		pool:        pool,
		nostrRelays: nostrRelays,
	}
}

func (d NostrDNS) Resolve(ctx context.Context, name string) (context.Context, net.IP, error) {
	if strings.HasSuffix(name, ".nostr") || strings.HasPrefix(name, "npub") || strings.HasPrefix(name, "nprofile") {

		return ctx, nil, nil
	}
	addr, err := net.ResolveIPAddr("ip", name)
	if err != nil {
		return ctx, nil, err
	}
	if d.pool == nil {
		return ctx, nil, fmt.Errorf("pool is nil")
	}
	since := nostr.Timestamp(time.Now().Add(-time.Second * 10).Unix())
	ev := d.pool.QuerySingle(ctx, d.nostrRelays, nostr.Filter{
		Kinds: []int{nostr.KindTextNote},
		Since: &since,
		Tags:  nostr.TagMap{"n": []string{"nws"}},
	})
	if ev == nil {
		return ctx, nil, fmt.Errorf("failed to find exit node event")
	}
	if ev.CreatedAt < since {
		return ctx, nil, fmt.Errorf("exit node event is expired")
	}
	ctx = context.WithValue(ctx, "publicKey", ev.PubKey)
	return ctx, addr.IP, err
}
