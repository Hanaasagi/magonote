package internal

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/Hanaasagi/magonote/pkg/ps1parser"
	td "github.com/Hanaasagi/magonote/pkg/textdetection/tabledetection"
)

const (
	// some grid algothims constants
	minLines            = 3
	minColumns          = 3
	confidenceThreshold = 0.8
)

type TableDetectionConfig struct {
	MinLines            int
	MinColumns          int
	ConfidenceThreshold float64
}

type ColorDetectionConfig struct {
}

// ExclusionRule represents a rule for excluding matches
type ExclusionRule struct {
	Type    string // "regex" or "text"
	Pattern string // The pattern or text to exclude
}

// ExclusionConfig holds configuration for match exclusion
type ExclusionConfig struct {
	Rules []ExclusionRule
}

// PS1FilterConfig holds configuration for PS1 prompt filtering
type PS1FilterConfig struct {
	PS1Pattern string // The PS1 pattern to match against
	Enabled    bool   // Whether PS1 filtering is enabled
}

func NewExclusionConfig(rules []ExclusionRule) *ExclusionConfig {
	return &ExclusionConfig{
		Rules: rules,
	}
}

// NewPS1FilterConfig creates a new PS1 filter configuration
func NewPS1FilterConfig(ps1Pattern string) *PS1FilterConfig {
	return &PS1FilterConfig{
		PS1Pattern: ps1Pattern,
		Enabled:    ps1Pattern != "",
	}
}

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

	{"rust_test", `^test\s+(?P<match>[^\s]+)\s+\.\.\.\s+(ok|FAILED)$`},
	{"go_test", `^--- (PASS|FAIL):\s+(?P<match>[^\s]+)`},

	{"path", `(?P<match>([.\w\-@$~\[\]]+)?(/[.\w\-@$\[\]]+)+)`},
	{"color", `#[0-9a-fA-F]{6}`},
	{"uid", `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`},
	{"ipfs", `Qm[0-9a-zA-Z]{44}`},

	// Avoid this regex: it matches substring on strings like "webapp-editor-7fdbfbf4b-k68b7".
	// {"sha", `[0-9a-f]{7,40}`},
	{"sha", `(?:^|[^a-zA-Z0-9_-])(?P<match>[0-9a-f]{7,40})(?:[^a-zA-Z0-9_-]|$)`},

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

// Option defines a functional option for configuring State
type Option interface {
	apply(*State)
}

// optionFunc is a function that implements Option interface
type optionFunc func(*State)

func (f optionFunc) apply(s *State) {
	f(s)
}

// WithTableDetection configures table detection with specified parameters
func WithTableDetection(minLines, minColumns int, confidenceThreshold float64) Option {
	return optionFunc(func(s *State) {
		s.TableDetectionConfig = &TableDetectionConfig{
			MinLines:            minLines,
			MinColumns:          minColumns,
			ConfidenceThreshold: confidenceThreshold,
		}
	})
}

// WithColorDetection enables color detection
func WithColorDetection() Option {
	return optionFunc(func(s *State) {
		s.ColorDetectionConfig = &ColorDetectionConfig{}
	})
}

// WithExclusionRules configures exclusion rules
func WithExclusionRules(rules []ExclusionRule) Option {
	return optionFunc(func(s *State) {
		s.ExclusionConfig = &ExclusionConfig{
			Rules: rules,
		}
	})
}

// State represents the current state of the application
type State struct {
	Lines                []string
	Alphabet             string
	CustomPatterns       []string
	processor            TextProcessor
	styleMatches         []Match
	compiledPatterns     []*CompiledPattern
	cacheValid           bool
	TableDetectionConfig *TableDetectionConfig
	ColorDetectionConfig *ColorDetectionConfig
	ExclusionConfig      *ExclusionConfig
	PS1FilterConfig      *PS1FilterConfig
	originalText         string // Store original text with ANSI codes for PS1 parsing
}

