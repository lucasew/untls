package main

import (
	"context"
	"crypto/tls"
	"flag"
	"io"
	"log"
	"net"
	"sync"
)

var localPort int
var remote string

func init() {
	flag.IntVar(&localPort, "l", 0, "Raw TCP port to listen")
	flag.StringVar(&remote, "t", "", "Which TCP socket, that can be a TLS socket, to proxy")
}

func main() {
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ln, portStr, err := CreateListener(localPort)
	if err != nil {
		log.Fatalf("failed to listen socket %s: %s", portStr, err)
	}
	defer ln.Close()
	log.Printf("info: listening on port %s", portStr)

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
