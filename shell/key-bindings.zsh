# CTRL-G - Use magonote interactive selector and insert result into command line
__magonote_select() {
  setopt localoptions pipefail no_aliases 2> /dev/null
  local item
  local lines=$(($(tput lines) * 2))
  item=$(tmux capture-pane -e -p -S -$lines -E -1 | magonote  --list)
  [[ -n "$item" ]] && echo -n "${(q)item}"
}

magonote-widget() {
  LBUFFER="${LBUFFER}$(__magonote_select)"
  local ret=$?
  zle reset-prompt
  return $ret
}

zle     -N   magonote-widget
bindkey '^G' magonote-widget
