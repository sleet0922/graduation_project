# Distributed Bocker test

This directory contains three independent Incusfiles for the graduation
project:

```sh
bocker image build --name graduation-postgres Incusfile.postgres
bocker image build --name graduation-livekit Incusfile.livekit
bocker image build --name graduation-backend Incusfile.backend
bocker image run graduation-postgres --name postgres
bocker image run graduation-livekit --name livekit
bocker image run graduation-backend --name backend
```

All three use Bocker's managed NAT network. The intended service names are
`postgres.bocker`, `livekit.bocker`, and `backend.bocker`; IP addresses are not
embedded in the files. PostgreSQL listens on `5432`, LiveKit on `7880`/`7881`
and `7882/udp`, and the Go API on `8081`.

On Debian systemd images the files pin `/etc/resolv.conf` to the Incus DNS
gateway because `systemd-resolved` otherwise forwards the private zone to the
host resolver. A transparent DNS proxy on the host may still rewrite private
answers; in that case verify the service path with the current `bocker
container list --json` IPv4 values or disable interception for `*.bocker`.
