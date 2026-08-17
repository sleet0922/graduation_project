# Graduation project backend

This repository is the complete Incus build context for the Go backend,
PostgreSQL 18, Redis, LiveKit 1.13.5 and Caddy. It uses Debian 13 in a
privileged container with NAT networking.

LiveKit is not stored in this repository. During the temporary build stage,
the Incusfile downloads the v1.13.5 source archive from:

```text
https://github.com/livekit/livekit/archive/refs/tags/v1.13.5.tar.gz
```

If GitHub is unavailable, the same tagged source is downloaded from a domestic
Go module proxy. Both archive formats have pinned SHA-256 values, and the source
version is verified before compilation.

The API and LiveKit share one temporary builder. Its `ASDF go ${GO_VERSION}`
directive asks Bocker to install the exact Go version through embedded asdf,
then discard asdf, the Go toolchain and all build dependencies. The Incusfile
performs all package installation, compilation and database initialization
directly. Static Caddy, LiveKit and systemd configuration lives under `configs/`.

Requirements:

- Bocker 3.1.8 or newer
- `ssl/1.pem` and `ssl/1.key`
- free host ports `81/tcp`, `444/tcp`, `444/udp`, `7881/tcp` and `7882/udp`

Build the complete backend directly from the Incusfile, then start it:

```bash
bocker image build --permission super --network nat --name graduation-project ./Incusfile
bocker image run graduation-project --name graduation-project --permission super --network nat
```

Once started, verify it with:

```bash
bocker container exec graduation-project \
  systemctl is-active postgresql@18-main redis-server graduation-project livekit caddy
curl -k https://127.0.0.1:444/health
```

HTTP on port 81 redirects to HTTPS on port 444. PostgreSQL and Redis remain
container-internal. LiveKit signaling is proxied by Caddy, `444/udp` carries
HTTP/3, and `7881/tcp` plus `7882/udp` carry WebRTC media through NAT.

## Git workflow

Configure the GitHub remote:

```bash
git remote add origin https://github.com/sleet0922/graduation_project.git
git remote set-url origin git@github.com:sleet0922/graduation_project.git
```

Pull and inspect the current branch:

```bash
git pull origin main
git status
```

Clean unreachable Git objects and inspect the repository size:

```bash
git gc --prune=now
du -sh ./
```

Commit and push changes:

```bash
git add .
git commit -m "$(date +%Y-%m-%d)"
git push origin main
```

Build the API directly:

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -extldflags=-static" -gcflags="all=-l=4 -B -C" -o ./main ./cmd/api/main.go
```
