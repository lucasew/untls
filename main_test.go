package main

import (
	"io"
	"net"
	"testing"
	"time"
)

func TestIdleTimeout(t *testing.T) {
	// Start a listener
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	// Accept connection in goroutine
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// Keep connection open but send nothing
		io.Copy(io.Discard, conn)
	}()

	// Dial
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close()

	// Wrap with short timeout
	timeout := 200 * time.Millisecond
	wrapped := &idleTimeoutConn{Conn: conn, timeout: timeout}

	// Read should block until timeout
	start := time.Now()
	buf := make([]byte, 1)
	_, err = wrapped.Read(buf)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Check if error is timeout
	if netErr, ok := err.(net.Error); !ok || !netErr.Timeout() {
		t.Fatalf("expected timeout error, got: %v", err)
	}

	elapsed := time.Since(start)
	// We allow some slack, but it should be at least timeout
	if elapsed < timeout {
		t.Errorf("timeout happened too early: %v < %v", elapsed, timeout)
	}
}
