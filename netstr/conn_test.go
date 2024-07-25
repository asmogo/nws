package netstr

import (
	"context"
	"github.com/nbd-wtf/go-nostr"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNostrConnection_Read(t *testing.T) {
	tests := []struct {
		name    string
		event   nostr.IncomingEvent
		nc      func() *NostrConnection
		wantN   int
		wantErr bool
	}{
		{
			name:  "Read invalid relay",
			event: nostr.IncomingEvent{Relay: nil},
			nc: func() *NostrConnection {
				ctx, cancelFunc := context.WithCancel(context.Background())
				return &NostrConnection{
					uuid:             uuid.New(),
					ctx:              ctx,
					cancel:           cancelFunc,
					subscriptionChan: make(chan nostr.IncomingEvent, 1),
					privateKey:       "788de536151854213cc28dff9c3042e7897f0a1d59b391ddbbc1619d7e716e78",
				}
			},
			wantN:   0,
			wantErr: false,
		},
		{
			name: "Read",
			event: nostr.IncomingEvent{
				Relay: &nostr.Relay{URL: "wss://relay.example.com"},
				Event: &nostr.Event{
					ID:      "eventID",
					PubKey:  "8f97a664471f0b6d599a1e4a781c9a25f39902d96fb462c08df48697bb851611",
					Content: "BnHzzyrUhKjDcDPOGfXJDYijUsgxw0hUZq2m+bX5QFI=?iv=NrEqv/jL+SASB2YTjo9i9Q=="}},
			nc: func() *NostrConnection {
				ctx, cancelFunc := context.WithCancel(context.Background())
				return &NostrConnection{
					uuid:             uuid.New(),
					ctx:              ctx,
					cancel:           cancelFunc,
					subscriptionChan: make(chan nostr.IncomingEvent, 1),
					privateKey:       "788de536151854213cc28dff9c3042e7897f0a1d59b391ddbbc1619d7e716e78",
				}
			},
			wantN:   11, // hello world
			wantErr: false,
		},
		// Add more cases here to cover more corner situations
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nc := tt.nc()
			defer nc.Close()
			b := make([]byte, 1024)
			nc.subscriptionChan <- tt.event
			gotN, err := nc.Read(b)
			if (err != nil) != tt.wantErr {
				t.Errorf("Read() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotN != tt.wantN {
				t.Errorf("Read() gotN = %v, want %v", gotN, tt.wantN)
			}
		})
	}
	func() {
		// Prevent goroutine leak
		for range make([]struct{}, 1000) {
			runtime.Gosched()
		}
	}()
}

func TestNewConnection(t *testing.T) {
	testCases := []struct {
		name       string
		opts       []NostrConnOption
		expectedID string
	}{
		{
			name: "NoOptions",
		},
		{
			name: "WithPrivateKey",
			opts: []NostrConnOption{WithPrivateKey("privateKey")},
		},
		{
			name: "WithSub",
			opts: []NostrConnOption{WithSub(true)},
		},
		{
			name: "WithDst",
			opts: []NostrConnOption{WithDst("destination")},
		},
		{
			name: "WithUUID",
			opts: []NostrConnOption{WithUUID(uuid.New())},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			connection := NewConnection(ctx, tc.opts...)

			assert.NotNil(t, connection)
			assert.NotNil(t, connection.pool)
			assert.NotNil(t, connection.ctx)
			assert.NotNil(t, connection.cancel)
			assert.NotNil(t, connection.subscriptionChan)

		})
	}
}
