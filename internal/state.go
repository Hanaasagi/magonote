package internal

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	td "github.com/Hanaasagi/magonote/pkg/textdetection"
)

const (
	// some grid algothims constants
	minLines            = 3
	minColumns          = 3
	confidenceThreshold = 0.8
)

// MatchPattern represents a pattern that should be matched
type MatchPattern struct {
	Name    string
	Pattern string
}

var commonExt = []string{
	// Programming languages
	"py", "rb", "hs", "lua",
	"go", "rs", "cpp", "zig", "c",
	"h", "hpp", "h", "lua",
	"sh", "bash", "sql",
	"js", "ts", "tsx", "css", "html",
	"vim",
	// Configuration
	"json", "yaml", "yml", "xml", "toml", "bzl",
	"ini", "conf", "cfg", "lock",
	// Text
	"md", "rst", "txt", "log",
	// Data
	"csv", "parquet",
	// Binary
	"so", "dylib", "a",
	// Media
	"png", "jpg", "jpeg", "gif", "webp", "svg", "ico", "mov", "mp4",
	// Misc
	"pem", "crt", "key",
}

var commonExtPattern = strings.Join(commonExt, "|")

var ExcludePatterns = []MatchPattern{
	// {"bash", `\x1b\[([0-9]{1,2};)?([0-9]{1,2})?m`},
	{"bash", `[\x00-\x1F\x7F]\[([0-9]{1,2};)?([0-9]{1,2})?m`},
}

var BuiltinPatterns = []MatchPattern{
	{"markdown_url", `\[[^]]*\]\(([^)]+)\)`},
	{"url", `(?P<match>(https?://|git@|git://|ssh://|ftp://|file:///)[^ ]+)`},
	{"diff_summary", `diff --git a/([.\w\-@~\[\]]+?/[.\w\-@\[\]]+) b/([.\w\-@~\[\]]+?/[.\w\-@\[\]]+)`},
	{"diff_a", `--- a/([^ ]+)`},
	{"diff_b", `\+\+\+ b/([^ ]+)`},
	{"docker", `sha256:([0-9a-f]{64})`},
	{"path", `(?P<match>([.\w\-@$~\[\]]+)?(/[.\w\-@$\[\]]+)+)`},
	{"color", `#[0-9a-fA-F]{6}`},
	{"uid", `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`},
	{"ipfs", `Qm[0-9a-zA-Z]{44}`},
	{"sha", `[0-9a-f]{7,40}`},
	// IPv4: 192.168.1.1:8080
	{"ipv4_port", `\b\d{1,3}(?:\.\d{1,3}){3}:\d{1,5}\b`},
	{"ipv4", `\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`},

	// IPv6: [2001:db8::1]:443
	{"ipv6_port", `\[[A-Fa-f0-9:]+\]:\d{1,5}`},
	{"ipv6", `[A-f0-9:]+:+[A-f0-9:]+[%\w\d]+`},
	{"address", `0x[0-9a-fA-F]+`},
	// {"file_list_item", `\S+`},
	// {"file_list_item", `\S+(?:\s{2,}|\s*$)`},

	{"filename", `(?i)(?P<match>\b[\w\-.]+\.(?:` + commonExtPattern + `)\b)`},
	{"datetime_iso8601", `\b\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})?\b`},
	{"datetime_common", `\b\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}\b`},
	{"date_dash", `\b\d{4}-\d{2}-\d{2}\b`},
	{"date_slash", `\b\d{2}/\d{2}/\d{4}\b`},
	// {"number", `[0-9]{4,}`},
}

// Match represents a matched pattern in the text
type Match struct {
	X       int
	Y       int
	Pattern string
	Text    string
	Hint    *string
}

// Equals checks if two matches are equal
func (m Match) Equals(other Match) bool {
	return m.X == other.X && m.Y == other.Y
}

// String returns a string representation of the match
func (m Match) String() string {
	hint := "<undefined>"
	if m.Hint != nil {
		hint = *m.Hint
	}
	return fmt.Sprintf("Match{x:%d,y:%d,pattern:%s,text:%s,hint:<%s>}", m.X, m.Y, m.Pattern, m.Text, hint)
}

// CompiledPattern stores a compiled regex with its name
type CompiledPattern struct {
	Name    string
	Pattern *regexp.Regexp
}

// PatternCache provides thread-safe caching of compiled regex patterns
type PatternCache struct {
	cache map[string]*CompiledPattern
	mutex sync.RWMutex
}

var globalPatternCache = &PatternCache{
	cache: make(map[string]*CompiledPattern),
}

