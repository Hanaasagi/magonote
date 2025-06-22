#!/usr/bin/env bash

CURRENT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

DEFAULT_MAGONOTE_KEY=space

MAGONOTE_KEY="$(tmux show-option -gqv @magonote-key)"
MAGONOTE_KEY=${MAGONOTE_KEY:-$DEFAULT_MAGONOTE_KEY}

CURRENT_DIR="/home/kumiko/.tmux/plugins/tmux-magonote"
tmux set-option -ag command-alias "magonote-pick=run-shell -b ${CURRENT_DIR}/start.sh"
tmux bind-key "${MAGONOTE_KEY}" magonote-pick
