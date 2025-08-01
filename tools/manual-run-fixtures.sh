#!/usr/bin/env bash

set -euo pipefail

LINE_WIDTH=$(tput cols)
LINE_WIDTH=$((LINE_WIDTH > 80 ? 80 : LINE_WIDTH))

print_banner() {
  local label="$1"
  local total_len=${#label}
  local prefix_len=$(((LINE_WIDTH - total_len - 2) / 2))
  local suffix_len=$((LINE_WIDTH - total_len - 2 - prefix_len))
  printf "%0.s=" $(seq 1 $prefix_len)
  printf " %s " "$label"
  printf "%0.s=" $(seq 1 $suffix_len)
  echo
}

for file in test/e2e/fixtures/*.txt; do
  label="Running: $file"
  print_banner "$label"
  echo
  cat "$file" | ./build/magonote
  echo
  print_banner "End of $file"
  sleep 0.2
done
