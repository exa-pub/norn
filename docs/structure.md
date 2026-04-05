# Project Structure

## Principles

### Each package owns its types and errors

A service package defines the types it returns and the errors it produces. There is no shared `entity/` or `errors/` package — domain types live next to the logic that operates on them. This keeps abstractions from leaking across package boundaries: changing how `agent` represents a session doesn't touch `terminal` or `instance`.

A handler imports the service it calls, receives the types that service defines, and maps them to proto. No intermediary layer required.

### Separation by deployment boundary

Each runnable component (`server`, `daemon`) is isolated under its own top-level directory in `internal/`. They have independent dependencies, independent lifecycles, and communicate only through defined APIs. Code that belongs to one component never imports from another.

### Transport and logic are separate layers

Transport (`api/`) maps wire formats to service calls. Business logic (`service/`) knows nothing about HTTP, gRPC, or protobuf. A handler converts proto → domain, calls a service, converts domain → proto. That's it.

### A service package owns one domain concept end-to-end

Each `service/` subdirectory owns its state, its types, its errors, its invariants, and its persistence. Services depend on each other through interfaces, not concrete types. If two services need the same error sentinel — they each define their own. Identical code in two packages is not a problem; an accidental dependency between them is.

### Infrastructure has no domain knowledge

Packages in `internal/pkg/` provide building blocks: HTTP server helpers, Docker SDK wrappers, utilities. They know nothing about agents, terminals, or containers. If a package needs to know what an "agent" is to function — it's not infrastructure, it's a service.

### No premature sharing

Business logic lives in the component that uses it. Duplication across `server/` and `daemon/` is acceptable and preferred over coupling. When two packages share identical code, the question is not "how do I extract it?" but "is this really the same concept, or just two things that happen to look alike today?"

### Protocol kind determines `api/` subdivision

`api/` groups handlers by protocol: `api/connect/` for ConnectRPC, `api/ws/` for WebSocket, `api/middleware/` for HTTP middleware. The subdirectory name tells you the wire protocol without reading the code.

### Naming is uniform

RPC methods use the same verb set everywhere: `Create`, `Start`, `Stop`, `Delete`, `Rename`, `Get`, `List`. Messages follow: `CreateRequest` / `CreateResponse`. No entity-prefixed forms (`CreateAgent`, `CloseTerminal`).

### Generated code is a build artifact

`internal/gen/` is produced by `buf generate` from `proto/` and is never edited. Proto directory structure mirrors app structure: `proto/norn/server/...`, `proto/norn/daemon/...`.

## Layout

```
cmd/norn/main.go          # Entrypoint: cobra root, wires subcommands
internal/
├── gen/                   # Generated protobuf code (read-only)
├── pkg/                   # Infrastructure (no domain knowledge)
│   ├── httpsrv/           # HTTP/h2c server lifecycle
│   ├── dockerutils/       # Docker SDK wrapper
│   └── envflag/           # Cobra flag/env binding
├── server/                # norn server — runs on host
│   ├── cmd/               # Command() + bootstrap
│   ├── api/
│   │   ├── connect/       # ConnectRPC handlers
│   │   ├── ws/            # WebSocket handlers
│   │   └── middleware/    # Auth
│   └── service/           # Each package owns its types + errors
│       ├── instance/
│       ├── agent/
│       ├── terminal/
│       ├── daemonconn/    # gRPC connection pool to daemons
│       ├── storage/
│       └── devcontainer/  # Devcontainer CLI wrapper
└── daemon/                # norn run daemon — runs inside container
    ├── cmd/               # Command() + bootstrap
    ├── api/
    │   └── connect/       # ConnectRPC handlers (gRPC transport)
    └── service/           # Each package owns its types + errors
        ├── agent/
        ├── terminal/
        ├── storage/
        └── tty/
proto/norn/
├── server/                # Browser ↔ server API
└── daemon/                # Server ↔ daemon API
```

Each `cmd/` package exports a single `Command() *cobra.Command`. The top-level `cmd/norn/main.go` only wires them:

```go
root.AddCommand(servercmd.Command())
runCmd.AddCommand(daemoncmd.Command())
```
