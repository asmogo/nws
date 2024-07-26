package socks5

import (
	"github.com/google/uuid"
	"github.com/puzpuzpuz/xsync/v3"
	"log/slog"
	"net"
)

type TCPListener struct {
	listener        net.Listener
	connectChannels *xsync.MapOf[string, chan net.Conn] // todo -- use [16]byte for uuid instead of string
}

func NewTCPListener(address string) (*TCPListener, error) {
	l, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
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
func (l *TCPListener) handleConnection(conn net.Conn) {
	//defer conn.Close()
	for {
		// read uuid from the connection
		readbuffer := make([]byte, 36)

		_, err := conn.Read(readbuffer)
		if err != nil {
			return
		}

		// check if uuid is in the map
		ch, ok := l.connectChannels.Load(string(readbuffer))
		if !ok {
			slog.Error("uuid not found in map")
			return
		}
		// send the connection to the channel
		ch <- conn
		return
	}
}
