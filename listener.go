package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
)

/**
 * GetFreePort asks the kernel for a free open port that is ready to use.
 *
 * Flow:
 * 1. Resolves a TCP address on localhost with port 0 (random).
 * 2. Listens on that address to let the kernel assign a port.
 * 3. Retrieves the assigned port.
 * 4. Closes the listener immediately.
 *
 * Edge Case:
 * There is a race condition where another process might bind to the returned port
 * between the time it is released here and when the caller tries to use it.
 *
 * Credit: https://gist.github.com/sevkin/96bdae9274465b2d09191384f86ef39d
 *
 * @returns {int} port - The free port number.
 * @returns {error} err - Error if unable to resolve or listen.
 */
func GetFreePort() (port int, err error) {
	var a *net.TCPAddr
	if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer func() { _ = l.Close() }()
			return l.Addr().(*net.TCPAddr).Port, nil
		}
	}
	return
}

/**
 * CreateListener initializes a TCP listener, prioritizing systemd socket activation.
 *
 * Logic:
 * - Checks if `LISTEN_PID` matches the current PID. If so, it assumes systemd passed
 *   the socket via file descriptor 3 (SD_LISTEN_FDS_START).
 * - If not running under systemd, it falls back to listening on `127.0.0.1:<port>`.
 *
 * @param {int} port - The fallback port to listen on if systemd is not detected.
 * @returns {net.Listener} - The initialized listener.
 * @returns {string} - A description of the listener source ("systemd" or "127.0.0.1:<port>").
 * @returns {error} - Error if listener creation fails.
 */
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
