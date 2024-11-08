package exit

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/asmogo/nws/protocol"
	"github.com/ekzyis/nip44"
	"github.com/nbd-wtf/go-nostr"
)

const (
	headerTimeout = 5 * time.Second
)

var (
	errNoCertificateEvent = errors.New("failed to find encrypted direct message")
)

func (e *Exit) StartReverseProxy(ctx context.Context, httpTarget string, port int32) error {
	incomingEvent := e.pool.QuerySingle(ctx, e.config.NostrRelays, nostr.Filter{
		Authors: []string{e.publicKey},
		Kinds:   []int{protocol.KindCertificateEvent},
		Tags:    nostr.TagMap{"p": []string{e.publicKey}},
	})
	var cert tls.Certificate
	var err error
	if incomingEvent == nil {
		certificate, err := e.createAndStoreCertificateData(ctx)
		if err != nil {
			return err
		}
		cert = *certificate
	} else {
		cert, err = e.handleCertificateEvent(incomingEvent, ctx, cert)
		if err != nil {
			return err
		}
	}
	target, _ := url.Parse(httpTarget)

	httpsConfig := &http.Server{
		ReadHeaderTimeout: headerTimeout,
		Addr:              fmt.Sprintf(":%d", port),
		TLSConfig:         &tls.Config{Certificates: []tls.Certificate{cert}},
		Handler:           http.HandlerFunc(httputil.NewSingleHostReverseProxy(target).ServeHTTP),
	}
	return httpsConfig.ListenAndServeTLS("", "")

}

func (e *Exit) handleCertificateEvent(
	incomingEvent *nostr.IncomingEvent,
	ctx context.Context,
	cert tls.Certificate,
) (tls.Certificate, error) {
	slog.Info("found certificate event", "certificate", incomingEvent.Content)
	// load private key from file
	privateKeyEvent := e.pool.QuerySingle(ctx, e.config.NostrRelays, nostr.Filter{
		Authors: []string{e.publicKey},
		Kinds:   []int{protocol.KindPrivateKeyEvent},
		Tags:    nostr.TagMap{"p": []string{e.publicKey}},
	})
	if privateKeyEvent == nil {
		return tls.Certificate{}, errNoCertificateEvent
	}
	privateKeyBytes, targetPublicKeyBytes, err := protocol.GetEncryptionKeys(e.config.NostrPrivateKey, privateKeyEvent.PubKey)
	if err != nil {
		return tls.Certificate{}, err
	}
	sharedKey, err := nip44.GenerateConversationKey(privateKeyBytes, targetPublicKeyBytes)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to compute shared key: %w", err)
	}
	decodedMessage, err := nip44.Decrypt(sharedKey, privateKeyEvent.Content)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to decrypt private key: %w", err)
	}
	message, err := protocol.UnmarshalJSON([]byte(decodedMessage))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to unmarshal message: %w", err)
	}
	block, _ := pem.Decode(message.Data)
	if block == nil {
		_, err = fmt.Fprintf(os.Stderr, "error: failed to decode PEM block containing private key\n")
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("failed to write error: %w", err)
		}
		os.Exit(1)
	}

	if got, want := block.Type, "RSA PRIVATE KEY"; got != want {
		_, err = fmt.Fprintf(os.Stderr, "error: decoded PEM block of type %s, but wanted %s", got, want)
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("failed to write error: %w", err)
		}
		os.Exit(1)
	}

	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to parse private key: %w", err)
	}
	certBlock, _ := pem.Decode([]byte(incomingEvent.Content))
	if certBlock == nil {
		_, err = fmt.Fprintf(os.Stderr, "Failed to parse certificate PEM.")
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("failed to write error: %w", err)
		}
		os.Exit(1)
	}

	parsedCert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to parse certificate: %w", err)
	}
	cert = tls.Certificate{
		Certificate: [][]byte{certBlock.Bytes},
		PrivateKey:  priv,
		Leaf:        parsedCert,
	}
	return cert, nil
}

const (
	tenYears = 0 * 365 * 24 * time.Hour
	keySize  = 2048
	limit    = 128
	chmod    = 0644
)

func (e *Exit) createAndStoreCertificateData(ctx context.Context) (*tls.Certificate, error) {
	priv, _ := rsa.GenerateKey(rand.Reader, keySize)
	notBefore := time.Now()
	notAfter := notBefore.Add(tenYears)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), limit)
	serialNumber, _ := rand.Int(rand.Reader, serialNumberLimit)
	domain, _ := e.getDomain()

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"NWS"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{domain},
	}

	certBytes, _ := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	// save key pem to file
	err := os.WriteFile(fmt.Sprintf("%s.key", e.publicKey), keyPEM, chmod)
	if err != nil {
		return nil, err
	}
	cert, _ := tls.X509KeyPair(certPEM, keyPEM)
	certificate, err := e.storeCertificate(ctx, certPEM)
	if err != nil {
		return certificate, err
	}
	err = e.storePrivateKey(ctx, keyPEM)
	if err != nil {
		return certificate, err
	}
	return &cert, nil
}

func (e *Exit) storePrivateKey(ctx context.Context, keyPEM []byte) error {
	s, err := protocol.NewEventSigner(e.config.NostrPrivateKey)
	if err != nil {
		return err
	}
	event, err := s.CreateSignedEvent(e.publicKey, protocol.KindPrivateKeyEvent, nostr.Tags{
		nostr.Tag{"p", e.publicKey},
	}, protocol.WithData(keyPEM))
	if err != nil {
		return err
	}
	for _, responseRelay := range e.config.NostrRelays {
		var relay *nostr.Relay
		relay, err = e.pool.EnsureRelay(responseRelay)
		if err != nil {
			return err
		}
		err = relay.Publish(ctx, event)
		if err != nil {
			return err
		}
	}
	return nil
}
func (e *Exit) storeCertificate(ctx context.Context, certPEM []byte) (*tls.Certificate, error) {
	event := nostr.Event{
		CreatedAt: nostr.Now(),
		PubKey:    e.publicKey,
		Kind:      protocol.KindCertificateEvent,
		Content:   string(certPEM),
		Tags: nostr.Tags{
			nostr.Tag{"p", e.publicKey},
		},
	}
	err := event.Sign(e.config.NostrPrivateKey)
	if err != nil {
		return nil, err
	}
	for _, responseRelay := range e.config.NostrRelays {
		var relay *nostr.Relay
		relay, err = e.pool.EnsureRelay(responseRelay)
		if err != nil {
			return nil, err
		}
		err = relay.Publish(ctx, event)
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}
