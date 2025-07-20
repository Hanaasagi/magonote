#!/usr/bin/env bash

set -euo pipefail

for file in test/e2e/fixtures/*.txt; do
  echo "====================[ Running: $file ]===================="
  echo
  cat "$file" | ./build/magonote
  echo
  echo "====================[ End of $file ]======================"
done

