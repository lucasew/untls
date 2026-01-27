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

// from: https://gist.github.com/sevkin/96bdae9274465b2d09191384f86ef39d
// GetFreePort asks the kernel for a free open port that is ready to use.
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

func parseFlags() {
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

func GetPortStr() string {
	if localPort < 0 {
		return "systemd"
	} else {
		return fmt.Sprintf("%d", localPort)
	}
}

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

func main() {
	parseFlags()
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

var bufferPool = sync.Pool{
	New: func() interface{} {
		// TODO maybe different buffer size?
		// benchmark pls
		return make([]byte, 1<<15)
	},
}

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
