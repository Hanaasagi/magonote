#!/usr/bin/env bash

tmux list-windows -F "#{window_id}:#{pane_id}:#{window_name}" | \
    awk -F: '$2 != "%0" && $3 ~ /magonote/ { print $1 }' | \
    while IFS= read -r window_id; do
        if [[ -n "$window_id" ]]; then
            tmux kill-window -t "$window_id"
        fi
    done
