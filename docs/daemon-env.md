# Daemon Environment Variables

The daemon (`norn run daemon`) runs inside a devcontainer and is configured exclusively through environment variables. It does not read CLI flags from the user — the server passes the necessary arguments when launching the daemon via `devcontainer exec`.

## Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `NORN_DAEMON_SOCKET` | `/mnt/norn/run/daemon.sock` | Path to the Unix socket the daemon listens on |
| `NORN_DAEMON_RUN_DIR` | `/mnt/norn/run` | Directory for daemon state: agent metadata (`agents/`), logs (`logs/`), PID file |

These paths are relative to the mount layout created by the server. The server mounts `.norn/instances/{name}/run/` into the container at the configured `--norn-dc-mount-path`/run/ (default `/mnt/norn/run/`).

## How the daemon is launched

The server starts the daemon automatically after `devcontainer up`:

```
devcontainer exec ... /mnt/norn/bin/norn run daemon
```

The daemon reads its configuration from the environment. The server sets these variables via `--remote-env` when creating the container.

## Filesystem layout inside the container

```
/mnt/norn/                  ← NORN_DC_MOUNT_PATH (server-side config)
├── bin/norn                ← norn binary (readonly mount)
├── dotfiles/               ← shared dotfiles
├── instance/               ← instance data (server-managed, read-only for daemon)
└── run/                    ← NORN_DAEMON_RUN_DIR
    ├── daemon.sock         ← NORN_DAEMON_SOCKET
    ├── daemon.pid
    ├── agents/
    │   └── {uuid}.json
    └── logs/
        └── YYYY-MM-DD/
            ├── HH-mm-ss.log
            └── HH-mm-ss.err
```
