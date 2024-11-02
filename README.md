# untls

Reexposes a TCP over TLS port as a local TCP port

Usecases:
- Minecraft server over Tailscale Funnel: Unfortunately Minecraft doesn't support TLS sockets and Tailscale Funnel require them as implementation detail for multiplexing.

