# Sentinel Journal

## 2026-01-25 - Implement Idle Timeout to Prevent DoS
**Vulnerability:** The application lacks an idle timeout for TCP connections. This makes it vulnerable to resource exhaustion attacks like Slowloris or Torshammer, where an attacker keeps a connection open indefinitely by sending data very slowly or not at all, consuming file descriptors and goroutines.
**Learning:** The code explicitly contained a TODO comment `// TODO needs some timeout to prevent torshammer ddos`, highlighting that known security debt should be addressed promptly.
**Prevention:** Implement a wrapper around `net.Conn` that updates the read/write deadline on every activity. Enforce a reasonable idle timeout (e.g., 1 minute) to close stale connections.
