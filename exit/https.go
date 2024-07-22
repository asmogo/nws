package exit

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"github.com/nbd-wtf/go-nostr"
	"math/big"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"
)

func (e *Exit) DeleteEvent(ctx context.Context, ev *nostr.Event) error {
	for _, responseRelay := range e.config.NostrRelays {
		var relay *nostr.Relay
		relay, err := e.pool.EnsureRelay(responseRelay)
		if err != nil {
			return err
		}
		event := nostr.Event{
			CreatedAt: nostr.Now(),
			PubKey:    e.publicKey,
			Kind:      nostr.KindDeletion,
			Tags: nostr.Tags{
				nostr.Tag{"e", ev.ID},
			},
		}
		err = event.Sign(e.config.NostrPrivateKey)
		err = relay.Publish(ctx, event)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *Exit) StartReverseProxy(httpTarget string, port int32) error {
	ctx := context.Background()
	ev := e.pool.QuerySingle(ctx, e.config.NostrRelays, nostr.Filter{
		Authors: []string{e.publicKey},
		Kinds:   []int{nostr.KindTextNote},
		Tags:    nostr.TagMap{"p": []string{e.nprofile}},
	})

	var cert tls.Certificate
	if ev == nil {
		certificate, err := e.createAndStoreCertificateData(ctx)
		if err != nil {
			return err
		}
		cert = *certificate
	} else {
		// load private key from file
		keyData, err := os.ReadFile(fmt.Sprintf("%s.key", e.nprofile))
		if err != nil {
			return err
		}
		block, _ := pem.Decode(keyData)
		if block == nil {
			fmt.Fprintf(os.Stderr, "error: failed to decode PEM block containing private key\n")
			os.Exit(1)
		}

		if got, want := block.Type, "RSA PRIVATE KEY"; got != want {
			fmt.Fprintf(os.Stderr, "error: decoded PEM block of type %s, but wanted %s", got, want)
			os.Exit(1)
		}

		priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return err
		}
		certBlock, _ := pem.Decode([]byte(ev.Content))
		if certBlock == nil {
			fmt.Fprintf(os.Stderr, "Failed to parse certificate PEM.")
			os.Exit(1)
		}

		parsedCert, err := x509.ParseCertificate(certBlock.Bytes)
		if err != nil {
			return err
		}
		cert = tls.Certificate{
			Certificate: [][]byte{certBlock.Bytes},
			PrivateKey:  priv,
			Leaf:        parsedCert,
		}
	}
	target, _ := url.Parse(httpTarget)

	httpsConfig := &http.Server{
		Addr:      fmt.Sprintf(":%d", port),
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
		Handler:   http.HandlerFunc(httputil.NewSingleHostReverseProxy(target).ServeHTTP),
	}
	return httpsConfig.ListenAndServeTLS("", "")

}

func (e *Exit) handleRequest(w http.ResponseWriter, r *http.Request) {

}
func (e *Exit) createAndStoreCertificateData(ctx context.Context) (*tls.Certificate, error) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, _ := rand.Int(rand.Reader, serialNumberLimit)

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Acme Co"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certBytes, _ := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	// save key pem to file
	err := os.WriteFile(fmt.Sprintf("%s.key", e.nprofile), keyPEM, 0644)
	if err != nil {
		return nil, err
	}
	cert, _ := tls.X509KeyPair(certPEM, keyPEM)
	event := nostr.Event{
		CreatedAt: nostr.Now(),
		PubKey:    e.publicKey,
		Kind:      nostr.KindTextNote,
		Content:   string(certPEM),
		Tags: nostr.Tags{
			nostr.Tag{"p", e.nprofile},
		},
	}
	err = event.Sign(e.config.NostrPrivateKey)
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
	return &cert, nil
}
