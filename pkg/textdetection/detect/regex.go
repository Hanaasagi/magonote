package detect

import (
	"regexp"
	"strings"

	"github.com/Hanaasagi/magonote/pkg/textdetection/entity"
)

type regexDetector struct{}

func NewRegexDetector() Detector   { return regexDetector{} }
func (regexDetector) Name() string { return "regex" }

type patternDef struct {
	kind     entity.Kind
	re       *regexp.Regexp
	validate func(*Context, int, int, int, entity.Candidate) (entity.Candidate, bool)
}

var commonExtensions = []string{
	"py", "rb", "hs", "lua", "go", "rs", "cpp", "zig", "c", "h", "hpp", "mod", "sum",
	"sh", "bash", "sql", "js", "ts", "tsx", "css", "html", "vim", "json", "yaml", "yml",
	"xml", "toml", "bzl", "ini", "conf", "cfg", "lock", "md", "rst", "txt", "log", "csv",
	"parquet", "so", "dylib", "a", "png", "jpg", "jpeg", "gif", "webp", "svg", "ico", "mov",
	"mp4", "pem", "crt", "key",
}

var pathPattern = `(?P<match>(?:(?:~|\$[A-Za-z_]\w*|\.{1,2}|@?[\w.\-@$\[\]~+%,=]+)?(?:/[\w.\-@$\[\]~+%,=]+)+/?|(?:~|\$[A-Za-z_]\w*|\.{1,2})/)(?::[\w][\w.\-]*(?::\d+)?)?)`

var builtinPatterns = []patternDef{
	{kind: entity.KindURL, re: regexp.MustCompile(`(?i)(?:https?|ftps?|sftp|ssh|git|file|wss?)://[^\s<>"'\x60]+`), validate: validURL},
	{kind: entity.KindURL, re: regexp.MustCompile(`\bgit@[A-Za-z0-9._-]+:[^\s<>"'\x60]+`), validate: validSCPURL},
	{kind: entity.KindEmail, re: regexp.MustCompile(`[\w.+-]+@[\w-]+(?:\.[\w-]+)+`), validate: validEmail},
	{kind: entity.KindPath, re: regexp.MustCompile(pathPattern), validate: validPath},
	{kind: entity.KindDigest, re: regexp.MustCompile(`\bsha(?:1|256|512):[0-9a-f]{40,128}\b`), validate: validDigest},
	{kind: entity.KindUUID, re: regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`), validate: validUUID},
	{kind: entity.KindIPv4, re: regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}(?::\d{1,5})?\b`), validate: validIPv4},
	{kind: entity.KindIPv6Port, re: regexp.MustCompile(`\[[0-9A-Fa-f:.]*:[0-9A-Fa-f:.]*(?:%[\w.]+)?\]:\d{1,5}`), validate: validIPv6},
	{kind: entity.KindIPv6, re: regexp.MustCompile(`\[[0-9A-Fa-f:.]*:[0-9A-Fa-f:.]*(?:%[\w.]+)?\]|[0-9A-Fa-f:.]*:[0-9A-Fa-f:.]*(?:%[\w.]+)?`), validate: validIPv6},
	{kind: entity.KindMAC, re: regexp.MustCompile(`\b(?:[0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2}\b`), validate: validMAC},
	{kind: entity.KindSHA, re: regexp.MustCompile(`\b[0-9a-fA-F]{7,64}\b`), validate: validSHA},
	{kind: entity.KindHexAddress, re: regexp.MustCompile(`\b0x[0-9a-fA-F]+\b`), validate: validHexAddress},
	{kind: entity.KindColor, re: regexp.MustCompile(`#[0-9a-fA-F]{6}(?:[0-9a-fA-F]{2})?\b`), validate: validColor},
	{kind: entity.KindDatetime, re: regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})?\b`), validate: validDatetime},
	{kind: entity.KindDatetime, re: regexp.MustCompile(`\b\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\b`), validate: validDatetime},
	{kind: entity.KindDate, re: regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}\b`), validate: validDate},
	{kind: entity.KindDate, re: regexp.MustCompile(`\b\d{2}/\d{2}/\d{4}\b`), validate: validDate},
	{kind: entity.KindIPFS, re: regexp.MustCompile(`\bQm[1-9A-HJ-NP-Za-km-z]{44}\b`)},
	{kind: entity.KindFilename, re: regexp.MustCompile(`(?i)\b[\w.\-]+\.(?:` + strings.Join(commonExtensions, "|") + `)(?::\d+(?::\d+)?)?\b`), validate: validFilename},
	{kind: entity.KindFilename, re: regexp.MustCompile(`(?i)\B\.(?:gitignore|editorconfig|gitattributes|dockerignore|env|npmrc|yarnrc)(?::\d+(?::\d+)?)?\b`), validate: validFilename},
}

