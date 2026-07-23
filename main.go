package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

var localPort int
var remote string

func init() {
	flag.IntVar(&localPort, "l", 0, "Raw TCP port to listen")
	flag.StringVar(&remote, "t", "", "Which TCP socket, that can be a TLS socket, to proxy")
}

func main() {
	flag.Parse()
	if err := validateRemote(remote); err != nil {
		log.Fatal(err)
	}

	// localPort 0 → bind 127.0.0.1:0 and let the kernel pick a free port.
	// Avoid GetFreePort()+rebind: that races and can also disagree on address
	// family (localhost vs 127.0.0.1).
	ln, source, err := CreateListener(localPort)
	if err != nil {
		log.Fatalf("failed to listen socket %s: %s", source, err)
	}
	defer func() { _ = ln.Close() }()
	log.Printf("info: listening on %s", listenLabel(ln, source))

	// systemd (and interactive Ctrl-C) send SIGTERM/SIGINT. Catch them so we
	// can close the listener, unblock Accept, and exit 0 instead of being
	// SIGKILL'd after TimeoutStopSec with Accept still hanging.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		log.Printf("info: shutting down")
		_ = ln.Close()
	}()

	if err := acceptLoop(ctx, ln, remote); err != nil {
		log.Fatalf("accept loop: %s", err)
	}
}

// acceptLoop accepts clients until the listener is closed (normally because
// ctx was cancelled and the shutdown goroutine closed ln). Temporary accept
// failures are logged, backed off, and retried (same idea as net/http.Server);
// permanent Accept errors return so main can exit instead of spinning the CPU.
func acceptLoop(ctx context.Context, ln net.Listener, remote string) error {
	var tempDelay time.Duration
	for {
		downstream, err := ln.Accept()
		if err != nil {
			// Shutdown path: ctx cancelled then ln closed → Accept errors.
			if ctx.Err() != nil {
				return nil
			}
			// Temporary: back off and retry (e.g. transient resource pressure).
			// Permanent (including closed listener without cancel): stop.
			// net.Error.Temporary is deprecated but is still what net/http uses
			// for Accept; there is no replacement in the standard library.
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := time.Second; tempDelay > max {
					tempDelay = max
				}
				log.Printf("error/accept: %s; retrying in %v", err, tempDelay)
				timer := time.NewTimer(tempDelay)
				select {
				case <-ctx.Done():
					timer.Stop()
					return nil
				case <-timer.C:
				}
				continue
			}
			return fmt.Errorf("accept: %w", err)
		}
		tempDelay = 0
		log.Printf("conn/%s: accepted", downstream.RemoteAddr())
		// Dial and proxy off the accept loop so a slow or hung upstream
		// cannot stall Accept for other clients. Pass ctx so SIGTERM
		// aborts in-flight dials instead of waiting out dialTimeout.
		go serveConn(ctx, downstream, remote)
	}
}

// listenLabel is the human-readable bind description for startup logs.
// Under systemd socket activation the source is "systemd"; otherwise use the
// listener's actual local address so port 0 shows the OS-assigned port.
func listenLabel(ln net.Listener, source string) string {
	if source == "systemd" {
		return source
	}
	if ln != nil {
		if a := ln.Addr(); a != nil {
			return a.String()
		}
	}
	return source
}

// serveConn dials the upstream TLS endpoint and bridges the client.
// Safe to call from a goroutine per accepted connection. parentCtx is
// typically the process shutdown context so dials abort on SIGTERM.
func serveConn(parentCtx context.Context, downstream net.Conn, remote string) {
	addr := downstream.RemoteAddr()
	upstream, err := connectUpstream(parentCtx, downstream, remote)
	if err != nil {
		// connectUpstream already closed downstream.
		log.Printf("conn/%s: %s", addr, err)
		return
	}
	handleConn(downstream, upstream)
}

// dialTimeout bounds the whole upstream TCP+TLS handshake. Without a
// deadline, a blackholed or stuck peer leaves a goroutine and the client
// half-open forever (the accept loop is already off the hot path).
// Overridable in tests. Also cancelled early if parentCtx is done
// (process shutdown).
var dialTimeout = 10 * time.Second

// connectUpstream dials remote over TLS for a newly accepted client.
// parentCtx is combined with dialTimeout so either the wall-clock
// timeout or process shutdown ends the dial. On dial failure it closes
// downstream so the accept loop can continue without leaking the client
// socket or exiting the process.
func connectUpstream(parentCtx context.Context, downstream net.Conn, remote string) (net.Conn, error) {
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, cancel := context.WithTimeout(parentCtx, dialTimeout)
	defer cancel()

	upstream, err := (&tls.Dialer{Config: &tls.Config{}}).DialContext(ctx, "tcp", remote)
	if err != nil {
		_ = downstream.Close()
		return nil, err
	}
	return upstream, nil
}

// validateRemote checks that -t is a non-empty host:port suitable for tls.Dial.
// SplitHostPort alone accepts any non-empty port string (e.g. "abc"); require a
// numeric TCP port in 1–65535 so startup fails before the first Accept.
func validateRemote(addr string) error {
	if addr == "" {
		return fmt.Errorf("missing tcp socket to connect (-t host:port)")
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid -t address %q: want host:port: %w", addr, err)
	}
	if host == "" || port == "" {
		return fmt.Errorf("invalid -t address %q: want host:port", addr)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("invalid -t address %q: port must be 1-65535", addr)
	}
	return nil
}

const bufferSize = 1 << 15

/**
 * bufferPool is a sync.Pool used to reuse byte buffers for I/O operations.
 *
 * Allocating a new buffer for every connection copy operation would create significant
 * garbage collection pressure. Using a pool allows us to recycle these 32KB buffers (1<<15).
 */
var bufferPool = sync.Pool{
	New: func() any {
		// Note: 32KB buffer reduces GC pressure by utilizing sync.Pool
		b := make([]byte, bufferSize)
		return &b
	},
}

/**
 * handleConn bridges the connection between the downstream client and the upstream TLS server.
 *
 * It initiates a bidirectional copy of data:
 * 1. Downstream -> Upstream (run in a new goroutine).
 * 2. Upstream -> Downstream (run in the current goroutine).
 *
 * It uses sync.Once to ensure that connection cleanup (closing both sockets) happens exactly once,
 * preventing double-close errors or resource leaks. When either direction finishes (EOF or error),
 * both connections are closed.
 */
func handleConn(downstream, upstream net.Conn) {
	var once sync.Once
	closeConnections := func() {
		_ = downstream.Close()
		_ = upstream.Close()
		log.Printf("conn/%s: disconnected %v", downstream.RemoteAddr(), upstream.RemoteAddr())
	}

	cp := func(dst net.Conn, src net.Conn) {
		bufPtr := bufferPool.Get().(*[]byte)
		buf := *bufPtr
		defer bufferPool.Put(bufPtr)
		// Note: Splice unsupported for user-space TLS crypto. Complex timeout wrappers omitted per simplicity constraints.
		_, err := io.CopyBuffer(dst, src, buf)
		once.Do(func() {
			if err != nil {
				log.Printf("conn/%s: %v", downstream.RemoteAddr(), err)
			}
			closeConnections()
		})
	}
	go cp(downstream, upstream)
	cp(upstream, downstream)
}
