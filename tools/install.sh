#!/usr/bin/env bash

set -euo pipefail

LOCAL_BIN="$HOME/.local/bin"
BUILD_DIR="$(pwd)/build"
TARGETS=("magonote" "magonote-tmux")

# Uninstall mode
if [[ "${1-}" == "--uninstall" ]]; then
  for bin in "${TARGETS[@]}"; do
    link="$LOCAL_BIN/$bin"
    if [[ -L "$link" ]]; then
      echo "Removing symlink: $link"
      rm "$link"
    fi
  done
  exit 0
fi

mkdir -p "$LOCAL_BIN"

for bin in "${TARGETS[@]}"; do
  src="$BUILD_DIR/$bin"
  dst="$LOCAL_BIN/$bin"
  if [[ -f "$src" ]]; then
    ln -sf "$src" "$dst"
    echo "Linked $dst -> $src"
  else
    echo "Warning: $src not found, skipped." >&2
  fi
done
