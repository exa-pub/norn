
<h1>
    <img 
        src="web/public/favicon.svg" 
        alt="Norn" 
        width="32" 
        height="32" 
        align="absmiddle" 
    />
    Norn
</h1>

[![Release](https://img.shields.io/github/v/release/exa-pub/norn)](https://github.com/exa-pub/norn/releases)
[![CI](https://github.com/exa-pub/norn/actions/workflows/release.yml/badge.svg)](https://github.com/exa-pub/norn/actions/workflows/release.yml)
[![Go](https://img.shields.io/github/go-mod/go-version/exa-pub/norn)](go.mod)
[![License](https://img.shields.io/github/license/exa-pub/norn)](LICENSE)

Norn spins up isolated [devcontainers](https://containers.dev) and lets you run multiple [Claude Code](https://docs.anthropic.com/en/docs/claude-code) sessions inside them through a web UI. Each session gets its own terminal, persistent storage, and automatic resume on restart.

![Screenshot](docs/screenshot.png)

## Features

- **Instance management** — create, start, stop and delete devcontainers on demand
- **Agent sessions** — launch and manage multiple Claude Code conversations per instance, with automatic session resume
- **Terminals** — interactive shell access to running containers via the browser
- **Persistent storage** — instance state, agent sessions and logs survive restarts
- **Web UI** — sidebar with instances and agents, tabbed agent view, bottom panel with terminals and logs

## Quick start

### Docker

```bash
docker run --privileged -p 8080:8080 \
  -e EXEC_UID=$(id -u) -e EXEC_GID=$(id -g) \
  -v $(pwd):/workspace \
  -v norn-data:/data/.norn \
  -e NORN_WORKSPACE_FOLDER=/workspace \
  -e NORN_STORAGE_DIR=/data/.norn \
  --rm -i \
  ghcr.io/exa-pub/norn:latest
```

Open the URL printed in the logs. The image includes Docker-in-Docker and devcontainer CLI.

### Binary

Requires Docker and [devcontainer CLI](https://github.com/devcontainers/cli) on the host.

```bash
curl -fsSL https://raw.githubusercontent.com/exa-pub/norn/main/install.sh | sh
norn server
```

Or with an explicit workspace:

```bash
norn server --workspace-folder examples/simple
```

## Configuration

All settings can be passed as environment variables or CLI flags. Env vars take precedence over defaults but are overridden by explicit flags.

| Env Variable | Flag | Default | Description |
|-------------|------|---------|-------------|
| `NORN_ADDR` | `--addr` | `:8080` | Listen address |
| `NORN_STORAGE_DIR` | `--storage-dir` | `.norn` | Data directory |
| `NORN_SECRET` | `--auth-secret` | auto-generated | Auth secret |
| `NORN_WORKSPACE_FOLDER` | `--workspace-folder` | `.` | Path to folder with `.devcontainer/` |
| `NORN_DC_MOUNT_PATH` | `--norn-dc-mount-path` | `/mnt/norn/` | Mount path inside devcontainers |
| `NORN_DOTFILES_REPO` | `--dotfiles-repository` | | Dotfiles Git repo URL |
| `EXEC_UID` | | `1000` | UID to run norn process as (Docker only) |
| `EXEC_GID` | | `1000` | GID to run norn process as (Docker only) |

<details>
<summary>All options</summary>

| Env Variable | Flag | Default | Description |
|-------------|------|---------|-------------|
| `NORN_CONFIG` | `--config` | | Path to `devcontainer.json` |
| `NORN_OVERRIDE_CONFIG` | `--override-config` | | Override `devcontainer.json` |
| `NORN_DOCKER_PATH` | `--docker-path` | | Docker CLI path |
| `NORN_DOTFILES_COMMAND` | `--dotfiles-install-command` | | Dotfiles install command |
| `NORN_DOTFILES_PATH` | `--dotfiles-target-path` | | Dotfiles target path |
| `NORN_SECRETS_FILE` | `--secrets-file` | | Path to secrets JSON file |

Daemon (inside devcontainers): see [docs/daemon-env.md](docs/daemon-env.md).

</details>

### Docker Compose examples

```bash
# Custom port
NORN_PORT=9090 docker compose up

# Your own project
docker compose run --rm \
  -v /path/to/project:/workspace \
  -e NORN_WORKSPACE_FOLDER=/workspace \
  norn

# With auth secret
NORN_SECRET=mysecret docker compose up
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

See [LICENSE](LICENSE).
