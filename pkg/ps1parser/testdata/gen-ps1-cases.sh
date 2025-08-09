#!/usr/bin/env zsh

setopt EXTENDED_GLOB
theme_dir="${ZSH:-$HOME/.oh-my-zsh}/themes"
script_dir="${0:A:h}"
output_file="${script_dir}/testcases.json"

tmpfile=$(mktemp)

print "[" > "$tmpfile"

first=true

for theme_file in $theme_dir/*.zsh-theme; do
  theme_name=${theme_file:t:r}

  export ZSH_THEME="$theme_name"
  source "$ZSH/oh-my-zsh.sh" >/dev/null 2>&1

  ps1_val="$PS1"
  printed_ps1="$(print -P "$PS1")"

  if $first; then
    first=false
  else
    print "," >> "$tmpfile"
  fi

  jq -n \
    --arg theme "$theme_name" \
    --arg ps1 "$ps1_val" \
    --arg text "$printed_ps1" \
    '{theme: $theme, ps1: $ps1, text: $text}' >> "$tmpfile"
done

print "]" >> "$tmpfile"

mv "$tmpfile" "$output_file"

echo "Done. Output written to $output_file"
