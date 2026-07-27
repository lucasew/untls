package main

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"
)

// TestAcceptLoop_ConcurrentClients checks that a hung upstream dial for one
// client does not stall Accept for others. serveConn runs off the accept
// loop; a regression that dials inline would serialize clients behind
// dialTimeout.
func TestAcceptLoop_ConcurrentClients(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	// Accept TCP so dials hang in the TLS handshake (not connection refused).
	hang, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("hang listen: %v", err)
	}
	defer func() { _ = hang.Close() }()
	remote := hang.Addr().String()

	old := dialTimeout
	dialTimeout = 400 * time.Millisecond
	defer func() { dialTimeout = old }()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- acceptLoop(ctx, ln, remote)
	}()

	const nClients = 3
	var wg sync.WaitGroup
	start := time.Now()
	errs := make(chan error, nClients)
	for i := 0; i < nClients; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, derr := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
			if derr != nil {
				errs <- derr
				return
			}
			// serveConn closes the client after dialTimeout TLS hang.
			_ = c.SetDeadline(time.Now().Add(dialTimeout + 2*time.Second))
			buf := make([]byte, 1)
			_, _ = c.Read(buf)
			_ = c.Close()
			errs <- nil
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("client: %v", err)
		}
	}
	elapsed := time.Since(start)

	// If dials ran serially on the accept path, wall time would be ~n*dialTimeout.
	// With concurrent serveConn, all three should finish near one dialTimeout.
	if elapsed > dialTimeout+2*time.Second {
		t.Fatalf("clients took too long (accept may be serializing dials): elapsed=%v dialTimeout=%v", elapsed, dialTimeout)
	}

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
