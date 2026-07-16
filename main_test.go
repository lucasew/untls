package main

import (
	"net"
	"testing"
	"time"
)

func TestConnectUpstream_DialFailClosesDownstream(t *testing.T) {
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()

	// Bind then close so dial fails immediately with connection refused.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	_, err = connectUpstream(server, addr)
	if err == nil {
		t.Fatal("expected dial error for closed upstream port")
	}

	// server was closed inside connectUpstream; further use should fail.
	if _, werr := server.Write([]byte("x")); werr == nil {
		t.Fatal("expected write on closed downstream to fail")
	}
}

func TestServeConn_DialFailClosesDownstream(t *testing.T) {
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	// serveConn must return after dial failure without panicking and must
	// close the client side (same ownership rules as connectUpstream).
	done := make(chan struct{})
	go func() {
		serveConn(server, addr)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("serveConn did not return after dial failure")
	}

	if _, werr := server.Write([]byte("x")); werr == nil {
		t.Fatal("expected write on closed downstream to fail")
	}
}

func TestValidateRemote(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{name: "empty", addr: "", wantErr: true},
		{name: "host only", addr: "example.com", wantErr: true},
		{name: "port only", addr: ":443", wantErr: true},
		{name: "host empty port", addr: "example.com:", wantErr: true},
		{name: "host port", addr: "example.com:443", wantErr: false},
		{name: "ipv4 port", addr: "127.0.0.1:8443", wantErr: false},
		{name: "ipv6 port", addr: "[::1]:443", wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRemote(tt.addr)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateRemote(%q) err=%v wantErr=%v", tt.addr, err, tt.wantErr)
			}
		})
	}
}
