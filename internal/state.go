package internal

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Hanaasagi/magonote/pkg/textdetection"
	"github.com/Hanaasagi/magonote/pkg/textdetection/entity"
	"github.com/Hanaasagi/magonote/pkg/textdetection/profile"
	"github.com/Hanaasagi/magonote/pkg/textdetection/screen"
	"github.com/Hanaasagi/magonote/pkg/textdetection/table"
)

type TableDetectionConfig struct {
	MinLines            int
	MinColumns          int
	ConfidenceThreshold float64
}

type ColorDetectionConfig struct{}

type ProfilesDetectionConfig struct {
	ExcludePrompt  bool
	PromptPatterns []string
}

type ExclusionRule struct {
	Type    string
	Pattern string
}

type ExclusionConfig struct {
	Rules []ExclusionRule
}

type Match struct {
	X        int
	EndCol   int
	Y        int
	Pattern  string
	Text     string
	Hint     *string
	Location *entity.FileLocation
}

func (m Match) Equals(other Match) bool {
	return m.X == other.X && m.Y == other.Y
}

func (m Match) String() string {
	hint := "<undefined>"
	if m.Hint != nil {
		hint = *m.Hint
	}
	return fmt.Sprintf("Match{x:%d,end:%d,y:%d,pattern:%s,text:%s,hint:<%s>}", m.X, m.EndCol, m.Y, m.Pattern, m.Text, hint)
}

type Option interface{ apply(*State) }
type optionFunc func(*State)

func (f optionFunc) apply(s *State) { f(s) }

func WithTableDetection(minLines, minColumns int, confidenceThreshold float64) Option {
	return optionFunc(func(s *State) {
		s.TableDetectionConfig = &TableDetectionConfig{MinLines: minLines, MinColumns: minColumns, ConfidenceThreshold: confidenceThreshold}
	})
}

func WithColorDetection() Option {
	return optionFunc(func(s *State) { s.ColorDetectionConfig = &ColorDetectionConfig{} })
}

func WithProfiles(excludePrompt bool, patterns []string) Option {
	return optionFunc(func(s *State) {
		s.ProfilesConfig = &ProfilesDetectionConfig{ExcludePrompt: excludePrompt, PromptPatterns: patterns}
	})
}

func WithWords() Option {
	return optionFunc(func(s *State) { s.WordDetection = true })
}

func WithExclusionRules(rules []ExclusionRule) Option {
	return optionFunc(func(s *State) { s.ExclusionConfig = &ExclusionConfig{Rules: rules} })
}

type State struct {
	Lines                []string
	Alphabet             string
	CustomPatterns       []string
	TableDetectionConfig *TableDetectionConfig
	ColorDetectionConfig *ColorDetectionConfig
	ProfilesConfig       *ProfilesDetectionConfig
	ExclusionConfig      *ExclusionConfig
	WordDetection        bool

	rawInput string
	screen   *screen.Screen
}

func NewState(text string, alphabet string, patterns []string, opts ...Option) (*State, error) {
	if _, err := NewBuiltinAlphabet(alphabet); err != nil {
		return nil, fmt.Errorf("invalid alphabet %q: %w", alphabet, err)
	}
	parsed := screen.Parse(text)
	state := &State{
		Lines: parsed.PlainLines(), Alphabet: alphabet, CustomPatterns: patterns,
		rawInput: text, screen: parsed,
	}
	for _, opt := range opts {
		opt.apply(state)
	}
	return state, nil
}

func NewStateFromLines(lines []string, alphabet string, patterns []string, opts ...Option) (*State, error) {
	return NewState(strings.Join(lines, "\n"), alphabet, patterns, opts...)
}

func (s *State) Screen() *screen.Screen { return s.screen }

func (s *State) Matches(reverse bool, uniqueLevel int) []Match {
	start := time.Now()
	opts := textdetection.Options{CustomPatterns: s.CustomPatterns, Words: s.WordDetection}
	if s.TableDetectionConfig != nil {
		opts.Table = &table.Options{
			MinRows: s.TableDetectionConfig.MinLines, MinColumns: s.TableDetectionConfig.MinColumns,
			ConfidenceThreshold: s.TableDetectionConfig.ConfidenceThreshold,
		}
	}
	opts.Style = s.ColorDetectionConfig != nil
	if s.ProfilesConfig != nil {
		opts.Profiles = &profile.Options{ExcludePrompt: s.ProfilesConfig.ExcludePrompt, PromptPatterns: s.ProfilesConfig.PromptPatterns}
	}
	if s.ExclusionConfig != nil {
		for _, rule := range s.ExclusionConfig.Rules {
			opts.Exclusions = append(opts.Exclusions, textdetection.Exclusion{Type: rule.Type, Pattern: rule.Pattern})
		}
	}
	result, err := textdetection.Detect(s.rawInput, opts)
	if err != nil {
		slog.Warn("textdetection failed; returning empty matches", "error", err)
		return nil
	}
	s.screen, s.Lines = result.Screen, result.Screen.PlainLines()
	matches := make([]Match, 0, len(result.Entities))
	for _, entity := range result.Entities {
		matches = append(matches, Match{
			X: entity.Span.StartCol, EndCol: entity.Span.EndCol, Y: entity.Span.Line,
			Pattern: string(entity.Kind), Text: entity.Text, Location: entity.Location,
		})
	}
	if uniqueLevel >= 2 {
		matches = s.filterSuperUniqueMatches(matches)
	}
	alphabet, _ := NewBuiltinAlphabet(s.Alphabet)
	hints := alphabet.Hints(len(matches))
	s.assignHints(matches, hints, reverse, uniqueLevel)
	slog.Debug("textdetection completed", "duration_ms", time.Since(start).Milliseconds(), "matches_count", len(matches))
	for _, match := range matches {
		slog.Debug("match", "match", match)
	}
	return matches
}

