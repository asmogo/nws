package netstr

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/asmogo/nws/protocol"
	"github.com/nbd-wtf/go-nostr"
)

// NostrDNS does not resolve anything.
type NostrDNS struct {
	pool        *nostr.SimplePool
	nostrRelays []string
}

var (
	errPoolIsNil                 = errors.New("pool is nil")
	errFailedToFindExitNodeEvent = errors.New("failed to find exit node event")
	errExitNodeEventIsExpired    = errors.New("exit node event is expired")
)

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
		return ctx, nil, fmt.Errorf("failed to resolve ip address: %w", err)
	}
	if d.pool == nil {
		return ctx, nil, errPoolIsNil
	}
	since := nostr.Timestamp(time.Now().Add(-time.Second * 10).Unix())
	ev := d.pool.QuerySingle(ctx, d.nostrRelays, nostr.Filter{
		Kinds: []int{protocol.KindAnnouncementEvent},
		Since: &since,
	})
	if ev == nil {
		return ctx, nil, errFailedToFindExitNodeEvent
	}
	if ev.CreatedAt < since {
		return ctx, nil, errExitNodeEventIsExpired
	}
	ctx = context.WithValue(ctx, TargetPublicKey, ev.PubKey)
	return ctx, addr.IP, nil
}

type ContextKeyTargetPublicKey string

const TargetPublicKey ContextKeyTargetPublicKey = "TargetPublicKey"
