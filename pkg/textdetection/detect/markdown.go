package detect

import (
	"net/url"
	"strings"

	"github.com/Hanaasagi/magonote/pkg/textdetection/entity"
)

func markdownCandidates(ctx *Context, line int) []entity.Candidate {
	plain := ctx.Screen.Lines[line].Plain
	var candidates []entity.Candidate
	for i := 0; i+1 < len(plain); i++ {
		if plain[i] != ']' || plain[i+1] != '(' {
			continue
		}
		start, end, ok := markdownDestination(plain, i+1)
		if !ok || end <= start {
			continue
		}
		text := unescapeMarkdown(plain[start:end])
		if strings.HasPrefix(text, "#") || strings.HasPrefix(text, "?") {
			continue
		}
		kind := entity.KindPath
		if strings.Contains(text, "://") || strings.HasPrefix(text, "git@") || hasURIScheme(text) {
			kind = entity.KindURL
		}
		candidate := candidateFromBytes(ctx, kind, line, start, end, entity.ClassSemantic, "regex", 0.95)
		candidate.Text = text
		if kind == entity.KindURL {
			candidate, ok = validURL(ctx, line, start, end, candidate)
			if ok {
				candidate.Text = text // Markdown parentheses already delimit the URL; undo trimTrailing.
			}
		} else if !strings.Contains(text, "/") {
			candidate, ok = validFilename(ctx, line, start, end, candidate)
		} else {
			candidate, ok = validPath(ctx, line, start, end, candidate)
		}
		if !ok {
			continue
		}
		candidate.Span = ctx.Screen.SpanOf(line, start, end)
		candidate.Confidence = 0.95
		candidate.Meta = map[string]string{"source": "markdown"}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func hasURIScheme(text string) bool {
	u, err := url.Parse(text)
	return err == nil && len(u.Scheme) >= 2
}

func markdownDestinationEnd(line string, start int) (int, bool) {
	for i := 0; i+1 < len(line); i++ {
		if line[i] != ']' || line[i+1] != '(' {
			continue
		}
		destinationStart, destinationEnd, ok := markdownDestination(line, i+1)
		if ok && start >= destinationStart && start < destinationEnd {
			return destinationEnd, true
		}
	}
	return 0, false
}

// markdownDestination parses the destination after the opening '(' of a link.
// It handles nested/escaped parentheses, angle destinations, and optional titles.
func markdownDestination(line string, open int) (int, int, bool) {
	i := open + 1
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	if i >= len(line) {
		return 0, 0, false
	}
	if line[i] == '<' {
		start := i + 1
		for j := start; j < len(line); j++ {
			if line[j] == '\\' && j+1 < len(line) {
				j++
				continue
			}
			if line[j] == '>' {
				return start, j, validMarkdownTail(line, j+1)
			}
		}
		return 0, 0, false
	}

	depth := 0
	for j := i; j < len(line); j++ {
		if line[j] == '\\' && j+1 < len(line) {
			j++
			continue
		}
		switch line[j] {
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return i, j, true
			}
			depth--
		case ' ', '\t':
			if depth == 0 {
				return i, j, validMarkdownTail(line, j)
			}
		}
	}
	return 0, 0, false
}

func validMarkdownTail(line string, index int) bool {
	for index < len(line) && (line[index] == ' ' || line[index] == '\t') {
		index++
	}
	if index >= len(line) {
		return false
	}
	if line[index] == ')' {
		return true
	}
	open := line[index]
	close := open
	if open == '(' {
		close = ')'
	} else if open != '\'' && open != '"' {
		return false
	}
	for i := index + 1; i < len(line); i++ {
		if line[i] == '\\' && i+1 < len(line) {
			i++
			continue
		}
		if line[i] != close {
			continue
		}
		i++
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		return i < len(line) && line[i] == ')'
	}
	return false
}

func unescapeMarkdown(text string) string {
	var result strings.Builder
	for i := 0; i < len(text); i++ {
		if text[i] == '\\' && i+1 < len(text) && strings.ContainsRune("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~", rune(text[i+1])) {
			i++
		}
		result.WriteByte(text[i])
	}
	return result.String()
}
