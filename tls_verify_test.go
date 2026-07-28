package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"
)

// TestConnectUpstream_UntrustedTLSClosesDownstream: production dials with the
// default tls.Config (system CA pool). A peer that completes TCP but presents
// an untrusted certificate must fail the handshake and still close the client
// socket — same ownership rule as dial refused / dial timeout. Existing tests
// only cover connection-refused and hung-handshake peers.
func TestConnectUpstream_UntrustedTLSClosesDownstream(t *testing.T) {
	ln := mustSelfSignedTLSListener(t)
	defer func() { _ = ln.Close() }()

	// Accept TLS clients so the handshake can run (and fail verify on our side).
	// Without a peer Accept, we would hang until dialTimeout instead of failing
	// on certificate verification.
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()

	start := time.Now()
	_, err := connectUpstream(t.Context(), server, ln.Addr().String())
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected TLS certificate verification error for self-signed peer")
	}
	// Must not wait out the full dialTimeout: verify fails during handshake.
	if elapsed > dialTimeout/2 {
		t.Fatalf("looks like dialTimeout, not verify failure: elapsed=%v timeout=%v err=%v", elapsed, dialTimeout, err)
	}
	if _, werr := server.Write([]byte("x")); werr == nil {
		t.Fatal("expected write on closed downstream to fail")
	}
}

func mustSelfSignedTLSListener(t *testing.T) net.Listener {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "untls-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	cert := tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  key,
	}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	})
	if err != nil {
		t.Fatalf("tls.Listen: %v", err)
	}
	return ln
}
