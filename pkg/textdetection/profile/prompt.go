package profile

import (
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Hanaasagi/magonote/pkg/textdetection/screen"
)

var promptPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^(?P<prefix>(?:\S+@\S+)?[^$%#>]{0,60}?[$%]\s)(?P<cmd>\S.*)$`),
	regexp.MustCompile(`^(?P<prefix>\S*@\S*[:\s][^#]{0,60}?#\s)(?P<cmd>\S.*)$`),
	regexp.MustCompile(`^(?P<prefix>.{0,60}?[❯➜»›λ]\s+)(?P<cmd>\S.*)$`),
	regexp.MustCompile(`^(?P<prefix>\[[^\]]{1,60}\][$#%]\s)(?P<cmd>\S.*)$`),
	regexp.MustCompile(`^(?P<prefix>[^>]{1,60}?>\s)(?P<cmd>\S.*)$`),
}

var commandNamePattern = regexp.MustCompile(`^[A-Za-z_][\w.+-]*$`)
var assignmentPattern = regexp.MustCompile(`^[A-Za-z_]\w*=.*$`)

func DetectPrompts(s *screen.Screen, customPatterns []string) []Prompt {
	patterns := append([]*regexp.Regexp{}, promptPatterns...)
	for _, pattern := range customPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil || re.SubexpIndex("cmd") < 0 {
			slog.Warn("invalid custom prompt pattern; skipping", "pattern", pattern, "error", err)
			continue
		}
		patterns = append(patterns, re)
	}

	var prompts []Prompt
	for line := range s.Lines {
		plain := s.Lines[line].Plain
		for _, re := range patterns {
			indices := re.FindStringSubmatchIndex(plain)
			if indices == nil {
				continue
			}
			cmdIndex := re.SubexpIndex("cmd")
			cmdStart, cmdEnd := indices[2*cmdIndex], indices[2*cmdIndex+1]
			if cmdStart < 0 {
				continue
			}
			prefix := plain[:cmdStart]
			if strings.Contains(prefix, "->") || strings.Contains(prefix, " >") {
				continue
			}
			command := ParseCommand(plain[cmdStart:cmdEnd])
			if len(command.Argv) == 0 || !reasonableCommand(command.Argv[0]) {
				continue
			}
			prompts = append(prompts, Prompt{Line: line, PrefixEnd: s.ColOf(line, cmdStart), Command: command})
			break
		}
	}
	return prompts
}

func reasonableCommand(name string) bool {
	if strings.HasPrefix(name, "./") || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "~/") {
		return true
	}
	return commandNamePattern.MatchString(name)
}

func ParseCommand(raw string) Command {
	fields, segment := shellFields(raw)
	for len(fields) > 0 {
		if assignmentPattern.MatchString(fields[0]) {
			fields = fields[1:]
			continue
		}
		base := filepath.Base(fields[0])
		switch base {
		case "sudo", "env", "time", "nohup", "exec", "command", "nice":
			fields = fields[1:]
			for len(fields) > 0 && strings.HasPrefix(fields[0], "-") {
				option := fields[0]
				fields = fields[1:]
				if wrapperOptionTakesValue(base, option) && len(fields) > 0 {
					fields = fields[1:]
				}
			}
			continue
		}
		break
	}
	return Command{Raw: segment, Argv: fields}
}

func wrapperOptionTakesValue(base, option string) bool {
	switch base {
	case "sudo":
		return option == "-u" || option == "--user" || option == "-g" || option == "--group" || option == "-h" || option == "--host"
	case "env":
		return option == "-u" || option == "--unset" || option == "-C" || option == "--chdir"
	case "nice":
		return option == "-n" || option == "--adjustment"
	case "time":
		return option == "-o" || option == "--output" || option == "-f" || option == "--format"
	default:
		return false
	}
}

func shellFields(raw string) ([]string, string) {
	var fields []string
	var field strings.Builder
	quote := byte(0)
	escaped := false
	hasField := false
	end := len(raw)
	flush := func() {
		if hasField {
			fields = append(fields, field.String())
			field.Reset()
			hasField = false
		}
	}
	for i := 0; i < len(raw); i++ {
		b := raw[i]
		if escaped {
			field.WriteByte(b)
			hasField = true
			escaped = false
			continue
		}
		if quote != 0 {
			if b == quote {
				quote = 0
				hasField = true
			} else if b == '\\' && quote == '"' {
				escaped = true
			} else {
				field.WriteByte(b)
				hasField = true
			}
			continue
		}
		switch b {
		case '\'', '"':
			quote = b
			hasField = true
		case '\\':
			escaped = true
		case ' ', '\t':
			flush()
		case '|', ';':
			flush()
			end = i
			i = len(raw)
		case '&':
			if i+1 < len(raw) && raw[i+1] == '&' {
				flush()
				end = i
				i = len(raw)
			} else {
				field.WriteByte(b)
				hasField = true
			}
		default:
			field.WriteByte(b)
			hasField = true
		}
	}
	flush()
	return fields, strings.TrimSpace(raw[:end])
}

func Blocks(prompts []Prompt, lineCount int) []Block {
	blocks := make([]Block, 0, len(prompts))
	for i, prompt := range prompts {
		end := lineCount - 1
		if i+1 < len(prompts) {
			end = prompts[i+1].Line - 1
		}
		command := prompt.Command
		blocks = append(blocks, Block{Command: &command, StartLine: prompt.Line + 1, EndLine: end})
	}
	return blocks
}
