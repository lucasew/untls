package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"testing"
	"time"
)

// TestHandleConn_Bidirectional checks the core proxy: bytes flow both ways
// through handleConn and the bridge returns after a peer hangs up.
func TestHandleConn_Bidirectional(t *testing.T) {
	client, downstream := net.Pipe()
	upstream, server := net.Pipe()

	done := make(chan struct{})
	go func() {
		handleConn(downstream, upstream)
		close(done)
	}()

	// client -> server (downstream -> upstream)
	errc := make(chan error, 1)
	go func() {
		buf := make([]byte, 5)
		if _, err := io.ReadFull(server, buf); err != nil {
			errc <- err
			return
		}
		if !bytes.Equal(buf, []byte("hello")) {
			errc <- fmt.Errorf("upstream got %q want %q", buf, "hello")
			return
		}
		errc <- nil
	}()
	if _, err := client.Write([]byte("hello")); err != nil {
		t.Fatalf("client write: %v", err)
	}
	if err := <-errc; err != nil {
		t.Fatalf("client->server: %v", err)
	}

	// server -> client (upstream -> downstream)
	go func() {
		buf := make([]byte, 5)
		if _, err := io.ReadFull(client, buf); err != nil {
			errc <- err
			return
		}
		if !bytes.Equal(buf, []byte("world")) {
			errc <- fmt.Errorf("downstream got %q want %q", buf, "world")
			return
		}
		errc <- nil
	}()
	if _, err := server.Write([]byte("world")); err != nil {
		t.Fatalf("server write: %v", err)
	}
	if err := <-errc; err != nil {
		t.Fatalf("server->client: %v", err)
	}

	// Closing the client must tear down both pipes and return handleConn.
	if err := client.Close(); err != nil {
		t.Fatalf("client close: %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handleConn did not return after client closed")
	}

	// server end should be unusable after once-close of the bridge.
	if _, err := server.Write([]byte("x")); err == nil {
		t.Fatal("expected write on closed upstream peer to fail")
	}
}