// NewState creates a new state from input text with optional configurations
func NewState(
	text string, alphabet string, patterns []string, opts ...Option,
) *State {
	processor := CreateTextProcessor(text)
	lines, styleMatches, err := processor.Process(text)
	if err != nil {
		// Fallback to plain text processing on error
		lines = strings.Split(text, "\n")
		styleMatches = nil
		processor = NewPlainTextProcessor()
	}

	state := &State{
		Lines:                lines,
		Alphabet:             alphabet,
		CustomPatterns:       patterns,
		processor:            processor,
		styleMatches:         styleMatches,
		cacheValid:           false,
		TableDetectionConfig: nil,
		ColorDetectionConfig: nil,
		ExclusionConfig:      nil,
		PS1FilterConfig:      nil,
		originalText:         text, // Store original text for PS1 parsing
	}

	// Apply all options
	for _, opt := range opts {
		opt.apply(state)
	}

	return state
}

// NewStateFromLines creates a new state from lines with optional configurations (backward compatibility)
func NewStateFromLines(lines []string, alphabet string, patterns []string, opts ...Option) *State {
	text := strings.Join(lines, "\n")
	return NewState(text, alphabet, patterns, opts...)
}

// SetPS1FilterConfig sets the PS1 filter configuration for the state
func (s *State) SetPS1FilterConfig(config *PS1FilterConfig) {
	s.PS1FilterConfig = config
}

// SetPS1Pattern sets the PS1 pattern for filtering (convenience method)
func (s *State) SetPS1Pattern(ps1Pattern string) {
	s.PS1FilterConfig = NewPS1FilterConfig(ps1Pattern)
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

// getLastNonWhitespaceChar returns the last non-whitespace character in a string
func getLastNonWhitespaceChar(s string) rune {
	for i := len(s) - 1; i >= 0; i-- {
		r := rune(s[i])
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			return r
		}
	}
	return 0
}

