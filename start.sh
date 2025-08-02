#!/usr/bin/env bash
set -Eeuo pipefail

CURRENT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

BINARY="${CURRENT_DIR}/build/magonote-tmux"

get_opt_value() {
  tmux show -vg "@magonote-${1}" 2>/dev/null || true
}

get_opt_arg() {
  local opt="$1"
  local type="$2"
  local value

  value="$(get_opt_value "${opt}")"

  if [[ "${type}" == "string" ]]; then
    [[ -n "${value}" ]] && echo "--${opt} ${value}"
  elif [[ "${type}" == "boolean" ]]; then
    [[ "${value}" == "1" ]] && echo "--${opt}"
  else
    return 1
  fi
}

PARAMS=(--dir "${CURRENT_DIR}/build/")

add_param() {
  local opt="$1"
  local type="$2"
  local value

  value="$(get_opt_value "${opt}")"
  if [[ -n "${value}" ]]; then
    if [[ "${type}" == "string" ]]; then
      PARAMS+=("--${opt}" "${value}")
    elif [[ "${type}" == "boolean" && "${value}" == "1" ]]; then
      PARAMS+=("--${opt}")
    fi
  fi
}

add_param command        string
add_param upcase-command string
add_param multi-command  string
add_param osc52          boolean

"${BINARY}" "${PARAMS[@]}" || true
