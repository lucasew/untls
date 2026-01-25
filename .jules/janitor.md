## 2026-01-25 - Prevent Server Exit on Upstream Failure

**Issue:** The server process terminates completely if the upstream TLS connection fails (e.g., connection refused, timeout).
**Root Cause:** Usage of `return` inside the `main` accept loop upon `tls.Dial` error.
**Solution:** Replace `return` with `downstream.Close()` and `continue` to ensure the server keeps running and cleans up the failed client connection.
**Pattern:** Server accept loops must never exit on per-connection errors; resources must be cleaned up before continuing.
