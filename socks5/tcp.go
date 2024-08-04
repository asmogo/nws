package socks5

import (
	"fmt"
	"log/slog"
	"net"

	"github.com/google/uuid"
	"github.com/puzpuzpuz/xsync/v3"
)

type TCPListener struct {
	listener        net.Listener
	connectChannels *xsync.MapOf[string, chan net.Conn] // todo -- use [16]byte for uuid instead of string
}

const uuidLength = 36

func NewTCPListener(address string) (*TCPListener, error) {
	l, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to create tcp listener: %w", err)
	}
	return &TCPListener{
		listener:        l,
		connectChannels: xsync.NewMapOf[string, chan net.Conn](),
	}, nil
}

func (l *TCPListener) AddConnectChannel(uuid uuid.UUID, ch chan net.Conn) {
	l.connectChannels.Store(uuid.String(), ch)
}

// Start starts the listener
func (l *TCPListener) Start() {
	for {
		conn, err := l.listener.Accept()
		if err != nil {
			return
		}
		go l.handleConnection(conn)
	}
}

// handleConnection handles the connection
// It reads the uuid from the connection, checks if the uuid is in the map, and sends the connection to the channel
// It does not close the connection
func (l *TCPListener) handleConnection(conn net.Conn) {
	response := []byte{1}
	for {
		// read uuid from the connection
		readbuffer := make([]byte, uuidLength)
		_, err := conn.Read(readbuffer)
		if err != nil {
			return
		}
		// check if uuid is in the map
		connectionID := string(readbuffer)
		connChannel, ok := l.connectChannels.Load(connectionID)
		if !ok {
			slog.Error("uuid not found in map")
			continue
		}
		slog.Info("uuid found in map")
		l.connectChannels.Delete(connectionID)
		_, err = conn.Write(response)
		if err != nil {
			close(connChannel) // close the channel
			slog.Error("failed to write response to connection", "err", err)
			return
		}
		// send the connection to the channel
		connChannel <- conn
		return
	}
}
