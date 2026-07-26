# untls

Reexposes a TLS-wrapped TCP service as a plain local TCP port.

Typical use: a Minecraft server behind Tailscale Funnel (TLS required for
multiplexing) while the game client only speaks raw TCP.

## Usage

```text
untls -t <host:port> [-l <port>]
```

| Flag | Meaning |
|------|---------|
| `-t` | **Required.** Upstream address that speaks TLS, as `host:port` (port `1–65535`). |
| `-l` | Local plain-TCP listen port. Default `0`: kernel picks an ephemeral port. Always binds `127.0.0.1` only. |

Direction of traffic:

1. Local clients connect to `127.0.0.1:<port>` with **no** TLS.
2. `untls` dials `-t` with TLS and proxies bytes both ways.

On startup the process logs the real listen address (so with `-l 0` you see the
OS-assigned port, not `0`).

### Minecraft + Tailscale Funnel example

Funnel publishes something like `your-machine.tailnet.ts.net:443` (TLS).
The Minecraft process still listens on plain TCP (e.g. `25565`).

On the machine that should expose a local plain port:

```bash
untls -t your-machine.tailnet.ts.net:443 -l 25565
```

Then point the Minecraft client at `127.0.0.1:25565`.

**Common mistake:** swapping the flags. `-l` is only a local port number.
`-t` is the remote TLS endpoint, not the local game server.

## Runtime behavior

- **Upstream dial:** each accepted client gets its own TLS dial. A slow or hung
  peer is limited to a **10s** dial timeout; a failed dial closes that client
  and leaves the accept loop running for others.
- **Shutdown:** `SIGINT` / `SIGTERM` close the listener, unblock `Accept`, and
  exit `0` (so systemd `TimeoutStopSec` does not need to `SIGKILL` a stuck
  accept). In-flight dials are cancelled on the same signal.
- **TLS trust:** peer certificates use the system CA pool (same as a normal
  Go `tls.Dial`). Container images need CA certs installed to verify public CAs.

## Systemd socket activation

If `LISTEN_PID` matches this process, `untls` uses the socket passed on **FD 3**
(`SD_LISTEN_FDS_START`) instead of binding `-l` itself. Useful when you want
socket activation or a unit-managed listen address (and still get clean
shutdown via `SIGTERM`).

Minimal pair (local plain TCP on `127.0.0.1:25565`, proxy to a TLS upstream):

`untls.socket`:

```ini
[Unit]
Description=untls local plain-TCP socket

[Socket]
ListenStream=127.0.0.1:25565
Accept=no

[Install]
WantedBy=sockets.target
```

`untls.service`:

```ini
[Unit]
Description=untls TLS→TCP proxy
Requires=untls.socket
After=untls.socket

[Service]
# -l is ignored under socket activation; -t is still required.
ExecStart=/usr/local/bin/untls -t your-machine.tailnet.ts.net:443
# Optional hard stop bound if something hangs outside Accept.
TimeoutStopSec=15

[Install]
WantedBy=multi-user.target
```

Enable with `systemctl enable --now untls.socket` (the `.service` is started
when the socket gets traffic, or enable the service too if you prefer it always
up). Confirm the process log line `listening on systemd`.

## Build / test

```bash
mise install
mise run ci      # lint + test
mise run build   # linux + windows binaries under build/
```