// GetCompiledPattern returns a cached compiled pattern or compiles and caches it
func (pc *PatternCache) GetCompiledPattern(name, pattern string) *CompiledPattern {
	key := name + ":" + pattern

	pc.mutex.RLock()
	if compiled, exists := pc.cache[key]; exists {
		pc.mutex.RUnlock()
		return compiled
	}
	pc.mutex.RUnlock()

	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	// Check again after acquiring write lock
	if compiled, exists := pc.cache[key]; exists {
		return compiled
	}

	compiled := &CompiledPattern{
		Name:    name,
		Pattern: regexp.MustCompile(pattern),
	}
	pc.cache[key] = compiled
	return compiled
}

// State represents the current state of the application
type State struct {
	Lines            []string
	Alphabet         string
	CustomPatterns   []string
	compiledPatterns []*CompiledPattern
	cacheValid       bool
}

// NewState creates a new state
func NewState(lines []string, alphabet string, patterns []string) *State {
	return &State{
		Lines:          lines,
		Alphabet:       alphabet,
		CustomPatterns: patterns,
		cacheValid:     false,
	}
}

// getCompiledPatterns returns cached compiled patterns or compiles them
func (s *State) getCompiledPatterns() []*CompiledPattern {
	if s.cacheValid {
		return s.compiledPatterns
	}

	totalLen := len(ExcludePatterns) + len(s.CustomPatterns) + len(BuiltinPatterns)
	patterns := make([]*CompiledPattern, 0, totalLen)

	for _, p := range ExcludePatterns {
		patterns = append(patterns, globalPatternCache.GetCompiledPattern(p.Name, p.Pattern))
	}

	for _, p := range s.CustomPatterns {
		patterns = append(patterns, globalPatternCache.GetCompiledPattern("custom", p))
	}

	for _, p := range BuiltinPatterns {
		patterns = append(patterns, globalPatternCache.GetCompiledPattern(p.Name, p.Pattern))
	}

	s.compiledPatterns = patterns
	s.cacheValid = true
	return patterns
}

// processLine processes a single line and returns matches
func (s *State) processLine(y int, line string, patterns []*CompiledPattern) []Match {
	if len(line) == 0 {
		return nil
	}

	var matches []Match
	offset := 0
	remaining := line

	for len(remaining) > 0 {
		bestMatch := s.findBestMatch(remaining, patterns)
		if bestMatch == nil {
			break
		}

		if bestMatch.Pattern.Name != "bash" {
			captures := s.extractCaptures(bestMatch.Text, bestMatch.Pattern.Pattern)
			for _, capture := range captures {
				matches = append(matches, Match{
					X:       offset + bestMatch.Index + capture.Start,
					Y:       y,
					Pattern: bestMatch.Pattern.Name,
					Text:    capture.Text,
					Hint:    nil,
				})
			}
		}

		// Move past this match
		moveBy := bestMatch.Index + bestMatch.Length
		offset += moveBy
		remaining = remaining[moveBy:]
	}

	return matches
}

type submatch struct {
	Pattern *CompiledPattern
	Index   int
	Length  int
	Text    string
}

// findBestMatch finds the earliest match in the text
func (s *State) findBestMatch(text string, patterns []*CompiledPattern) *submatch {
	var bestMatch *submatch

	for _, pattern := range patterns {
		indices := pattern.Pattern.FindStringSubmatchIndex(text)
		if len(indices) >= 2 {
			match := &submatch{
				Pattern: pattern,
				Index:   indices[0],
				Length:  indices[1] - indices[0],
				Text:    text[indices[0]:indices[1]],
			}

			if bestMatch == nil || match.Index < bestMatch.Index {
				bestMatch = match
			}
		}
	}

	return bestMatch
}

type Capture struct {
	Text  string
	Start int
}

// extractCaptures extracts capture groups from a match
func (s *State) extractCaptures(text string, pattern *regexp.Regexp) []Capture {
	namedMatches := pattern.FindStringSubmatch(text)
	if len(namedMatches) == 0 {
		return []Capture{{Text: text, Start: 0}}
	}

	namedIndices := pattern.FindStringSubmatchIndex(text)
	subexpNames := pattern.SubexpNames()

	// Check for named capture group "match"
	for i, name := range subexpNames {
		if i == 0 || name != "match" || i >= len(namedMatches) || namedMatches[i] == "" {
			continue
		}
		return []Capture{{
			Text:  namedMatches[i],
			Start: namedIndices[i*2] - namedIndices[0],
		}}
	}

	// Use numbered capture groups
	var captures []Capture
	for i := 1; i < len(namedMatches); i++ {
		if namedMatches[i] != "" {
			captures = append(captures, Capture{
				Text:  namedMatches[i],
				Start: namedIndices[i*2] - namedIndices[0],
			})
		}
	}

	if len(captures) == 0 {
		return []Capture{{Text: text, Start: 0}}
	}
	return captures
}

