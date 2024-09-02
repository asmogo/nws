package netstr

import (
	"context"
	"fmt"
	"github.com/asmogo/nws/protocol"
	"github.com/ekzyis/nip44"
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
					Content: `AuaBj8mXZ9n9IfdonNra0lpaed6Alc+H0xjUdyN9h6mCSuy7ZrEjWUZQj4HWNd4P1RCme1pda0z8hyItT4nVzESByRiQT5+hf+ij0aJw9+DW/ggJIWGbpm4wp7bk4loYKdERr+nzorqEjWNzpxsJXhXJ0nKtIxu61To5XY4SjuMqpUuOtznuHiPJJhKNWSSRPV92L/iVoOnjKJhfR5jOWBK3vA==`}},
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
			wantN:   5, // hello world
			wantErr: false,
		},
		// Add more cases here to cover more corner situations
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nc := tt.nc()
			defer nc.Close()
			b := make([]byte, 1024)
			if tt.event.Event != nil {
				private, public, err := protocol.GetEncryptionKeys(nc.privateKey, tt.event.PubKey)
				if err != nil {
					panic(err)
				}
				sharedKey, err := nip44.GenerateConversationKey(private, public)
				if err != nil {
					panic(err)
				}
				fmt.Println(nip44.Encrypt(sharedKey, tt.event.Content, &nip44.EncryptOptions{}))
			}
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