func (regexDetector) Detect(ctx *Context) []entity.Candidate {
	var candidates []entity.Candidate
	for line := range ctx.Screen.Lines {
		plain := ctx.Screen.Lines[line].Plain
		for _, def := range builtinPatterns {
			for _, indices := range def.re.FindAllStringIndex(plain, -1) {
				candidate := candidateFromBytes(ctx, def.kind, line, indices[0], indices[1], entity.ClassSemantic, "regex", 0.9)
				if def.validate != nil {
					var ok bool
					candidate, ok = def.validate(ctx, line, indices[0], indices[1], candidate)
					if !ok {
						continue
					}
				}
				candidates = append(candidates, candidate)
			}
		}
		candidates = append(candidates, specialCandidates(ctx, line)...)
	}
	return candidates
}

type captureDef struct {
	re       *regexp.Regexp
	kind     entity.Kind
	groups   int
	source   string
	location bool
}

var capturePatterns = []captureDef{
	{re: regexp.MustCompile(`^diff --git a/(\S+) b/(\S+)`), kind: entity.KindPath, groups: 2, source: "diff"},
	{re: regexp.MustCompile(`^--- a/(\S+)`), kind: entity.KindPath, groups: 1, source: "diff"},
	{re: regexp.MustCompile(`^\+\+\+ b/(\S+)`), kind: entity.KindPath, groups: 1, source: "diff"},
	{re: regexp.MustCompile(`^test\s+(\S+)\s+\.\.\.\s+(?:ok|FAILED)$`), kind: entity.KindTestName, groups: 1},
	{re: regexp.MustCompile(`^\s*--- (?:PASS|FAIL):\s+(\S+)`), kind: entity.KindTestName, groups: 1},
	{re: regexp.MustCompile(`File "([^"]+)", line (\d+)`), kind: entity.KindPath, groups: 1, location: true},
}

func specialCandidates(ctx *Context, line int) []entity.Candidate {
	plain := ctx.Screen.Lines[line].Plain
	var candidates []entity.Candidate
	for _, def := range capturePatterns {
		for _, indices := range def.re.FindAllStringSubmatchIndex(plain, -1) {
			for group := 1; group <= def.groups; group++ {
				start, end := indices[2*group], indices[2*group+1]
				if start < 0 || end <= start {
					continue
				}
				candidate := candidateFromBytes(ctx, def.kind, line, start, end, entity.ClassSemantic, "regex", 0.95)
				if def.source != "" {
					candidate.Meta = map[string]string{"source": def.source}
				}
				if def.location {
					lineStart, lineEnd := indices[4], indices[5]
					candidate.Meta = map[string]string{"line": plain[lineStart:lineEnd]}
					candidate.Location = fileLocation(candidate.Text, plain[lineStart:lineEnd], "")
				}
				candidates = append(candidates, candidate)
			}
		}
	}
	candidates = append(candidates, markdownCandidates(ctx, line)...)
	return candidates
}
