#!/usr/bin/env bash

CURRENT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
START_SCRIPT="${CURRENT_DIR}/start.sh"
DEFAULT_MAGONOTE_KEY="space"

MAGONOTE_KEY="$(tmux show-option -gqv @magonote-key)"
MAGONOTE_KEY="${MAGONOTE_KEY:-$DEFAULT_MAGONOTE_KEY}"

if [[ ! -x "${START_SCRIPT}" ]]; then
  tmux display-message "magonote: start.sh not found or not executable at ${START_SCRIPT}"
else
  tmux set-option -ag command-alias "magonote-pick=run-shell -b ${START_SCRIPT}"
  tmux bind-key "${MAGONOTE_KEY}" magonote-pick
fi
