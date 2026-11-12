package textdetection

import (
	"encoding/json"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/Hanaasagi/magonote/pkg/textdetection/detect"
	"github.com/Hanaasagi/magonote/pkg/textdetection/entity"
	"github.com/Hanaasagi/magonote/pkg/textdetection/internal/resolve"
	"github.com/Hanaasagi/magonote/pkg/textdetection/profile"
	"github.com/Hanaasagi/magonote/pkg/textdetection/screen"
	"github.com/Hanaasagi/magonote/pkg/textdetection/table"
	"github.com/Hanaasagi/magonote/pkg/textdetection/token"
)

type Entity = entity.Entity
type Kind = entity.Kind
type Span = screen.Span
type Table = table.Table

type Exclusion struct {
	Type    string `json:"type"`
	Pattern string `json:"pattern"`
}

type Options struct {
	CustomPatterns []string
	Exclusions     []Exclusion
	Table          *table.Options
	Style          bool
	Profiles       *profile.Options
	Words          bool
	TabWidth       int
}

type Result struct {
	Screen   *screen.Screen
	Entities []entity.Entity
	Tables   []table.Table
	Prompts  []profile.Prompt
	Timings  map[string]time.Duration
}

// Detect parses and recognizes input. Invalid user regular expressions are
// logged and skipped; malformed terminal input never produces an error.
func Detect(input string, opts Options) (*Result, error) {
	start := time.Now()
	parsed := screen.Parse(input, opts.TabWidth)
	timings := map[string]time.Duration{"screen": time.Since(start)}

	chunks := make([][]token.Chunk, len(parsed.Lines))
	for line := range parsed.Lines {
		chunks[line] = token.Chunks(parsed, line)
	}
	prompts := []profile.Prompt{}
	var skip map[int]bool
	var blocks []profile.Block
	if opts.Profiles != nil {
		prompts = profile.DetectPrompts(parsed, opts.Profiles.PromptPatterns)
		skip = make(map[int]bool, len(prompts))
		for _, prompt := range prompts {
			skip[prompt.Line] = true
		}
		blocks = profile.Blocks(prompts, len(parsed.Lines))
	}

	phase := time.Now()
	var tables []table.Table
	if opts.Table != nil {
		tables = table.Detect(parsed, chunks, skip, *opts.Table)
	}
	if tables == nil {
		tables = []table.Table{}
	}
	timings["table"] = time.Since(phase)

	phase = time.Now()
	var profileCandidates []entity.Candidate
	if opts.Profiles != nil {
		profileCtx := &profile.Context{Screen: parsed, Chunks: chunks, Tables: tables}
		profileCandidates = profile.DetectBlocks(blocks, profileCtx)
	}
	timings["profile"] = time.Since(phase)

	phase = time.Now()
	ctx := &detect.Context{
		Screen: parsed, Chunks: chunks, Tables: tables, Prompts: prompts, Blocks: blocks,
		Options: detect.Options{Style: opts.Style, Words: opts.Words},
	}
	detectors := []detect.Detector{detect.NewRegexDetector()}
	if len(opts.CustomPatterns) > 0 {
		detectors = append(detectors, detect.NewCustomDetector(opts.CustomPatterns))
	}
	if opts.Style {
		detectors = append(detectors, detect.NewStyleDetector())
	}
	var candidates []entity.Candidate
	for _, detector := range detectors {
		candidates = append(candidates, detector.Detect(ctx)...)
	}
	candidates = append(candidates, profileCandidates...)
	candidates = append(candidates, detect.NewCellDetector().Detect(ctx)...)
	if opts.Words {
		candidates = append(candidates, detect.NewWordDetector().Detect(ctx)...)
	}
	timings["detect"] = time.Since(phase)

	phase = time.Now()
	exclusions := exclusionSpans(parsed, opts.Exclusions)
	if opts.Profiles != nil && opts.Profiles.ExcludePrompt {
		for _, prompt := range prompts {
			exclusions = append(exclusions, screen.Span{Line: prompt.Line, StartCol: 0, EndCol: prompt.PrefixEnd})
		}
	}
	entities := resolve.Candidates(candidates, exclusions)
	timings["resolve"] = time.Since(phase)
	return &Result{Screen: parsed, Entities: entities, Tables: tables, Prompts: prompts, Timings: timings}, nil
}

func exclusionSpans(s *screen.Screen, rules []Exclusion) []screen.Span {
	var spans []screen.Span
	for _, rule := range rules {
		if rule.Pattern == "" {
			continue
		}
		switch rule.Type {
		case "text":
			for line := range s.Lines {
				plain := s.Lines[line].Plain
				for offset := 0; offset < len(plain); {
					index := strings.Index(plain[offset:], rule.Pattern)
					if index < 0 {
						break
					}
					start := offset + index
					end := start + len(rule.Pattern)
					spans = append(spans, s.SpanOf(line, start, end))
					offset = end
				}
			}
		case "regex":
			re, err := regexp.Compile(rule.Pattern)
			if err != nil {
				slog.Warn("invalid exclusion pattern; skipping", "pattern", rule.Pattern, "error", err)
				continue
			}
			for line := range s.Lines {
				for _, indices := range re.FindAllStringIndex(s.Lines[line].Plain, -1) {
					spans = append(spans, s.SpanOf(line, indices[0], indices[1]))
				}
			}
		}
	}
	return spans
}

func (r *Result) JSON(includeTimings bool) ([]byte, error) {
	type jsonResult struct {
		Entities []entity.Entity          `json:"entities"`
		Tables   []table.Table            `json:"tables"`
		Prompts  []profile.Prompt         `json:"prompts"`
		Timings  map[string]time.Duration `json:"timings,omitempty"`
	}
	output := jsonResult{Entities: r.Entities, Tables: r.Tables, Prompts: r.Prompts}
	if output.Entities == nil {
		output.Entities = []entity.Entity{}
	}
	if output.Tables == nil {
		output.Tables = []table.Table{}
	}
	if output.Prompts == nil {
		output.Prompts = []profile.Prompt{}
	}
	if includeTimings {
		output.Timings = r.Timings
	}
	return json.Marshal(output)
}