// Matches returns all matches in the text
func (s *State) Matches(reverse bool, uniqueLevel int) []Match {
	patterns := s.getCompiledPatterns()

	matches := make([]Match, 0, len(s.Lines)*2)

	for y, line := range s.Lines {
		lineMatches := s.processLine(y, line, patterns)
		matches = append(matches, lineMatches...)
	}

	// Add grid-based matches
	gridMatches := s.getGridMatches(matches)
	matches = append(matches, gridMatches...)

	if uniqueLevel >= 2 {
		matches = s.filterSuperUniqueMatches(matches)
	}

	alphabet, err := NewBuiltinAlphabet(s.Alphabet)
	if err != nil {
		panic(fmt.Sprintf("Failed to create alphabet: %v", err))
	}
	hints := alphabet.Hints(len(matches))

	s.assignHints(matches, hints, reverse, uniqueLevel)
	return matches
}

// assignHints assigns hints to matches based on options
func (s *State) assignHints(matches []Match, hints []string, reverse bool, uniqueLevel int) {
	if len(matches) == 0 || len(hints) == 0 {
		return
	}

	// In-place reverse operations
	if !reverse {
		// Reverse hints only
		for i, j := 0, len(hints)-1; i < j; i, j = i+1, j-1 {
			hints[i], hints[j] = hints[j], hints[i]
		}
	} else {
		// Reverse matches
		for i, j := 0, len(matches)-1; i < j; i, j = i+1, j-1 {
			matches[i], matches[j] = matches[j], matches[i]
		}
		// Reverse hints
		for i, j := 0, len(hints)-1; i < j; i, j = i+1, j-1 {
			hints[i], hints[j] = hints[j], hints[i]
		}
	}

	if uniqueLevel == 1 {
		s.assignUniqueHints(matches, hints)
	} else {
		s.assignSimpleHints(matches, hints)
	}

	// Reverse matches back if needed
	if reverse {
		for i, j := 0, len(matches)-1; i < j; i, j = i+1, j-1 {
			matches[i], matches[j] = matches[j], matches[i]
		}
	}
}

// assignUniqueHints assigns unique hints to matches with same text
func (s *State) assignUniqueHints(matches []Match, hints []string) {
	previous := make(map[string]string, len(matches)/2)
	hintIndex := len(hints) - 1

	for i := range matches {
		if prevHint, ok := previous[matches[i].Text]; ok {
			matches[i].Hint = &prevHint
		} else if hintIndex >= 0 {
			hint := hints[hintIndex]
			hintIndex--
			matches[i].Hint = &hint
			previous[matches[i].Text] = hint
		}
	}
}

// assignSimpleHints assigns hints to matches sequentially
func (s *State) assignSimpleHints(matches []Match, hints []string) {
	hintIndex := len(hints) - 1
	for i := range matches {
		if hintIndex >= 0 {
			hint := hints[hintIndex]
			hintIndex--
			matches[i].Hint = &hint
		}
	}
}

// filterSuperUniqueMatches filters duplicate matches to show only one per unique text
func (s *State) filterSuperUniqueMatches(matches []Match) []Match {
	if len(matches) == 0 {
		return matches
	}

	// Group matches by text content
	textGroups := make(map[string][]Match)
	for _, match := range matches {
		textGroups[match.Text] = append(textGroups[match.Text], match)
	}

	// Create a deterministic processing order based on the first occurrence of each text
	type textInfo struct {
		text     string
		firstPos int // Y position of first occurrence
		group    []Match
	}

	var textInfos []textInfo
	seen := make(map[string]bool)

	// Process matches in their original order to maintain deterministic sequence
	for _, match := range matches {
		if !seen[match.Text] {
			seen[match.Text] = true
			textInfos = append(textInfos, textInfo{
				text:     match.Text,
				firstPos: match.Y,
				group:    textGroups[match.Text],
			})
		}
	}

	var result []Match
	var selectedLines []int // Track which lines we've already selected

	// First pass: handle single matches in deterministic order
	for _, info := range textInfos {
		if len(info.group) == 1 {
			result = append(result, info.group[0])
			selectedLines = append(selectedLines, info.group[0].Y)
		}
	}

	// Second pass: handle duplicate matches with spacing consideration in deterministic order
	for _, info := range textInfos {
		if len(info.group) > 1 {
			selected := s.selectBestMatchWithSpacing(info.group, selectedLines)
			result = append(result, selected)
			selectedLines = append(selectedLines, selected.Y)
		}
	}

	return result
}

