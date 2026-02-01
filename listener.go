package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
)

// from: https://gist.github.com/sevkin/96bdae9274465b2d09191384f86ef39d
// GetFreePort asks the kernel for a free open port that is ready to use.
func GetFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer func() { _ = l.Close() }()

	return l.Addr().(*net.TCPAddr).Port, nil
}

// CreateListener creates a listener based on the environment or arguments.
// It checks for systemd activation first.
// Returns the listener, a description of the source (e.g., "systemd" or "127.0.0.1:port"), and error.
func CreateListener(port int) (net.Listener, string, error) {
	if os.Getenv("LISTEN_PID") == strconv.Itoa(os.Getpid()) {
		// systemd run
		f := os.NewFile(3, "from systemd")
		l, err := net.FileListener(f)
		if err != nil {
			return nil, "systemd", err
		}
		return l, "systemd", nil
	}
	// manual run
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, strconv.Itoa(port), err
	}
	return l, strconv.Itoa(port), nil
}
