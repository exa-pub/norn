#!/bin/sh
set -e

# Install Claude Code CLI
npm install -g @anthropic-ai/claude-code

# Apply dotfiles from shared volume (if any)
DOTFILES_DIR="$HOME/dotfiles"
if [ -d "$DOTFILES_DIR" ] && [ "$(ls -A "$DOTFILES_DIR" 2>/dev/null)" ]; then
    echo "Applying dotfiles from $DOTFILES_DIR..."
    for f in "$DOTFILES_DIR"/.*; do
        name=$(basename "$f")
        [ "$name" = "." ] || [ "$name" = ".." ] && continue
        ln -sf "$f" "$HOME/$name"
    done
fi

echo "Ready."