func (s *State) assignHints(matches []Match, hints []string, reverse bool, uniqueLevel int) {
	if len(matches) == 0 || len(hints) == 0 {
		return
	}
	if !reverse {
		for i, j := 0, len(hints)-1; i < j; i, j = i+1, j-1 {
			hints[i], hints[j] = hints[j], hints[i]
		}
	} else {
		for i, j := 0, len(matches)-1; i < j; i, j = i+1, j-1 {
			matches[i], matches[j] = matches[j], matches[i]
		}
		for i, j := 0, len(hints)-1; i < j; i, j = i+1, j-1 {
			hints[i], hints[j] = hints[j], hints[i]
		}
	}
	if uniqueLevel == 1 {
		s.assignUniqueHints(matches, hints)
	} else {
		s.assignSimpleHints(matches, hints)
	}
	if reverse {
		for i, j := 0, len(matches)-1; i < j; i, j = i+1, j-1 {
			matches[i], matches[j] = matches[j], matches[i]
		}
	}
}

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

func (s *State) filterSuperUniqueMatches(matches []Match) []Match {
	if len(matches) == 0 {
		return matches
	}
	textGroups := make(map[string][]Match)
	for _, match := range matches {
		textGroups[match.Text] = append(textGroups[match.Text], match)
	}
	type textInfo struct {
		text  string
		group []Match
	}
	var textInfos []textInfo
	seen := make(map[string]bool)
	for _, match := range matches {
		if !seen[match.Text] {
			seen[match.Text] = true
			textInfos = append(textInfos, textInfo{text: match.Text, group: textGroups[match.Text]})
		}
	}
	var result []Match
	var selectedLines []int
	for _, info := range textInfos {
		if len(info.group) == 1 {
			result = append(result, info.group[0])
			selectedLines = append(selectedLines, info.group[0].Y)
		}
	}
	for _, info := range textInfos {
		if len(info.group) > 1 {
			selected := s.selectBestMatchWithSpacing(info.group, selectedLines)
			result = append(result, selected)
			selectedLines = append(selectedLines, selected.Y)
		}
	}
	return result
}

func (s *State) selectBestMatchWithSpacing(matches []Match, selectedLines []int) Match {
	if len(matches) == 1 {
		return matches[0]
	}
	totalLines := len(s.Lines)
	middleLine := totalLines / 2
	minSpacing := 2
	bestMatch := matches[0]
	bestDistance := abs(bestMatch.Y - middleLine)
	for i := 1; i < len(matches); i++ {
		distance := abs(matches[i].Y - middleLine)
		if s.isBetterMatchWithSpacing(matches[i], bestMatch, distance, bestDistance, totalLines, selectedLines, minSpacing) {
			bestMatch = matches[i]
			bestDistance = distance
		}
	}
	return bestMatch
}

func (s *State) isBetterMatchWithSpacing(candidate, current Match, candidateDistance, currentDistance, totalLines int, selectedLines []int, minSpacing int) bool {
	candidateHasConflict := s.hasSpacingConflict(candidate.Y, selectedLines, minSpacing)
	currentHasConflict := s.hasSpacingConflict(current.Y, selectedLines, minSpacing)
	if candidateHasConflict && !currentHasConflict {
		return false
	}
	if !candidateHasConflict && currentHasConflict {
		return true
	}
	if totalLines <= 3 {
		return candidate.Y < current.Y
	}
	if abs(candidateDistance-currentDistance) > 2 {
		return candidateDistance < currentDistance
	}
	if candidateDistance == currentDistance {
		return candidate.Y < current.Y
	}
	return candidateDistance < currentDistance
}

func (s *State) hasSpacingConflict(lineNum int, selectedLines []int, minSpacing int) bool {
	for _, selectedLine := range selectedLines {
		if abs(lineNum-selectedLine) < minSpacing {
			return true
		}
	}
	return false
}
