# Graduation project backend

This repository is the complete Incus build context for the Go backend,
PostgreSQL 18, Redis, LiveKit 1.13.5 and Caddy. It uses Debian 13 in a
privileged container with NAT networking.

LiveKit is not stored in this repository. During the temporary build stage,
the Incusfile downloads the v1.13.5 source archive from:

```text
https://github.com/livekit/livekit/archive/refs/tags/v1.13.5.tar.gz
```

If GitHub is unavailable, a domestic proxy is used as a fallback. The archive
SHA-256 and source version are verified before compilation.

Requirements:

- Bocker 3.1.6 or newer
- GNU Make
- `ssl/1.pem` and `ssl/1.key`
- free host ports `81/tcp`, `444/tcp`, `444/udp`, `7881/tcp` and `7882/udp`

Build and start the complete backend with one command:

```bash
make build-bocker
```

The target builds and starts the container with `--permission super` and NAT
networking. Once started, verify it with:

```bash
make bocker-status
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
