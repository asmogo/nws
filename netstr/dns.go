package netstr

import (
	"context"
	"net"
)

// NostrDNS does not resolve anything
type NostrDNS struct{}

func (d NostrDNS) Resolve(ctx context.Context, name string) (context.Context, net.IP, error) {
	return ctx, net.IP{0, 0, 0, 0}, nil
}
