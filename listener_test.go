package main

import (
	"io"
	"net"
	"os"
	"runtime"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestCreateListener_Manual(t *testing.T) {
	// Ensure LISTEN_PID does not select the systemd path.
	t.Setenv("LISTEN_PID", "")

	// Port 0: kernel assigns an ephemeral port on 127.0.0.1. Matches production
	// after the GetFreePort()+rebind race was removed from main. Avoid probing a
	// free port then rebinding — that TOCTOU and can disagree on address family
	// (GetFreePort uses localhost; CreateListener binds 127.0.0.1).
	ln, source, err := CreateListener(0)
	if err != nil {
		t.Fatalf("CreateListener(0): %v", err)
	}
	defer func() { _ = ln.Close() }()

	if source != "0" {
		t.Errorf("expected source %q, got %q", "0", source)
	}
	tcp, ok := ln.Addr().(*net.TCPAddr)
	if !ok || tcp.Port <= 0 {
		t.Fatalf("expected assigned TCP port, addr=%v", ln.Addr())
	}
}

func TestCreateListener_ManualFixedPort(t *testing.T) {
	t.Setenv("LISTEN_PID", "")

	// Hold-then-release a port and rebind via CreateListener. Retry if another
	// process claims it between Close and Listen (same TOCTOU class as GetFreePort).
	var (
		ln     net.Listener
		source string
		port   int
		err    error
	)
	for attempt := 0; attempt < 8; attempt++ {
		seed, serr := net.Listen("tcp", "127.0.0.1:0")
		if serr != nil {
			t.Fatalf("seed listen: %v", serr)
		}
		port = seed.Addr().(*net.TCPAddr).Port
		_ = seed.Close()

		ln, source, err = CreateListener(port)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("CreateListener(fixed port) after retries: %v", err)
	}
	defer func() { _ = ln.Close() }()

	if source != strconv.Itoa(port) {
		t.Errorf("expected source %s, got %s", strconv.Itoa(port), source)
	}
	if tcp, ok := ln.Addr().(*net.TCPAddr); !ok || tcp.Port != port {
		t.Fatalf("listener addr=%v, want port %d", ln.Addr(), port)
	}
}

func TestCreateListener_Systemd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("systemd listen-FD activation is not used on windows")
	}

	// Install a real listening socket on FD 3 and set LISTEN_PID so CreateListener
	// takes the socket-activation path. The previous test only checked the source
	// string while wrapping whatever happened to be on FD 3 (often a non-socket),
	// which never proved Accept works and risked NewFile taking ownership of an
	// unrelated descriptor until GC.
	seed, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("seed listen: %v", err)
	}
	wantAddr := seed.Addr().String()

	seedFile, err := seed.(*net.TCPListener).File()
	if err != nil {
		_ = seed.Close()
		t.Fatalf("TCPListener.File: %v", err)
	}
	// File() duplicates the fd; close the Go listener so only seedFile holds the socket.
	_ = seed.Close()

	// Preserve the caller's FD 3 when present so we can restore after the test.
	origFD, origErr := syscall.Dup(sdListenFdsStart)

	if err := syscall.Dup2(int(seedFile.Fd()), sdListenFdsStart); err != nil {
		_ = seedFile.Close()
		if origErr == nil {
			_ = syscall.Close(origFD)
		}
		t.Fatalf("Dup2 onto FD %d: %v", sdListenFdsStart, err)
	}
	// seedFile's fd is distinct from FD 3 after Dup2; drop the extra ref.
	_ = seedFile.Close()

	t.Cleanup(func() {
		// Best-effort: close whatever CreateListener / this test left on FD 3,
		// then restore the original descriptor if we saved one.
		_ = syscall.Close(sdListenFdsStart)
		if origErr == nil {
			_ = syscall.Dup2(origFD, sdListenFdsStart)
			_ = syscall.Close(origFD)
		}
	})

	t.Setenv("LISTEN_PID", strconv.Itoa(os.Getpid()))

	ln, source, err := CreateListener(12345)
	if err != nil {
		t.Fatalf("CreateListener (systemd path): %v", err)
	}
	defer func() { _ = ln.Close() }()

	if source != "systemd" {
		t.Fatalf("expected source %q, got %q", "systemd", source)
	}
	if got := ln.Addr().String(); got != wantAddr {
		t.Fatalf("listener addr=%q, want seed addr %q", got, wantAddr)
	}

	// Accept a real client through the activated FD to prove the listener is live.
	errc := make(chan error, 1)
	go func() {
		c, derr := net.DialTimeout("tcp", wantAddr, 2*time.Second)
		if derr != nil {
			errc <- derr
			return
		}
		defer func() { _ = c.Close() }()
		_, werr := c.Write([]byte("ping"))
		errc <- werr
	}()

	if tl, ok := ln.(*net.TCPListener); ok {
		_ = tl.SetDeadline(time.Now().Add(2 * time.Second))
	}
	client, err := ln.Accept()
	if err != nil {
		t.Fatalf("Accept on systemd listener: %v", err)
	}
	defer func() { _ = client.Close() }()

	buf := make([]byte, 4)
	_ = client.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.ReadFull(client, buf); err != nil {
		t.Fatalf("Read client payload: %v", err)
	}
	if string(buf) != "ping" {
		t.Fatalf("payload=%q, want %q", buf, "ping")
	}
	if err := <-errc; err != nil {
		t.Fatalf("dial/write side: %v", err)
	}
}

func TestGetFreePort(t *testing.T) {
	port, err := GetFreePort()
	if err != nil {
		t.Fatalf("GetFreePort failed: %v", err)
	}
	if port <= 0 || port > 65535 {
		t.Errorf("expected TCP port in 1-65535, got %d", port)
	}
}
