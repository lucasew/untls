package main

import (
	"os"
	"strconv"
	"testing"
)

func TestCreateListener_Manual(t *testing.T) {
	// Ensure LISTEN_PID is not matching
	t.Setenv("LISTEN_PID", "")

	// Get a free port first to test with specific port, or just use 0
	// Using 0 means net.Listen will pick one, but CreateListener returns the input string.
	// Since the app ensures localPort != 0 before calling this, we can test with a non-zero port
	// provided by GetFreePort, or just assume 0 behavior is consistent with code.

	// Let's use GetFreePort to simulate app behavior
	port, err := GetFreePort()
	if err != nil {
		t.Fatalf("failed to get free port: %v", err)
	}

	ln, source, err := CreateListener(port)
	if err != nil {
		t.Fatalf("CreateListener failed: %v", err)
	}
	defer ln.Close()

	if source != strconv.Itoa(port) {
		t.Errorf("expected source %s, got %s", strconv.Itoa(port), source)
	}
}

func TestCreateListener_Systemd(t *testing.T) {
	// Simulate systemd environment
	pid := os.Getpid()
	t.Setenv("LISTEN_PID", strconv.Itoa(pid))

	// Attempt to create listener
	// This should try to use file descriptor 3.
	// In this test environment, FD 3 might be anything or nothing.
	// Likely it will fail to create a listener from it unless we set it up.
	// However, we want to verify it *tried* the systemd path.
	// The return string should be "systemd".

	ln, source, err := CreateListener(12345)

	// If it succeeds (unlikely but possible if FD 3 is valid), close it.
	if err == nil {
		ln.Close()
	}

	// We expect source to be "systemd" regardless of error, because that's how we coded it.
	if source != "systemd" {
		t.Errorf("expected source 'systemd', got '%s' (err: %v)", source, err)
	}

	// We expect err to be non-nil usually, OR nil if FD 3 happened to be a usable socket.
	// But ensuring it takes the path is what matters.
	// The main indicator is the returned source string.
}

func TestGetFreePort(t *testing.T) {
	port, err := GetFreePort()
	if err != nil {
		t.Fatalf("GetFreePort failed: %v", err)
	}
	if port <= 0 {
		t.Errorf("expected positive port, got %d", port)
	}
}
