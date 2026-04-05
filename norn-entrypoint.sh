#!/bin/sh
set -e

EXEC_UID="${EXEC_UID:-1000}"
EXEC_GID="${EXEC_GID:-1000}"

# Required env vars
if [ -z "$NORN_STORAGE_DIR" ]; then
    echo "ERROR: NORN_STORAGE_DIR is required" >&2
    exit 1
fi
if [ -z "$NORN_WORKSPACE_FOLDER" ]; then
    echo "ERROR: NORN_WORKSPACE_FOLDER is required" >&2
    exit 1
fi

# Create group and user with the requested uid/gid
addgroup -g "$EXEC_GID" -S norn 2>/dev/null || true
adduser -u "$EXEC_UID" -G norn -S -D -h /home/norn norn 2>/dev/null || true

# Ensure home and storage dirs are writable
mkdir -p /home/norn
chown "$EXEC_UID:$EXEC_GID" /home/norn
mkdir -p "$NORN_STORAGE_DIR"
chown "$EXEC_UID:$EXEC_GID" "$NORN_STORAGE_DIR"

# Start Docker daemon in background (runs as root)
dockerd &

# Wait for Docker daemon to be ready
until docker info >/dev/null 2>&1; do
    sleep 0.5
done

# Make docker socket accessible to the exec user
chmod 666 /var/run/docker.sock

# Run the command as the specified user
exec su-exec "$EXEC_UID:$EXEC_GID" "$@"