// fixURLQuotes removes trailing quote from URL if it's quote-enclosed
func fixURLQuotes(url string, originalLine string, startPos int) string {
	if len(url) == 0 {
		return url
	}

	lastChar := url[len(url)-1]
	if lastChar != '\'' && lastChar != '"' {
		return url
	}

	// Find the position before the URL in the original line
	if startPos > 0 {
		beforeURL := originalLine[:startPos]
		lastCharBefore := getLastNonWhitespaceChar(beforeURL)

		if lastCharBefore == rune(lastChar) {
			return url[:len(url)-1]
		}
	}

	return url
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
				captureText := capture.Text

				// Special handling for URL pattern to fix quote issues
				if bestMatch.Pattern.Name == "url" {
					absolutePos := offset + bestMatch.Index + capture.Start
					captureText = fixURLQuotes(captureText, line, absolutePos)
				}

				matches = append(matches, Match{
					X:       offset + bestMatch.Index + capture.Start,
					Y:       y,
					Pattern: bestMatch.Pattern.Name,
					Text:    captureText,
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

	// 1. Add regex-based matches from plain text (highest priority)
	regexStart := time.Now()
	for y, line := range s.Lines {
		lineMatches := s.processLine(y, line, patterns)
		matches = append(matches, lineMatches...)
	}
	regexDuration := time.Since(regexStart)
	slog.Info("regex extraction completed", "duration_ms", regexDuration.Milliseconds(), "matches_count", len(matches))

	if s.ColorDetectionConfig != nil {
		// 2. Add style-based matches, excluding overlaps with regex matches
		if s.styleMatches != nil {
			styleMatches := make([]Match, 0, len(s.styleMatches))
			for _, match := range s.styleMatches {
				if isTextNoise(match.Text) {
					continue
				}
				styleMatches = append(styleMatches, match)
			}

			styleMatches = s.filterOverlappingMatches(styleMatches, matches)
			matches = append(matches, styleMatches...)
		}
	}

	if s.TableDetectionConfig != nil {
		// 3. Add grid-based matches, excluding overlaps with all previous matches
		gridMatches := s.getGridMatches(matches)
		gridMatches = s.filterOverlappingMatches(gridMatches, matches)

		matches = append(matches, gridMatches...)
	}

	if uniqueLevel >= 2 {
		matches = s.filterSuperUniqueMatches(matches)
	}

	// Apply PS1 filtering if configured
	if s.PS1FilterConfig != nil && s.PS1FilterConfig.Enabled {
		matches = s.applyPS1Filter(matches)
	}

	if s.ExclusionConfig != nil {
		matches = s.applyExclusionFilters(matches)
	}

	alphabet, err := NewBuiltinAlphabet(s.Alphabet)
	if err != nil {
		panic(fmt.Sprintf("Failed to create alphabet: %v", err))
	}
	hints := alphabet.Hints(len(matches))

	s.assignHints(matches, hints, reverse, uniqueLevel)
	for _, match := range matches {
		slog.Debug("match", "match", match)
	}
	return matches
}

// filterOverlappingMatches removes matches that overlap with existing matches
func (s *State) filterOverlappingMatches(candidateMatches []Match, existingMatches []Match) []Match {
	// Build position map for overlap detection
	existingPositions := make(map[string]bool, len(existingMatches)*5)
	for _, match := range existingMatches {
		for i := 0; i < len(match.Text); i++ {
			key := fmt.Sprintf("%d-%d", match.Y, match.X+i)
			existingPositions[key] = true
		}
	}

	var filteredMatches []Match
	for _, candidate := range candidateMatches {
		// Check overlap
		overlaps := false
		for i := 0; i < len(candidate.Text); i++ {
			key := fmt.Sprintf("%d-%d", candidate.Y, candidate.X+i)
			if existingPositions[key] {
				overlaps = true
				break
			}
		}

		if !overlaps {
			filteredMatches = append(filteredMatches, candidate)
		}
	}

	return filteredMatches
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
	tableStart := time.Now()
	inputLineCount := len(s.Lines)
	minLines := s.TableDetectionConfig.MinLines
	minColumns := s.TableDetectionConfig.MinColumns
	confidenceThreshold := s.TableDetectionConfig.ConfidenceThreshold

	// Use the new enhanced API with backward compatibility
	detector := td.NewDetector(
		td.WithMinLinesOption(minLines),
		td.WithMinColumnsOption(minColumns),
		td.WithConfidenceThresholdOption(confidenceThreshold),
	)

	tables, err := detector.DetectTables(s.Lines)
	var gridMatches []Match
	if err != nil || len(tables) == 0 {
		// Fallback to legacy API if new API fails
		legacyDetector := td.NewDualRoundDetector(
			td.WithMinLines(minLines),
			td.WithMinColumns(minColumns),
			td.WithConfidenceThreshold(confidenceThreshold),
		)
		segments := legacyDetector.DetectGrids(s.Lines)
		gridMatches = s.processLegacySegments(segments, existingMatches)
	} else {
		gridMatches = s.processNewTables(tables, existingMatches)
	}

	tableDuration := time.Since(tableStart)
	slog.Info("tabledetection completed", "input_lines", inputLineCount, "duration_ms", tableDuration.Milliseconds(), "matches_count", len(gridMatches))
	return gridMatches
}

// processNewTables processes tables from the new API
func (s *State) processNewTables(tables []td.Table, existingMatches []Match) []Match {
	// Build position map for overlap detection
	existingPositions := make(map[string]bool, len(existingMatches)*5)
	for _, match := range existingMatches {
		for i := 0; i < len(match.Text); i++ {
			key := fmt.Sprintf("%d-%d", match.Y, match.X+i)
			existingPositions[key] = true
		}
	}

	var gridMatches []Match
	for _, table := range tables {
		if table.Confidence < confidenceThreshold {
			continue
		}

		// Extract words from cells
		words := s.extractWordsFromTable(table)
		for _, word := range words {
			if isTextNoise(word.Text) {
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

// processLegacySegments processes segments from the legacy API (fallback)
func (s *State) processLegacySegments(segments []td.GridSegment, existingMatches []Match) []Match {

	var gridMatches []Match
	for _, segment := range segments {
		if segment.Confidence < confidenceThreshold {
			continue
		}

		words := s.extractValidWordsLegacy(segment)
		for _, word := range words {
			if isTextNoise(word.Text) {
				continue
			}

			gridMatches = append(gridMatches, Match{
				X:       word.X,
				Y:       word.Y,
				Pattern: "grid",
				Text:    word.Text,
				Hint:    nil,
			})
		}
	}

	return gridMatches
}

// extractWordsFromTable extracts words from the new Table structure
func (s *State) extractWordsFromTable(table td.Table) []GridWord {
	var words []GridWord

	for rowIdx, row := range table.Cells {
		for _, cell := range row {
			// Filter words similar to the original implementation
			if len(cell.Text) > 1 && s.isValidWordForGrid(cell.Text) {
				word := GridWord{
					Text:    cell.Text,
					X:       cell.StartPos,
					Y:       cell.LineIndex,
					LineIdx: rowIdx,
				}
				words = append(words, word)
			}
		}
	}

	return words
}

// extractValidWordsLegacy maintains the original implementation for fallback
func (s *State) extractValidWordsLegacy(segment td.GridSegment) []GridWord {
	var words []GridWord

	for lineIdx, line := range segment.Lines {
		matches := wordPattern.FindAllStringIndex(line, -1)
		for _, match := range matches {
			if match[1]-match[0] > 1 { // Skip single characters
				text := line[match[0]:match[1]]
				if s.isValidWordForGrid(text) {
					words = append(words, GridWord{
						Text:    text,
						X:       match[0],
						Y:       segment.StartLine + lineIdx,
						LineIdx: lineIdx,
					})
				}
			}
		}
	}

	return words
}

// isValidWordForGrid checks if a word should be included in grid matching
func (s *State) isValidWordForGrid(word string) bool {
	// Skip very short words
	if len(word) < 2 {
		return false
	}

	// Skip if it's all digits (likely not interesting for grid matching)
	allDigits := true
	for _, char := range word {
		if !unicode.IsDigit(char) {
			allDigits = false
			break
		}
	}
	if allDigits && len(word) < 4 {
		return false
	}

	// Skip if it contains only special characters
	hasAlphanumeric := false
	for _, char := range word {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			hasAlphanumeric = true
			break
		}
	}

	return hasAlphanumeric
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

// ExclusionRegion represents a region in the text that should be excluded
type ExclusionRegion struct {
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
	Rule      ExclusionRule
}

// applyExclusionFilters applies exclusion rules to filter out unwanted matches
func (s *State) applyExclusionFilters(matches []Match) []Match {
	if s.ExclusionConfig == nil || len(s.ExclusionConfig.Rules) == 0 {
		return matches
	}

	filterStart := time.Now()
	// First, find all exclusion regions in the original text
	exclusionRegions := s.findExclusionRegions()
	if len(exclusionRegions) == 0 {
		return matches
	}

	// Filter matches that overlap with exclusion regions
	filtered := make([]Match, 0, len(matches))
	for _, match := range matches {
		if !s.matchOverlapsWithExclusionRegions(match, exclusionRegions) {
			filtered = append(filtered, match)
		} else {
			slog.Debug("Excluding match", "text", match.Text, "pattern", match.Pattern, "x", match.X, "y", match.Y)
		}
	}

	filterDuration := time.Since(filterStart)
	slog.Info(
		"exclusion filter completed", "duration_ms",
		filterDuration.Milliseconds(),
		"filtered_count", len(matches)-len(filtered),
	)

	return filtered
}

// findExclusionRegions finds all regions in the input text that match exclusion rules
func (s *State) findExclusionRegions() []ExclusionRegion {
	var regions []ExclusionRegion

	for _, rule := range s.ExclusionConfig.Rules {
		switch rule.Type {
		case "text":
			textRegions := s.findTextExclusionRegions(rule)
			regions = append(regions, textRegions...)
		case "regex":
			regexRegions := s.findRegexExclusionRegions(rule)
			regions = append(regions, regexRegions...)
		}
	}

	return regions
}

// findTextExclusionRegions finds regions that contain the specified text
func (s *State) findTextExclusionRegions(rule ExclusionRule) []ExclusionRegion {
	var regions []ExclusionRegion

	// Skip empty patterns to avoid infinite loops
	if rule.Pattern == "" {
		return regions
	}

	for lineIdx, line := range s.Lines {
		startIdx := 0
		for {
			idx := strings.Index(line[startIdx:], rule.Pattern)
			if idx == -1 {
				break
			}

			actualIdx := startIdx + idx
			regions = append(regions, ExclusionRegion{
				StartLine: lineIdx,
				StartCol:  actualIdx,
				EndLine:   lineIdx,
				EndCol:    actualIdx + len(rule.Pattern),
				Rule:      rule,
			})

			startIdx = actualIdx + len(rule.Pattern)
		}
	}

	return regions
}

// findRegexExclusionRegions finds regions that match the specified regex
func (s *State) findRegexExclusionRegions(rule ExclusionRule) []ExclusionRegion {
	var regions []ExclusionRegion

	// Skip empty patterns
	if rule.Pattern == "" {
		return regions
	}

	compiled := globalPatternCache.GetCompiledPattern("exclusion:"+rule.Pattern, rule.Pattern)
	if compiled == nil || compiled.Pattern == nil {
		return regions
	}

	for lineIdx, line := range s.Lines {
		matches := compiled.Pattern.FindAllStringIndex(line, -1)
		for _, match := range matches {
			regions = append(regions, ExclusionRegion{
				StartLine: lineIdx,
				StartCol:  match[0],
				EndLine:   lineIdx,
				EndCol:    match[1],
				Rule:      rule,
			})
		}
	}

	return regions
}

// matchOverlapsWithExclusionRegions checks if a match overlaps with any exclusion region
func (s *State) matchOverlapsWithExclusionRegions(match Match, regions []ExclusionRegion) bool {
	matchStartLine := match.Y
	matchStartCol := match.X
	matchEndLine := match.Y
	matchEndCol := match.X + len(match.Text)

	for _, region := range regions {
		// Check if there's any overlap between the match and the region
		if s.regionsOverlap(
			matchStartLine, matchStartCol, matchEndLine, matchEndCol,
			region.StartLine, region.StartCol, region.EndLine, region.EndCol,
		) {
			slog.Debug("Match overlaps with exclusion region",
				"matchText", match.Text,
				"matchPos", fmt.Sprintf("(%d,%d)-(%d,%d)", matchStartLine, matchStartCol, matchEndLine, matchEndCol),
				"regionPos", fmt.Sprintf("(%d,%d)-(%d,%d)", region.StartLine, region.StartCol, region.EndLine, region.EndCol),
				"ruleType", region.Rule.Type,
				"rulePattern", region.Rule.Pattern)
			return true
		}
	}

	return false
}

// regionsOverlap checks if two rectangular regions overlap
func (s *State) regionsOverlap(
	r1StartLine, r1StartCol, r1EndLine, r1EndCol int,
	r2StartLine, r2StartCol, r2EndLine, r2EndCol int,
) bool {
	// Check if regions are on different lines with no overlap
	if r1EndLine < r2StartLine || r2EndLine < r1StartLine {
		return false
	}

	// If they're on the same line or overlapping lines, check column overlap
	if r1StartLine == r2StartLine && r1EndLine == r2EndLine {
		// Both on the same single line
		return r1EndCol > r2StartCol && r2EndCol > r1StartCol
	}

	// For multi-line or different line scenarios, we consider them overlapping
	// if the line ranges overlap at all
	return true
}

// applyPS1Filter filters out matches that overlap with PS1 prompt regions
func (s *State) applyPS1Filter(matches []Match) []Match {
	filterStart := time.Now()

	// Find PS1 prompt regions in the original text
	promptRegions, err := s.findPS1PromptRegions()
	if err != nil {
		slog.Warn("Failed to parse PS1 pattern, skipping PS1 filtering", "error", err, "pattern", s.PS1FilterConfig.PS1Pattern)
		return matches
	}

	if len(promptRegions) == 0 {
		return matches
	}

	// Filter matches that overlap with prompt regions
	filtered := make([]Match, 0, len(matches))
	for _, match := range matches {
		if !s.matchOverlapsWithPS1Regions(match, promptRegions) {
			filtered = append(filtered, match)
		} else {
			slog.Debug("Excluding match in PS1 prompt region", "text", match.Text, "pattern", match.Pattern, "x", match.X, "y", match.Y)
		}
	}

	filterDuration := time.Since(filterStart)
	slog.Info(
		"PS1 filter completed", "duration_ms",
		filterDuration.Milliseconds(),
		"filtered_count", len(matches)-len(filtered),
		"prompt_regions", len(promptRegions),
	)

	return filtered
}

// findPS1PromptRegions finds all PS1 prompt regions in the processed text
func (s *State) findPS1PromptRegions() ([]PS1PromptRegion, error) {
	if s.PS1FilterConfig == nil || !s.PS1FilterConfig.Enabled || s.PS1FilterConfig.PS1Pattern == "" {
		return nil, nil
	}

	// Use the processed lines to ensure coordinate consistency with match positions
	// This ensures PS1 regions and color matches use the same coordinate system
	processedText := strings.Join(s.Lines, "\n")

	// Use ps1parser to find prompt matches in the processed text
	// Use flexible options to handle spacing and color differences between PS1 pattern and actual output
	options := ps1parser.MatchOptions{
		IgnoreColors:      true,  // Ignore colors since PS1 pattern may differ from actual output
		IgnoreSpacing:     true,  // Allow flexible spacing to handle formatting differences
		CaseSensitive:     false, // Be flexible with case
		MaxLineSpan:       0,     // No line span limit for multiline prompts
		TimeoutPatterns:   false,
		AnchorAtLineStart: true, // Prompts start at line head
	}

	matchResults, err := ps1parser.ParseAndMatch(s.PS1FilterConfig.PS1Pattern, processedText, options)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PS1 pattern: %w", err)
	}

	// Convert ps1parser.MatchResult to our PS1PromptRegion using line-based positions
	regions := make([]PS1PromptRegion, 0, len(matchResults))
	for _, result := range matchResults {
		regions = append(regions, PS1PromptRegion{
			StartLine: result.Position.StartLine,
			StartCol:  result.Position.StartCol,
			EndLine:   result.Position.EndLine,
			EndCol:    result.Position.EndCol,
			Matched:   result.Matched,
			Groups:    result.Groups,
		})
	}

	return regions, nil
}

// matchOverlapsWithPS1Regions checks if a match overlaps with any PS1 prompt region
func (s *State) matchOverlapsWithPS1Regions(match Match, regions []PS1PromptRegion) bool {
	matchStartLine := match.Y
	matchStartCol := match.X
	matchEndLine := match.Y
	matchEndCol := match.X + len(match.Text)

	for _, region := range regions {
		// Check if there's any overlap between the match and the prompt region
		if s.regionsOverlap(
			matchStartLine, matchStartCol, matchEndLine, matchEndCol,
			region.StartLine, region.StartCol, region.EndLine, region.EndCol,
		) {
			slog.Debug("Match overlaps with PS1 prompt region",
				"matchText", match.Text,
				"matchPos", fmt.Sprintf("(%d,%d)-(%d,%d)", matchStartLine, matchStartCol, matchEndLine, matchEndCol),
				"promptPos", fmt.Sprintf("(%d,%d)-(%d,%d)", region.StartLine, region.StartCol, region.EndLine, region.EndCol),
				"promptMatched", region.Matched)
			return true
		}
	}

	return false
}

// PS1PromptRegion represents a region in the text that contains a PS1 prompt
type PS1PromptRegion struct {
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
	Matched   string            // The matched prompt text
	Groups    map[string]string // Named capture groups from PS1 parsing
}

// ExtractValidWords extracts valid words from the grid segment (backward compatibility)
func ExtractValidWords(gs td.GridSegment) []GridWord {
	// Use the new enhanced word extractor
	extractor := td.NewWordExtractor()
	cells := extractor.ExtractCells(gs)

	var words []GridWord
	for _, row := range cells {
		for _, cell := range row {
			word := GridWord{
				Text:    cell.Text,
				X:       cell.StartPos,
				Y:       cell.LineIndex,
				LineIdx: cell.Row,
			}
			words = append(words, word)
		}
	}

	return words
}
