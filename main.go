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
	"strconv"
	"sync"
)

var localPort int
var remote string

/*
GetFreePort asks the kernel for a free open port that is ready to use.

It works by resolving the TCP address for "localhost:0" and attempting to listen on it.
When port 0 is requested, the kernel automatically assigns a free ephemeral port.
The listener is then closed, and the assigned port number is returned.

Note: There is a theoretical race condition where the port could be claimed by another process
immediately after it is released by this function, but it is generally safe for short intervals.

Source: https://gist.github.com/sevkin/96bdae9274465b2d09191384f86ef39d
*/
func GetFreePort() (port int, err error) {
	var a *net.TCPAddr
	if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer l.Close()
			return l.Addr().(*net.TCPAddr).Port, nil
		}
	}
	return
}

func init() {
	flag.IntVar(&localPort, "l", 0, "Raw TCP port to listen")
	flag.StringVar(&remote, "t", "", "Which TCP socket, that can be a TLS socket, to proxy")
	flag.Parse()
	var err error
	if localPort == 0 {
		localPort, err = GetFreePort()
		if err != nil {
			log.Fatalf("failed to find free port: %s", err)
		}
	}
	if remote == "" {
		log.Fatalf("missing tcp socket to connect")
	}
}

/*
GetPortStr returns a string representation of the port being listened on.

If the application is running under systemd socket activation (localPort < 0),
it returns "systemd". Otherwise, it returns the numeric port string.
This is primarily used for logging purposes.
*/
func GetPortStr() string {
	if localPort < 0 {
		return "systemd"
	} else {
		return fmt.Sprintf("%d", localPort)
	}
}

/*
GetListener creates a net.Listener for the application to accept connections.

It detects if the application is being run with systemd socket activation by checking
the LISTEN_PID environment variable.
 1. Systemd Mode: If LISTEN_PID matches the current PID, it wraps the file descriptor 3
    (which systemd passes for the socket) using net.FileListener.
 2. Manual Mode: If not running under systemd, it creates a standard TCP listener
    on 127.0.0.1 using the configured localPort.
*/
func GetListener() (net.Listener, error) {
	if os.Getenv("LISTEN_PID") == strconv.Itoa(os.Getpid()) {
		// systemd run
		f := os.NewFile(3, "from systemd")
		localPort = -1
		return net.FileListener(f)
	} else {
		// manual run
		return net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	}
}

/*
main is the entry point of the application.

It initializes the listener (either systemd or manual), and enters an infinite loop
to accept incoming TCP connections. For each connection:
1. It accepts the connection from the listener.
2. It dials the remote TLS upstream specified by the `-t` flag.
3. It spawns a goroutine (handleConn) to proxy data between the local connection and the upstream.

The application handles graceful shutdown of connections via context (though currently simple).
*/
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ln, err := GetListener()
	if err != nil {
		log.Fatalf("failed to listen socket %s: %s", GetPortStr(), err)
	}
	defer ln.Close()
	log.Printf("info: listening on port %s", GetPortStr())

	for {
		select {
		case <-ctx.Done():
			break
		default:
			downstream, err := ln.Accept()
			if err != nil {
				log.Printf("error/accept: %s", err.Error())
				continue
			}
			log.Printf("conn: %s", downstream.RemoteAddr().String())
			upstream, err := tls.Dial("tcp", remote, &tls.Config{})
			if err != nil {
				log.Printf("conn/%s: %s", downstream.RemoteAddr(), err)
				return
			}
			go handleConn(downstream, upstream)
		}
	}
}

/*
bufferPool is a sync.Pool used to reuse byte buffers for I/O operations.

Allocating a new buffer for every connection copy operation would create significant
garbage collection pressure. Using a pool allows us to recycle these 32KB buffers (1<<15).
*/
var bufferPool = sync.Pool{
	New: func() interface{} {
		// TODO maybe different buffer size?
		// benchmark pls
		return make([]byte, 1<<15)
	},
}

/*
handleConn bridges the connection between the downstream client and the upstream TLS server.

It initiates a bidirectional copy of data:
1. Downstream -> Upstream (run in a new goroutine).
2. Upstream -> Downstream (run in the current goroutine).

It uses sync.Once to ensure that connection cleanup (closing both sockets) happens exactly once,
preventing double-close errors or resource leaks. When either direction finishes (EOF or error),
both connections are closed.
*/
func handleConn(downstream, upstream net.Conn) {
	var once sync.Once
	closeConnections := func() {
		downstream.Close()
		upstream.Close()
		log.Printf("conn/%s: disconnected %v", downstream.RemoteAddr(), upstream.RemoteAddr())
	}

	cp := func(dst net.Conn, src net.Conn) {
		buf := bufferPool.Get().([]byte)
		defer bufferPool.Put(buf)
		// TODO use splice on linux
		// TODO needs some timeout to prevent torshammer ddos
		_, err := io.CopyBuffer(dst, src, buf)
		once.Do(func() {
			if err != nil {
				log.Print(err)
			}
			closeConnections()
		})
	}
	go cp(downstream, upstream)
	cp(upstream, downstream)
}
