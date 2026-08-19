# Graduation project backend

This repository is the complete Bocker YAML build context for the Go backend,
Debian 13's PostgreSQL 17 and Redis packages, LiveKit 1.13.5 and Caddy. It uses Debian 13 in a
privileged container with NAT networking.

LiveKit is not stored in this repository. During the temporary YAML builder stage,
`Incusfile.yaml` downloads the v1.13.5 source archive from:

```text
https://github.com/livekit/livekit/archive/refs/tags/v1.13.5.tar.gz
```

If GitHub is unavailable, the same tagged source is downloaded from a domestic
Go module proxy. Both archive formats have pinned SHA-256 values before compilation.

The API and LiveKit share one temporary builder stage. Its YAML `mise` step
asks Bocker to install the exact Go version through embedded mise,
then discard mise, the Go toolchain and all build dependencies. `Incusfile.yaml`
performs all package installation, compilation and database initialization
directly. API and LiveKit services intentionally run as root in the target
container. Static Caddy, LiveKit and systemd configuration lives under `configs/`.

The build file is `Incusfile.yaml`. It uses strict YAML only; the legacy
line-oriented `Incusfile` syntax is not supported.
The YAML uses declarative `download`, `exec.capture`, `write` and `service`
steps for the parts that previously required large shell blocks. Commands keep
their argv boundaries, while generated values stay build-scoped.

Requirements:

- Bocker 3.2.1 or newer
- `ssl/1.pem` and `ssl/1.key`
- free host ports `81/tcp`, `444/tcp`, `444/udp`, `7881/tcp` and `7882/udp`

Build the complete backend directly from `Incusfile.yaml`, then start it:

```bash
bocker image build --permission super --network nat --name graduation-project ./Incusfile.yaml
bocker image run graduation-project --name graduation-project --permission super --network nat
```

Once started, verify it with:

```bash
bocker container exec graduation-project \
  systemctl is-active postgresql redis-server graduation-project livekit caddy
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
