# examples/simple

Minimal devcontainer for use with Norn. Contains Claude Code CLI and mounts for Norn-managed volumes.

## Volumes

Norn passes paths via environment variables when running `devcontainer up`:

| Env var | Mount target | Purpose |
|---------|-------------|---------|
| `INSTANCE_MNT_PATH` | `/home/vscode/mnt` | Persistent storage for this instance |
| `DOTS_PATH` | `/home/vscode/dotfiles` | Shared dotfiles across all instances |

## Usage with Norn

```sh
norn --storage-dir .norn \
     --workspace-folder examples/simple \
     --devcontainer-config examples/simple/.devcontainer/devcontainer.json
```

Then via API:

```sh
# Create instance
curl -X POST localhost:8080/norn.containers.v1.ContainerService/CreateContainer \
  -H 'Content-Type: application/json' \
  -d '{"name": "dev"}'

# Start instance
curl -X POST localhost:8080/norn.containers.v1.ContainerService/StartContainer \
  -H 'Content-Type: application/json' \
  -d '{"name": "dev"}'
```
