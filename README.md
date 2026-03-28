# Norn

AI agent devcontainer manager. Runs isolated development environments and provides a web UI for managing multiple Claude Code sessions inside them.

![Screenshot](docs/screenshot.png)

## Features

- **Instance management** — create, start, stop and delete devcontainers on demand
- **Agent sessions** — launch and manage multiple Claude Code conversations per instance, with automatic session resume
- **Terminals** — interactive shell access to running containers via the browser
- **Persistent storage** — instance state, agent sessions and logs survive restarts
- **Web UI** — sidebar with instances and agents, tabbed agent view, bottom panel with terminals and logs

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/exa-pub/norn/main/install.sh | sh
```

To install a specific version:

```bash
NORN_VERSION=v0.1.0 curl -fsSL https://raw.githubusercontent.com/exa-pub/norn/main/install.sh | sh
```

## Quick start

### Prerequisites

- Docker
- [devcontainer CLI](https://github.com/devcontainers/cli)

### Run

```bash
# Just run inside project with .devcontainer/
norn

# norn --workspace-folder examples/simple --storage-dir .norn
```

Open the URL printed in the terminal. The auth secret is passed via URL fragment on first launch.

### Build from source

Requires Go 1.26+ and Node.js (LTS).

```bash
make build
./bin/norn --workspace-folder examples/simple --storage-dir .norn
```

### Development

```bash
make dev-go    # run Go server
make dev-web   # run Vite dev server with hot reload
make test      # run tests
```

## CLI flags

```
--addr                     Listen address (default :8080)
--storage-dir              Data directory (default .norn)
--workspace-folder         Path to folder with .devcontainer/ (default .)
--auth-secret              Auth secret (default: auto-generated, saved to storage-dir)
--config                   Path to devcontainer.json
--override-config          Override devcontainer.json
--docker-path              Path to docker CLI
--remote-env KEY=VALUE     Environment variables for devcontainer exec
--mount                    Additional mount points
--dotfiles-repository      Git repo for dotfiles
--dotfiles-install-command Command to run after cloning dotfiles
--dotfiles-target-path     Dotfiles install path in container
--secrets-file             Path to JSON file with secrets
```

The `NORN_SECRET` environment variable can be used instead of `--auth-secret`.

## Environment variables for devcontainers

Norn automatically sets these environment variables when creating an instance. Use them in your `devcontainer.json` via `${localEnv:VAR}`:

| Variable | Description |
|----------|-------------|
| `INSTANCE_MNT_PATH` | Host path to persistent storage for this instance |
| `DOTS_PATH` | Host path to shared dotfiles directory (common across all instances) |

See [examples/simple](examples/simple) for a working `devcontainer.json` that uses these variables for mounts.

## Architecture

```
Norn Server
├── API (ConnectRPC + WebSocket)
│   ├── ContainerService   — instance lifecycle
│   ├── AgentService       — Claude Code sessions
│   └── TerminalService    — interactive shells
├── Services
│   ├── instance/   — devcontainer management via Docker
│   ├── agent/      — agent session state and PTY
│   ├── terminal/   — terminal session state and PTY
│   ├── tty/        — PTY multiplexing with ring buffer
│   └── storage/    — file-based persistence
└── Web UI (React + TypeScript + Vite)
```

## License

See [LICENSE](LICENSE).
