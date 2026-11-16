package detect

import (
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Hanaasagi/magonote/pkg/textdetection/entity"
)

func editedCandidate(ctx *Context, line, start, end int, candidate entity.Candidate, text string) entity.Candidate {
	candidate.Text = text
	candidate.Span = ctx.Screen.SpanOf(line, start, end)
	return candidate
}

func validURL(ctx *Context, line, start, end int, candidate entity.Candidate) (entity.Candidate, bool) {
	if destinationEnd, ok := markdownDestinationEnd(ctx.Screen.Lines[line].Plain, start); ok && end > destinationEnd {
		return candidate, false
	}
	text := trimTrailing(candidate.Text)
	if text == "" {
		return candidate, false
	}
	u, err := url.Parse(text)
	if err != nil || u.Scheme == "" {
		return candidate, false
	}
	if u.Port() != "" && !validPort(u.Port()) {
		return candidate, false
	}
	if u.Scheme == "file" {
		if u.Path == "" {
			return candidate, false
		}
		return editedCandidate(ctx, line, start, start+len(text), candidate, text), true
	}
	if u.Host == "" {
		switch u.Scheme {
		case "mailto", "tel", "data", "news", "urn", "sms":
			if u.Opaque == "" {
				return candidate, false
			}
		default:
			return candidate, false
		}
	}
	return editedCandidate(ctx, line, start, start+len(text), candidate, text), true
}

func validSCPURL(ctx *Context, line, start, end int, candidate entity.Candidate) (entity.Candidate, bool) {
	text := trimTrailing(candidate.Text)
	colon := strings.IndexByte(text, ':')
	if colon <= 4 || colon == len(text)-1 {
		return candidate, false
	}
	return editedCandidate(ctx, line, start, start+len(text), candidate, text), true
}

func trimTrailing(text string) string {
	for text != "" {
		last := text[len(text)-1]
		if strings.ContainsRune(".,;:!?\"'", rune(last)) {
			text = text[:len(text)-1]
			continue
		}
		pair := map[byte]byte{')': '(', ']': '[', '}': '{'}
		open, ok := pair[last]
		if !ok {
			break
		}
		if strings.Count(text, string(last)) > strings.Count(text, string(open)) {
			text = text[:len(text)-1]
			continue
		}
		break
	}
	return text
}

var pathSuffixPattern = regexp.MustCompile(`:([\w][\w.\-]*)(?::(\d+))?$`)
var versionPattern = regexp.MustCompile(`^\d+(?:\.\d+)*$`)

func fileLocation(path, lineText, columnText string) *entity.FileLocation {
	location := &entity.FileLocation{Path: path}
	if lineText != "" {
		line, err := strconv.Atoi(lineText)
		if err != nil || line <= 0 {
			return nil
		}
		location.Line = line
	}
	if columnText != "" {
		column, err := strconv.Atoi(columnText)
		if err == nil && column > 0 {
			location.Column = column
		}
		// Invalid column is silently ignored; the path and line remain usable.
	}
	return location
}

func validPath(ctx *Context, line, start, end int, candidate entity.Candidate) (entity.Candidate, bool) {
	text := trimTrailing(candidate.Text)
	plain := ctx.Screen.Lines[line].Plain
	if text == "" || strings.Contains(text, "://") || strings.HasPrefix(plain, "diff --git ") || strings.HasPrefix(plain, "--- a/") || strings.HasPrefix(plain, "+++ b/") {
		return candidate, false
	}

	base := text
	meta := map[string]string(nil)
	if match := pathSuffixPattern.FindStringSubmatchIndex(text); match != nil && match[1] == len(text) {
		base = text[:match[0]]
		suffix := text[match[2]:match[3]]
		if addr, err := netip.ParseAddr(suffix); err == nil && addr.Is4() {
			text = base
		} else if isDigits(suffix) {
			meta = map[string]string{"line": suffix}
			column := ""
			if match[4] >= 0 {
				column = text[match[4]:match[5]]
				meta["col"] = column
			}
			candidate.Location = fileLocation(base, suffix, column)
		} else {
			meta = map[string]string{"tag": suffix}
		}
	}
	if base == "" || !plausiblePath(base) {
		return candidate, false
	}
	candidate.Meta = meta
	return editedCandidate(ctx, line, start, start+len(text), candidate, text), true
}

