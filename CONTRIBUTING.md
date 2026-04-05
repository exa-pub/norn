# Contributing

## Prerequisites

- Go 1.26+
- Node.js (LTS)
- Docker
- [devcontainer CLI](https://github.com/devcontainers/cli)
- [buf](https://buf.build/) (for protobuf generation)

## Build

```bash
make build          # full build: proto → web → go binary
```

Individual steps:

```bash
make proto          # buf generate
make web            # npm install + build
make go             # go build (includes embedded frontend)
```

## Development

```bash
make dev-go         # run Go server (go run)
make dev-web        # run Vite dev server with hot reload
make test           # go test ./...
make test-integration  # integration tests (requires Docker)
```

## Run locally

```bash
make run-simple     # build + run with examples/simple workspace
```

Or with Docker Compose (builds from local source):

```bash
make build
docker compose up --build
```

## Project structure

See [docs/structure.md](docs/structure.md) for principles and layout.

```
cmd/norn/              # CLI entrypoint
internal/
├── gen/               # Generated protobuf (read-only)
├── pkg/               # Infrastructure (no domain knowledge)
├── server/            # norn server (host-side)
│   ├── cmd/           # Command + bootstrap
│   ├── api/           # ConnectRPC, WebSocket, middleware
│   └── service/       # Business logic
└── daemon/            # norn run daemon (inside devcontainers)
    ├── cmd/           # Command + bootstrap
    ├── api/           # ConnectRPC handlers
    └── service/       # Business logic
proto/norn/
├── server/            # Browser ↔ server API
└── daemon/            # Server ↔ daemon API
```

## Protobuf

Proto files are in `proto/`. Generated code goes to `internal/gen/`.

```bash
make proto          # or: buf generate
```

Naming convention: `Create`, `Start`, `Stop`, `Delete`, `Rename`, `Get`, `List`. No entity-prefixed forms.

## Architecture

Norn has two components in one binary:

- **`norn server`** runs on the host, manages containers, serves the web UI, proxies requests to daemons
- **`norn run daemon`** runs inside each devcontainer, owns all PTY sessions (agents + terminals)

They communicate via ConnectRPC over Unix sockets. See [ai.tmp/agent-plan.md](ai.tmp/agent-plan.md) for the full architecture plan.

### Key concepts

- **Instance** — a named devcontainer. Status is computed (not stored) from Docker API + startup state + error file
- **Agent Session** — a persistent Claude Code conversation. Metadata in `run/agents/`. Runtime state in daemon memory
- **Terminal** — an ephemeral shell. Fully in daemon memory, lost on restart
- **TTY Session** — underlying PTY with ring buffer (256KB) for replay on late-joining clients

### Data flow

```
Browser → WebSocket → Server → gRPC/Unix socket → Daemon → PTY (claude/bash)
```

### Storage layout

```
.norn/
├── shared/dotfiles/
└── instances/{name}/
    ├── identity.json
    ├── last_error
    ├── mnt/
    ├── logs/
    └── run/               # daemon workspace (mounted into container)
        ├── daemon.sock
        ├── agents/
        └── logs/
```

## Daemon environment

The daemon is configured via environment variables. See [docs/daemon-env.md](docs/daemon-env.md).
