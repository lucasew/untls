package main

import (
	"net"
	"os"
	"strconv"
)

// sdListenFdsStart is SD_LISTEN_FDS_START: the first FD systemd passes for
// socket activation (see sd_listen_fds(3)).
const sdListenFdsStart = 3

// CreateListener opens the plain-TCP accept socket.
//
// If LISTEN_PID matches this process, it uses the systemd-activated socket on
// FD 3. Otherwise it binds 127.0.0.1:port (port 0 = kernel-assigned ephemeral).
// The source string is "systemd" or the requested port decimal for logs.
func CreateListener(port int) (net.Listener, string, error) {
	if os.Getenv("LISTEN_PID") == strconv.Itoa(os.Getpid()) {
		// systemd run
		f := os.NewFile(sdListenFdsStart, "from systemd")
		l, err := net.FileListener(f)
		if err != nil {
			return nil, "systemd", err
		}
		return l, "systemd", nil
	}
	// manual run
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, strconv.Itoa(port), err
	}
	return l, strconv.Itoa(port), nil
}