// selectBestMatchWithSpacing selects the best match considering spacing from other selected lines
func (s *State) selectBestMatchWithSpacing(matches []Match, selectedLines []int) Match {
	if len(matches) == 1 {
		return matches[0]
	}

	totalLines := len(s.Lines)
	middleLine := totalLines / 2
	minSpacing := 2 // Minimum spacing between selected matches

	// Find the match closest to the middle line that doesn't conflict with selected lines
	bestMatch := matches[0]
	bestDistance := abs(bestMatch.Y - middleLine)

	for i := 1; i < len(matches); i++ {
		distance := abs(matches[i].Y - middleLine)

		// Check if this match is better than the current best
		if s.isBetterMatchWithSpacing(matches[i], bestMatch, distance, bestDistance, totalLines, selectedLines, minSpacing) {
			bestMatch = matches[i]
			bestDistance = distance
		}
	}

	return bestMatch
}

// isBetterMatchWithSpacing determines if candidate is better considering spacing constraints
func (s *State) isBetterMatchWithSpacing(candidate, current Match, candidateDistance, currentDistance, totalLines int, selectedLines []int, minSpacing int) bool {
	// Check spacing conflicts for both candidate and current
	candidateHasConflict := s.hasSpacingConflict(candidate.Y, selectedLines, minSpacing)
	currentHasConflict := s.hasSpacingConflict(current.Y, selectedLines, minSpacing)

	// If one has conflict and the other doesn't, prefer the one without conflict
	if candidateHasConflict && !currentHasConflict {
		return false
	}
	if !candidateHasConflict && currentHasConflict {
		return true
	}

	// For small number of lines, prefer earlier lines
	if totalLines <= 3 {
		return candidate.Y < current.Y
	}

	// If distances are significantly different, prefer the one closer to middle
	if abs(candidateDistance-currentDistance) > 2 {
		return candidateDistance < currentDistance
	}

	// If both are roughly equal distance from middle, prefer earlier lines
	if candidateDistance == currentDistance {
		return candidate.Y < current.Y
	}

	return candidateDistance < currentDistance
}

// hasSpacingConflict checks if a line number conflicts with selected lines
func (s *State) hasSpacingConflict(lineNum int, selectedLines []int, minSpacing int) bool {
	for _, selectedLine := range selectedLines {
		if abs(lineNum-selectedLine) < minSpacing {
			return true
		}
	}
	return false
}

// getGridMatches detects grid patterns and extracts valid words from them
func (s *State) getGridMatches(existingMatches []Match) []Match {
	detector := td.NewGridDetector(
		td.WithMinLines(minLines),
		td.WithMinColumns(minColumns),
		td.WithConfidenceThreshold(confidenceThreshold),
	)

	segments := detector.DetectGrids(s.Lines)
	if len(segments) == 0 {
		return nil
	}

	// Build position map for overlap detection
	existingPositions := make(map[string]bool, len(existingMatches)*5)
	for _, match := range existingMatches {
		for i := 0; i < len(match.Text); i++ {
			key := fmt.Sprintf("%d-%d", match.Y, match.X+i)
			existingPositions[key] = true
		}
	}

	var gridMatches []Match
	for _, segment := range segments {
		if segment.Confidence < confidenceThreshold {
			continue
		}

		words := ExtractValidWords(segment)
		for _, word := range words {
			if len(word.Text) < 3 || isCommonWord(word.Text) {
				continue
			}

			// Check overlap
			overlaps := false
			for i := 0; i < len(word.Text); i++ {
				key := fmt.Sprintf("%d-%d", word.Y, word.X+i)
				if existingPositions[key] {
					overlaps = true
					break
				}
			}

			if !overlaps {
				gridMatches = append(gridMatches, Match{
					X:       word.X,
					Y:       word.Y,
					Pattern: "grid",
					Text:    word.Text,
					Hint:    nil,
				})
			}
		}
	}

	return gridMatches
}

// GridWord represents a word extracted from a grid segment
type GridWord struct {
	Text    string
	X       int
	Y       int
	LineIdx int
}

// Pre-compiled pattern for better performance
var wordPattern = regexp.MustCompile(`\b[a-zA-Z][a-zA-Z0-9_\-:/]*\b`)

// ExtractValidWords extracts valid words from the grid segment
func ExtractValidWords(gs td.GridSegment) []GridWord {
	var words []GridWord

	for lineIdx, line := range gs.Lines {
		matches := wordPattern.FindAllStringIndex(line, -1)
		for _, match := range matches {
			if match[1]-match[0] > 1 { // Skip single characters
				words = append(words, GridWord{
					Text:    line[match[0]:match[1]],
					X:       match[0],
					Y:       gs.StartLine + lineIdx,
					LineIdx: lineIdx,
				})
			}
		}
	}

	return words
}