func plausiblePath(path string) bool {
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "$") {
		return true
	}
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")
	if len(parts) < 2 {
		return false
	}
	first := parts[0]
	if first == "" {
		return true
	}
	r, _ := utf8FirstRune(first)
	if !unicode.IsLetter(r) && r != '.' && r != '_' && r != '@' {
		return false
	}
	if versionPattern.MatchString(parts[len(parts)-1]) {
		return false
	}
	if len(parts) == 2 {
		switch strings.ToLower(first) {
		case "text", "application", "image", "audio", "video", "font", "model", "multipart", "message":
			return false
		}
	}
	return true
}

func utf8FirstRune(text string) (rune, int) {
	for _, r := range text {
		return r, 1
	}
	return 0, 0
}

func isDigits(text string) bool {
	if text == "" {
		return false
	}
	for _, r := range text {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func validDigest(ctx *Context, line, start, end int, candidate entity.Candidate) (entity.Candidate, bool) {
	parts := strings.SplitN(candidate.Text, ":", 2)
	want := map[string]int{"sha1": 40, "sha256": 64, "sha512": 128}[parts[0]]
	if len(parts) != 2 || len(parts[1]) != want {
		return candidate, false
	}
	candidate.Meta = map[string]string{"hex": parts[1]}
	return candidate, true
}

func validUUID(ctx *Context, line, start, end int, candidate entity.Candidate) (entity.Candidate, bool) {
	return candidate, true
}

func uniformHexCase(text string) bool {
	lower, upper := false, false
	for _, r := range text {
		lower = lower || (r >= 'a' && r <= 'f')
		upper = upper || (r >= 'A' && r <= 'F')
	}
	return !lower || !upper
}

func validSHA(ctx *Context, line, start, end int, candidate entity.Candidate) (entity.Candidate, bool) {
	plain := ctx.Screen.Lines[line].Plain
	if adjacentWordOrDash(plain, start, end) || !uniformHexCase(candidate.Text) {
		return candidate, false
	}
	if len(candidate.Text) < 40 && !strings.ContainsAny(candidate.Text, "abcdefABCDEF") {
		return candidate, false
	}
	return candidate, true
}

func adjacentWordOrDash(text string, start, end int) bool {
	isWord := func(b byte) bool {
		return b == '-' || b == '_' || b >= '0' && b <= '9' || b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z'
	}
	return start > 0 && isWord(text[start-1]) || end < len(text) && isWord(text[end])
}

func validIPv4(ctx *Context, line, start, end int, candidate entity.Candidate) (entity.Candidate, bool) {
	plain := ctx.Screen.Lines[line].Plain
	if start > 0 && (plain[start-1] == '.' || unicode.IsDigit(rune(plain[start-1]))) || end < len(plain) && (plain[end] == '.' || unicode.IsDigit(rune(plain[end]))) {
		return candidate, false
	}
	host := candidate.Text
	port := ""
	if colon := strings.LastIndexByte(host, ':'); colon >= 0 {
		port = host[colon+1:]
		host = host[:colon]
	}
	addr, err := netip.ParseAddr(host)
	if err != nil || !addr.Is4() || port != "" && !validPort(port) {
		return candidate, false
	}
	if port != "" {
		candidate.Kind = entity.KindIPv4Port
	}
	return candidate, true
}

func validPort(port string) bool {
	n, err := strconv.Atoi(port)
	return err == nil && n >= 0 && n <= 65535
}

func validIPv6(ctx *Context, line, start, end int, candidate entity.Candidate) (entity.Candidate, bool) {
	plain := ctx.Screen.Lines[line].Plain
	if start > 0 && (isASCIIWord(plain[start-1]) || plain[start-1] == '.') {
		return candidate, false
	}
	if end < len(plain) && isASCIIWord(plain[end]) {
		return candidate, false
	}
	if end < len(plain) && plain[end] == '.' && end+1 < len(plain) && plain[end+1] >= '0' && plain[end+1] <= '9' {
		return candidate, false
	}
	text := candidate.Text
	address := text
	if strings.HasPrefix(text, "[") {
		close := strings.Index(text, "]:")
		if close >= 0 {
			address = text[1:close]
			if !validPort(text[close+2:]) {
				return candidate, false
			}
			candidate.Kind = entity.KindIPv6Port
		} else {
			address = strings.TrimSuffix(strings.TrimPrefix(text, "["), "]")
			candidate.Kind = entity.KindIPv6
		}
	}
	if len(address) < 3 || strings.Count(address, ":") < 2 || !strings.ContainsAny(address, "0123456789abcdefABCDEF") {
		return candidate, false
	}
	addr, err := netip.ParseAddr(address)
	if err != nil || !addr.Is6() {
		return candidate, false
	}
	return candidate, true
}

func isASCIIWord(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b == '_'
}

func validMAC(ctx *Context, line, start, end int, candidate entity.Candidate) (entity.Candidate, bool) {
	return candidate, strings.Count(candidate.Text, ":") == 5 || strings.Count(candidate.Text, "-") == 5
}

func validEmail(ctx *Context, line, start, end int, candidate entity.Candidate) (entity.Candidate, bool) {
	text := candidate.Text
	if len(text) == 0 || text[0] == '.' || text[0] == '+' || text[0] == '-' {
		return candidate, false
	}
	at := strings.IndexByte(text, '@')
	if at < 0 {
		return candidate, false
	}
	domain := text[at+1:]
	lastDot := strings.LastIndexByte(domain, '.')
	if lastDot < 0 {
		return candidate, false
	}
	tld := domain[lastDot+1:]
	if len(tld) < 2 || len(tld) > 63 {
		return candidate, false
	}
	return candidate, true
}

func validHexAddress(ctx *Context, line, start, end int, candidate entity.Candidate) (entity.Candidate, bool) {
	return candidate, len(candidate.Text) >= 3
}

func validColor(ctx *Context, line, start, end int, candidate entity.Candidate) (entity.Candidate, bool) {
	plain := ctx.Screen.Lines[line].Plain
	return candidate, start == 0 || !isASCIIWord(plain[start-1])
}

func validDatetime(ctx *Context, line, start, end int, candidate entity.Candidate) (entity.Candidate, bool) {
	text := candidate.Text
	layouts := []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02 15:04:05"}
	for _, layout := range layouts {
		if _, err := time.Parse(layout, text); err == nil {
			return candidate, true
		}
	}
	return candidate, false
}

func validDate(ctx *Context, line, start, end int, candidate entity.Candidate) (entity.Candidate, bool) {
	layout := "2006-01-02"
	if strings.Contains(candidate.Text, "/") {
		layout = "02/01/2006"
	}
	_, err := time.Parse(layout, candidate.Text)
	return candidate, err == nil
}

var filenameSuffixPattern = regexp.MustCompile(`:(\d+)(?::(\d+))?$`)

func validFilename(ctx *Context, line, start, end int, candidate entity.Candidate) (entity.Candidate, bool) {
	if match := filenameSuffixPattern.FindStringSubmatchIndex(candidate.Text); match != nil && match[1] == len(candidate.Text) {
		candidate.Meta = map[string]string{"line": candidate.Text[match[2]:match[3]]}
		column := ""
		if match[4] >= 0 {
			column = candidate.Text[match[4]:match[5]]
			candidate.Meta["col"] = column
		}
		candidate.Location = fileLocation(candidate.Text[:match[0]], candidate.Text[match[2]:match[3]], column)
	}
	return candidate, true
}
