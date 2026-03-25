#!/bin/sh
set -e

# Persist ~/.claude.json and ~/.claude/ between container rebuilds
# Data lives in /workspaces/.claude-persist/ which survives rebuilds
PERSIST_DIR="${WORKSPACE_FOLDER:?WORKSPACE_FOLDER is not set}/.devcontainer/.persist"
mkdir -p "$PERSIST_DIR/.claude"
ln -sf "$PERSIST_DIR/.claude.json" ~/.claude.json
ln -sf "$PERSIST_DIR/.claude" ~/.claude


# make
sudo apt-get update -y
sudo apt-get install -y make

# buf
ARCH=$(uname -m)
curl -sSL "https://github.com/bufbuild/buf/releases/latest/download/buf-Linux-${ARCH}" -o /tmp/buf
sudo install /tmp/buf /usr/local/bin/buf

# Go protobuf + connect plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest

# Node protobuf + connect plugins
npm install -g @bufbuild/protoc-gen-es @connectrpc/protoc-gen-connect-es

# Claude Code CLI
npm install -g @anthropic-ai/claude-code
