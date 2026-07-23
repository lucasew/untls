package main

import (
	"context"
	"net"
	"testing"
	"time"
)

// TestAcceptLoop_StopsOnCancel: cancel ctx and close the listener; acceptLoop
// must return nil promptly so main can exit cleanly under SIGTERM/SIGINT.
func TestAcceptLoop_StopsOnCancel(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		// Upstream never used: we stop before accepting a client.
		done <- acceptLoop(ctx, ln, "127.0.0.1:1")
	}()

	// Let Accept block, then mirror the production shutdown sequence:
	// cancel first, then close the listener so Accept unblocks.
	time.Sleep(50 * time.Millisecond)
	cancel()
	_ = ln.Close()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("acceptLoop: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("acceptLoop did not stop after cancel + Close")
	}
}

// TestAcceptLoop_PermanentAcceptError: closing the listener without cancelling
// ctx must return an error promptly. Previously Accept errors were logged and
// retried forever, which spins the CPU if the socket is closed outside the
// normal SIGTERM path.
func TestAcceptLoop_PermanentAcceptError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	// Intentionally no defer Close: we close below as the failure trigger.
	// If acceptLoop returns first for some other reason, close to avoid leak.
	defer func() { _ = ln.Close() }()

	// Never cancel: models a listener closed without the shutdown sequence.
	ctx := context.Background()
	done := make(chan error, 1)
	go func() {
		done <- acceptLoop(ctx, ln, "127.0.0.1:1")
	}()

	time.Sleep(50 * time.Millisecond)
	_ = ln.Close()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected permanent Accept error when listener closed without cancel")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("acceptLoop did not return after permanent Accept error (busy-loop?)")
	}
}

// TestAcceptLoop_AcceptsThenStops: one client is accepted before shutdown.
func TestAcceptLoop_AcceptsThenStops(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	// Closed port so serveConn's dial fails quickly and closes the client.
	dead, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("dead listen: %v", err)
	}
	remote := dead.Addr().String()
	_ = dead.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- acceptLoop(ctx, ln, remote)
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial listener: %v", err)
	}
	// serveConn closes the client after dial failure; Read should EOF/err.
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1)
	_, _ = conn.Read(buf)
	_ = conn.Close()

	cancel()
	_ = ln.Close()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("acceptLoop: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("acceptLoop did not stop after cancel + Close")
	}
}

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

	_, err = connectUpstream(context.Background(), server, addr)
	if err == nil {
		t.Fatal("expected dial error for closed upstream port")
	}

	// server was closed inside connectUpstream; further use should fail.
	if _, werr := server.Write([]byte("x")); werr == nil {
		t.Fatal("expected write on closed downstream to fail")
	}
}

// TestConnectUpstream_DialTimeout: listen but never Accept so TCP completes
// (kernel backlog) while the TLS handshake never finishes. Without a deadline
// this hung forever; with dialTimeout the call must fail promptly and still
// close downstream.
func TestConnectUpstream_DialTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	old := dialTimeout
	dialTimeout = 200 * time.Millisecond
	defer func() { dialTimeout = old }()

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()

	start := time.Now()
	_, err = connectUpstream(context.Background(), server, ln.Addr().String())
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error for hung TLS handshake")
	}
	// Allow a little early return from scheduling jitter, but it must not be
	// an instant hard-fail (that would be connection refused, not a hang).
	if elapsed < dialTimeout/2 {
		t.Fatalf("returned too fast (not a hang): elapsed=%v timeout=%v err=%v", elapsed, dialTimeout, err)
	}
	if elapsed > dialTimeout+2*time.Second {
		t.Fatalf("took too long: elapsed=%v timeout=%v err=%v", elapsed, dialTimeout, err)
	}
	if _, werr := server.Write([]byte("x")); werr == nil {
		t.Fatal("expected write on closed downstream to fail")
	}
}

// TestConnectUpstream_ParentCancel: a cancelled parent context must abort
// the dial before dialTimeout (SIGTERM path for in-flight handshakes).
func TestConnectUpstream_ParentCancel(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	old := dialTimeout
	dialTimeout = 10 * time.Second
	defer func() { dialTimeout = old }()

	client, server := net.Pipe()
	defer func() { _ = client.Close() }()

	parent, cancel := context.WithCancel(context.Background())
	// Let the dial start against a peer that never completes TLS.
	time.AfterFunc(50*time.Millisecond, cancel)

	start := time.Now()
	_, err = connectUpstream(parent, server, ln.Addr().String())
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected error after parent cancel")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("parent cancel did not abort promptly: elapsed=%v err=%v", elapsed, err)
	}
	if elapsed >= dialTimeout/2 {
		t.Fatalf("looks like dialTimeout, not parent cancel: elapsed=%v timeout=%v", elapsed, dialTimeout)
	}
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
		serveConn(context.Background(), server, addr)
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
		{name: "non-numeric port", addr: "example.com:abc", wantErr: true},
		{name: "service name port", addr: "example.com:https", wantErr: true},
		{name: "port zero", addr: "example.com:0", wantErr: true},
		{name: "port negative", addr: "example.com:-1", wantErr: true},
		{name: "port too large", addr: "example.com:65536", wantErr: true},
		{name: "host port", addr: "example.com:443", wantErr: false},
		{name: "ipv4 port", addr: "127.0.0.1:8443", wantErr: false},
		{name: "ipv6 port", addr: "[::1]:443", wantErr: false},
		{name: "port max", addr: "example.com:65535", wantErr: false},
		{name: "port one", addr: "example.com:1", wantErr: false},
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

func TestListenLabel(t *testing.T) {
	if got := listenLabel(nil, "systemd"); got != "systemd" {
		t.Fatalf("systemd: got %q", got)
	}

	// Port 0 must surface the OS-assigned port, not the "0" source string.
	t.Setenv("LISTEN_PID", "")
	ln, source, err := CreateListener(0)
	if err != nil {
		t.Fatalf("CreateListener(0): %v", err)
	}
	defer func() { _ = ln.Close() }()

	if source != "0" {
		t.Fatalf("source = %q, want \"0\"", source)
	}
	tcp, ok := ln.Addr().(*net.TCPAddr)
	if !ok || tcp.Port == 0 {
		t.Fatalf("expected non-zero bound port, addr=%v", ln.Addr())
	}
	got := listenLabel(ln, source)
	if got != ln.Addr().String() {
		t.Fatalf("listenLabel = %q, want %q", got, ln.Addr().String())
	}
	if got == "0" || got == source {
		t.Fatalf("listenLabel must not report unbound source %q", got)
	}
}
