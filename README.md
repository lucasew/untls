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
| `-t` | **Required.** Upstream address that speaks TLS, as `host:port`. |
| `-l` | Local plain-TCP listen port. Default: pick a free port. Always binds `127.0.0.1`. |

Direction of traffic:

1. Local clients connect to `127.0.0.1:< -l >` with **no** TLS.
2. `untls` dials `-t` with TLS and proxies bytes both ways.

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

## Systemd socket activation

If `LISTEN_PID` matches this process, `untls` uses the socket passed on FD 3
instead of binding `-l` itself.

## Build / test

```bash
mise install
mise run ci      # lint + test
mise run build   # linux + windows binaries under build/
```
