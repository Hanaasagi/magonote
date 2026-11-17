package action

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// Vars carries the concrete selection values bound to the command at execution
// time. They are passed to bash as separate positional arguments (see
// BindScript), never interpolated into the command string, so selected text
// cannot inject shell syntax.
type Vars struct {
	Text   string
	Kind   string
	Pane   string
	Path   string
	Line   int
	Column int
}

// BindScript binds the selection variables from positional arguments and evals
// the rendered command ($6). Referencing the values as variables inside the
// command (rather than splicing them in) is what keeps execution injection-safe.
const BindScript = `MAGONOTE_TEXT="$1"; MAGONOTE_KIND="$2"; MAGONOTE_PANE="$3"; MAGONOTE_LINE="$4"; MAGONOTE_COLUMN="$5"; MAGONOTE_PATH="$6"; MAGONOTE_TEXT_SH="$(printf '%q' "$MAGONOTE_TEXT")"; MAGONOTE_PATH_SH="$(printf '%q' "$MAGONOTE_PATH")"; MAGONOTE_LINE_ARG=""; if [ "$MAGONOTE_LINE" -gt 0 ] 2>/dev/null; then MAGONOTE_LINE_ARG="+$MAGONOTE_LINE "; fi; eval "$7"`

// varRefs are the bash variable references substituted for template fields.
var varRefs = struct {
	Text, TextSh, Kind, Pane, Path, PathSh, Line, Column, LineArg string
}{
	Text:    "${MAGONOTE_TEXT}",
	TextSh:  "${MAGONOTE_TEXT_SH}",
	Kind:    "${MAGONOTE_KIND}",
	Pane:    "${MAGONOTE_PANE}",
	Path:    "${MAGONOTE_PATH}",
	PathSh:  "${MAGONOTE_PATH_SH}",
	Line:    "${MAGONOTE_LINE}",
	Column:  "${MAGONOTE_COLUMN}",
	LineArg: "${MAGONOTE_LINE_ARG}",
}

// Render expands {{.Text}}/{{.TextSh}}/{{.Kind}}/{{.Pane}}/{{.Path}}/{{.PathSh}}/
// {{.Line}}/{{.Column}}/{{.LineArg}} (and the legacy {} alias for {{.Text}}) into
// bash variable references. The *Sh variants are printf %q-escaped for safe use
// in a destination shell (e.g. keystrokes executed by the pane). The result is a
// command string to be eval'd with BindScript, not a final literal command.
func Render(tmpl string) (string, error) {
	tmpl = strings.ReplaceAll(tmpl, "{}", "{{.Text}}")
	t, err := template.New("action").Option("missingkey=zero").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parse action template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, varRefs); err != nil {
		return "", fmt.Errorf("render action template: %w", err)
	}
	return buf.String(), nil
}

// keyNames maps key notation to tmux key names.
var keyNames = map[string]string{
	"CR": "Enter", "Enter": "Enter", "Return": "Enter",
	"Esc": "Escape", "Escape": "Escape",
	"Tab": "Tab", "Space": "Space", "BS": "BSpace", "Backspace": "BSpace",
	"Up": "Up", "Down": "Down", "Left": "Left", "Right": "Right",
}

// KeysCommand turns rendered keystrokes (literal text plus <CR>-style key
// notation) into a shell command that sends them to the active pane. Literal
// runs are sent with send-keys -l (so arbitrary text is never interpreted as key
// names); each key token is sent as a separate send-keys so it is looked up as a
// key. This avoids the trap where `send-keys -l "text" Enter` would type the
// literal word "Enter".
func KeysCommand(keys string) string {
	var parts []string
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			parts = append(parts, fmt.Sprintf(`tmux send-keys -t "${MAGONOTE_PANE}" -l %s`, doubleQuote(lit.String())))
			lit.Reset()
		}
	}
	for i := 0; i < len(keys); {
		if keys[i] == '<' {
			if end := strings.IndexByte(keys[i:], '>'); end > 0 {
				token := keys[i+1 : i+end]
				if name, ok := keyName(token); ok {
					flush()
					parts = append(parts, fmt.Sprintf(`tmux send-keys -t "${MAGONOTE_PANE}" %s`, name))
					i += end + 1
					continue
				}
			}
		}
		lit.WriteByte(keys[i])
		i++
	}
	flush()
	return strings.Join(parts, " ; ")
}

// keyName resolves a notation token (without angle brackets) to a tmux key name.
// Control chords like C-c pass through unchanged.
func keyName(token string) (string, bool) {
	if name, ok := keyNames[token]; ok {
		return name, true
	}
	if strings.HasPrefix(token, "C-") || strings.HasPrefix(token, "M-") {
		return token, true
	}
	return "", false
}

// doubleQuote wraps s in double quotes, escaping characters that would break the
// quoting. '$' is left intact so variable references and intended environment
// expansions still resolve; selected text arrives via a variable, not here.
func doubleQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "`", "\\`")
	return `"` + s + `"`
}
