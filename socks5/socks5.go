package socks5

import (
	"bufio"
	"fmt"
	"github.com/asmogo/nws/config"
	"github.com/nbd-wtf/go-nostr"
	"log"
	"net"
	"os"

	"context"
)

const (
	socks5Version = uint8(5)
)

// Config is used to setup and configure a Server
type Config struct {
	// AuthMethods can be provided to implement custom authentication
	// By default, "auth-less" mode is enabled.
	// For password-based auth use UserPassAuthenticator.
	AuthMethods []Authenticator

	// If provided, username/password authentication is enabled,
	// by appending a UserPassAuthenticator to AuthMethods. If not provided,
	// and AUthMethods is nil, then "auth-less" mode is enabled.
	Credentials CredentialStore

	// Resolver can be provided to do custom name resolution.
	// Defaults to DNSResolver if not provided.
	Resolver NameResolver

	// Rules is provided to enable custom logic around permitting
	// various commands. If not provided, PermitAll is used.
	Rules RuleSet

	// Rewriter can be used to transparently rewrite addresses.
	// This is invoked before the RuleSet is invoked.
	// Defaults to NoRewrite.
	Rewriter AddressRewriter

	// BindIP is used for bind or udp associate
	BindIP net.IP

	// Logger can be used to provide a custom log target.
	// Defaults to stdout.
	Logger *log.Logger

	// Optional function for dialing out
	Dial func(ctx context.Context, network, addr string) (net.Conn, error)

	entryConfig *config.EntryConfig
}

var ErrorNoServerAvailable = fmt.Errorf("no socks server available")

// Server is reponsible for accepting connections and handling
// the details of the SOCKS5 protocol
type Server struct {
	config      *Config
	authMethods map[uint8]Authenticator
	pool        *nostr.SimplePool
	tcpListener *TCPListener
}

// New creates a new Server and potentially returns an error
func New(conf *Config, pool *nostr.SimplePool, config *config.EntryConfig) (*Server, error) {
	// Ensure we have at least one authentication method enabled
	if len(conf.AuthMethods) == 0 {
		if conf.Credentials != nil {
			conf.AuthMethods = []Authenticator{&UserPassAuthenticator{conf.Credentials}}
		} else {
			conf.AuthMethods = []Authenticator{&NoAuthAuthenticator{}}
		}
	}

	// Ensure we have a DNS resolver
	if conf.Resolver == nil {
		conf.Resolver = DNSResolver{}
	}

	// Ensure we have a rule set
	if conf.Rules == nil {
		conf.Rules = PermitAll()
	}

	// Ensure we have a log target
	if conf.Logger == nil {
		conf.Logger = log.New(os.Stdout, "", log.LstdFlags)
	}
	if conf.entryConfig == nil {
		conf.entryConfig = config
	}

	server := &Server{
		config: conf,
		pool:   pool,
	}
	if conf.entryConfig.PublicAddress != "" {
		listener, err := NewTCPListener(conf.entryConfig.PublicAddress)
		if err != nil {
			return nil, err
		}
		go listener.Start()
		server.tcpListener = listener
	}
	server.authMethods = make(map[uint8]Authenticator)

	for _, a := range conf.AuthMethods {
		server.authMethods[a.GetCode()] = a
	}

	return server, nil
}
func (s *Server) Configuration() (*Config, error) {
	if s.config != nil {
		return s.config, nil
	}
	return nil, fmt.Errorf("socks: configuration not set yet")
}

// ListenAndServe is used to create a listener and serve on it
func (s *Server) ListenAndServe(network, port string) error {
	bind := net.JoinHostPort(s.config.BindIP.String(), port)
	l, err := net.Listen(network, bind)
	if err != nil {
		return err
	}
	return s.Serve(l)
}

// Serve is used to serve connections from a listener
func (s *Server) Serve(l net.Listener) error {
	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		go s.ServeConn(conn)
	}
	return nil
}

// GetAuthContext is used to retrieve the auth context from connection
func (s *Server) GetAuthContext(conn net.Conn, bufConn *bufio.Reader) (*AuthContext, error) {
	// Read the version byte
	version := []byte{0}
	if _, err := bufConn.Read(version); err != nil {
		s.config.Logger.Printf("[ERR] socks: Failed to get version byte: %v", err)
		return nil, err
	}

	// Ensure we are compatible
	if version[0] != socks5Version {
		err := fmt.Errorf("Unsupported SOCKS version: %v", version)
		s.config.Logger.Printf("[ERR] socks: %v", err)
		return nil, err
	}

	// Authenticate the connection
	authContext, err := s.authenticate(conn, bufConn)
	if err != nil {
		err = fmt.Errorf("Failed to authenticate: %v", err)
		s.config.Logger.Printf("[ERR] socks: %v", err)
		return nil, err
	}
	return authContext, nil
}

// GetRequest is used to retrieve Request from connection
func (s *Server) GetRequest(conn net.Conn, bufConn *bufio.Reader) (*Request, error) {
	request, err := NewRequest(bufConn)
	if err != nil {
		if err == unrecognizedAddrType {
			if err := SendReply(conn, addrTypeNotSupported, nil); err != nil {
				return nil, fmt.Errorf("Failed to send reply: %v", err)
			}
		}
		return nil, fmt.Errorf("Failed to read destination address: %v", err)
	}
	return request, nil
}

// ServeConn is used to serve a single connection.
func (s *Server) ServeConn(conn net.Conn) error {
	s.config.Logger.Print("[INFO] serving socks5 connection")
	defer conn.Close()
	bufConn := bufio.NewReader(conn)
	authContext, err := s.GetAuthContext(conn, bufConn)
	if err != nil {
		return err
	}
	request, err := s.GetRequest(conn, bufConn)
	if err != nil {
		return err
	}

	request.AuthContext = authContext
	if client, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		request.RemoteAddr = &AddrSpec{IP: client.IP, Port: client.Port}
	}
	s.config.Logger.Printf("[INFO] handling request from %s", request.RemoteAddr.IP)
	// Process the client request
	if err := s.handleRequest(request, conn); err != nil {
		err = fmt.Errorf("failed to handle request: %v", err)
		s.config.Logger.Printf("[ERR] socks: %v", err)
		return err
	}

	return nil
}
