package textdetection

import (
	"sort"
	"strings"
	"unicode"
)

// ============================================================================
// Adaptive Tokenizer Implementation
// ============================================================================

// AdaptiveTokenizer implements smart tokenization with multiple strategies
type AdaptiveTokenizer struct {
	config DetectionConfig
}

// NewAdaptiveTokenizer creates a new adaptive tokenizer with the given configuration
func NewAdaptiveTokenizer(config DetectionConfig) *AdaptiveTokenizer {
	return &AdaptiveTokenizer{config: config}
}

// tokenize implements TokenizationStrategy interface
func (at *AdaptiveTokenizer) tokenize(lines []string, lineIndex int) []Token {
	if lineIndex >= len(lines) {
		return nil
	}

	line := lines[lineIndex]
	basicTokens := at.tokenizeBasic(line)

	// Skip advanced tokenization strategies if using MultiSpaceMode
	// MultiSpaceMode should be simpler and more direct
	if at.config.TokenizationMode == MultiSpaceMode {
		return basicTokens
	}

	// Advanced strategies only for SingleSpaceMode
	// PRIORITY 1: Try left-alignment priority merging for better visual alignment
	// This addresses the alignment ambiguity by prioritizing left-aligned columns
	if at.shouldUseLeftAlignmentMerging(lines, lineIndex, basicTokens) {
		if mergedTokens := at.tokenizeWithLeftAlignmentPriority(lines, lineIndex, basicTokens); mergedTokens != nil {
			return mergedTokens
		}
	}

	// PRIORITY 2: Use projection analysis for compound headers when beneficial
	// Only if left-alignment merging didn't apply
	if at.shouldUseProjectionAnalysis(lines, lineIndex, basicTokens) {
		if projectionTokens := at.tokenizeWithProjection(lines, lineIndex); projectionTokens != nil {
			return projectionTokens
		}
	}

	return basicTokens
}

// ============================================================================
// Basic Tokenization
// ============================================================================

// tokenizeBasic performs basic space-based tokenization
func (at *AdaptiveTokenizer) tokenizeBasic(line string) []Token {
	var tokens []Token
	var current strings.Builder
	var start int
	inToken := false
	consecutiveSpaces := 0

	// Determine minimum spaces required for separation based on mode
	minSpacesForSeparation := MinSpacesForSingleSpaceMode
	if at.config.TokenizationMode == MultiSpaceMode {
		minSpacesForSeparation = MinSpacesForMultiSpaceMode
	}

	for i, char := range line {
		if unicode.IsSpace(char) {
			if inToken {
				consecutiveSpaces++

				// For MultiSpaceMode, only split on 2+ consecutive spaces
				// For SingleSpaceMode, split on any space (original behavior)
				if consecutiveSpaces >= minSpacesForSeparation {
					// Finalize current token
					tokens = append(tokens, Token{
						Text:  current.String(),
						Start: start,
						End:   i - consecutiveSpaces,
					})
					current.Reset()
					inToken = false
					consecutiveSpaces = 0
				}
			} else {
				consecutiveSpaces++
			}
		} else {
			// Non-space character
			if !inToken {
				// Starting a new token
				start = i
				inToken = true
				consecutiveSpaces = 0
			} else if consecutiveSpaces > 0 && consecutiveSpaces < minSpacesForSeparation {
				// We have spaces within a token (only for MultiSpaceMode)
				// Add the spaces to the current token
				for j := 0; j < consecutiveSpaces; j++ {
					current.WriteRune(' ')
				}
				consecutiveSpaces = 0
			}
			current.WriteRune(char)
		}
	}

	// Handle final token
	if inToken {
		tokens = append(tokens, Token{
			Text:  current.String(),
			Start: start,
			End:   len(line) - 1,
		})
	}

	return tokens
}

// ============================================================================
// Projection Analysis Strategy
// ============================================================================

// shouldUseProjectionAnalysis determines if projection analysis should be applied
func (at *AdaptiveTokenizer) shouldUseProjectionAnalysis(lines []string, lineIndex int, originalTokens []Token) bool {
	if len(lines) < MinBoundariesForAnalysis || lineIndex >= len(lines) {
		return false
	}

	line := lines[lineIndex]

	// Check for potential over-segmentation patterns
	singleSpaceCount := at.countSingleSpaceGaps(originalTokens, line)
	if singleSpaceCount == 0 {
		return false
	}

	// Only apply projection analysis if we detect true alignment issues
	// Rather than just any single-space gaps
	if !at.hasRealAlignmentIssues(lines, lineIndex, originalTokens) {
		return false
	}

	// Test projection analysis viability
	projection := at.computeProjection(lines)
	boundaries := at.findBoundaries(projection)

	if len(boundaries) < MinBoundariesForAnalysis {
		return false
	}

	projectionTokens := at.tokenizeLineWithBoundaries(line, boundaries)

	return len(projectionTokens) > 0 &&
		len(projectionTokens) < len(originalTokens) &&
		len(projectionTokens) >= MinBoundariesForAnalysis &&
		len(originalTokens)-len(projectionTokens) <= MinBoundariesForAnalysis &&
		at.improvedAlignment(originalTokens, projectionTokens)
}

// hasRealAlignmentIssues detects if this line has genuine alignment problems
func (at *AdaptiveTokenizer) hasRealAlignmentIssues(lines []string, lineIndex int, originalTokens []Token) bool {
	line := lines[lineIndex]

	// Count consecutive single-space token pairs
	consecutiveSingleSpaces := 0
	maxConsecutive := 0

	for i := 0; i < len(originalTokens)-1; i++ {
		currentEnd := originalTokens[i].End
		nextStart := originalTokens[i+1].Start

		if nextStart-currentEnd == MinTokenWidth &&
			currentEnd+1 < len(line) &&
			line[currentEnd+1] == ' ' {
			consecutiveSingleSpaces++
			if consecutiveSingleSpaces > maxConsecutive {
				maxConsecutive = consecutiveSingleSpaces
			}
		} else {
			consecutiveSingleSpaces = 0
		}
	}

	// Only trigger projection analysis if we have consecutive compound tokens
	if maxConsecutive == 0 {
		return false
	}

	// Check if other lines have similar token counts
	dataLineTokenCounts := []int{}
	for i, otherLine := range lines {
		if i != lineIndex && !at.shouldSkipLine(otherLine) {
			otherTokens := at.tokenizeBasic(otherLine)
			if len(otherTokens) > 0 {
				dataLineTokenCounts = append(dataLineTokenCounts, len(otherTokens))
			}
		}
	}

	if len(dataLineTokenCounts) > 0 {
		avgDataTokens := 0
		for _, count := range dataLineTokenCounts {
			avgDataTokens += count
		}
		avgDataTokens /= len(dataLineTokenCounts)

		// If current line has significantly more tokens than data lines,
		// it's likely a header that should be processed separately
		if float64(len(originalTokens)) > float64(avgDataTokens)*MaxBoundaryRatio {
			return false
		}
	}

	return true
}

// shouldSkipLine determines if a line should be skipped during analysis
func (at *AdaptiveTokenizer) shouldSkipLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed == "" || strings.HasPrefix(trimmed, "$")
}

// tokenizeWithProjection applies projection analysis to tokenize a line
func (at *AdaptiveTokenizer) tokenizeWithProjection(lines []string, lineIndex int) []Token {
	projection := at.computeProjection(lines)
	boundaries := at.findBoundaries(projection)

	if len(boundaries) < MinBoundariesForAnalysis {
		return nil
	}

	return at.tokenizeLineWithBoundaries(lines[lineIndex], boundaries)
}

// computeProjection creates a character-level projection of all lines
func (at *AdaptiveTokenizer) computeProjection(lines []string) []int {
	if len(lines) == 0 {
		return nil
	}

	maxLen := 0
	for _, line := range lines {
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}

	projection := make([]int, maxLen)
	for _, line := range lines {
		for i, char := range line {
			if !unicode.IsSpace(char) {
				projection[i]++
			} else if at.config.TokenizationMode == MultiSpaceMode {
				// In MultiSpaceMode, single spaces within compound tokens should be considered as content
				// Check if this space is surrounded by non-space characters
				if i > 0 && i < len(line)-1 &&
					!unicode.IsSpace(rune(line[i-1])) && !unicode.IsSpace(rune(line[i+1])) {
					// This is likely a space within a compound token, treat as content
					projection[i]++
				}
			}
		}
	}

	return projection
}

// findBoundaries identifies column boundaries from projection data
func (at *AdaptiveTokenizer) findBoundaries(projection []int) []int {
	if len(projection) == 0 {
		return nil
	}

	boundaries := []int{0}
	inColumn := false
	columnStart := 0
	minWidth := MinTokenWidth

	// Adjust minimum width based on tokenization mode
	if at.config.TokenizationMode == MultiSpaceMode {
		minWidth = CompoundTokenMinWidth // Compound tokens need more space
	}

	for i, density := range projection {
		if density > 0 && !inColumn {
			inColumn = true
			columnStart = i
		} else if density == 0 && inColumn {
			if i-columnStart >= minWidth {
				boundaries = append(boundaries, i)
			}
			inColumn = false
		}
	}

	if inColumn && len(projection)-columnStart >= minWidth {
		boundaries = append(boundaries, len(projection))
	}

	return boundaries
}

// tokenizeLineWithBoundaries splits a line based on predefined boundaries
func (at *AdaptiveTokenizer) tokenizeLineWithBoundaries(line string, boundaries []int) []Token {
	var tokens []Token

	for i := 0; i < len(boundaries)-1; i++ {
		start := boundaries[i]
		end := min(boundaries[i+1], len(line))

		if start >= len(line) {
			break
		}

		columnText := strings.TrimSpace(line[start:end])
		if columnText != "" {
			actualStart := start + strings.Index(line[start:end], columnText)
			actualEnd := actualStart + len(columnText) - 1

			tokens = append(tokens, Token{
				Text:  columnText,
				Start: actualStart,
				End:   actualEnd,
			})
		}
	}

	return tokens
}

// countSingleSpaceGaps counts single-space gaps between tokens
func (at *AdaptiveTokenizer) countSingleSpaceGaps(tokens []Token, line string) int {
	// Adjust behavior for different tokenization modes
	if at.config.TokenizationMode == MultiSpaceMode {
		// In MultiSpaceMode, we're less concerned about single-space gaps
		// since single spaces are preserved within tokens
		return 0
	}

	count := 0
	for i := 0; i < len(tokens)-1; i++ {
		currentEnd := tokens[i].End
		nextStart := tokens[i+1].Start

		if nextStart-currentEnd == MinTokenWidth &&
			currentEnd+1 < len(line) &&
			line[currentEnd+1] == ' ' {
			count++
		}
	}
	return count
}

// improvedAlignment checks if projection tokens provide better alignment
func (at *AdaptiveTokenizer) improvedAlignment(originalTokens, projectionTokens []Token) bool {
	if len(projectionTokens) >= len(originalTokens) {
		return false
	}

	shortTokenCount := 0
	for _, token := range originalTokens {
		if len(token.Text) <= MinTokenWidth {
			shortTokenCount++
		}
	}

	reducedCount := len(originalTokens) - len(projectionTokens)
	return reducedCount >= shortTokenCount/2
}

// ============================================================================
// Left Alignment Merging Strategy
// ============================================================================

// shouldUseLeftAlignmentMerging determines if we should try to merge tokens for better left alignment
func (at *AdaptiveTokenizer) shouldUseLeftAlignmentMerging(lines []string, lineIndex int, originalTokens []Token) bool {
	if len(lines) < MinTokenWidth || lineIndex >= len(lines) || len(originalTokens) < MinTokenWidth {
		return false
	}

	// Get token counts for other lines to see if merging would improve consistency
	otherLineCounts := []int{}
	for i, otherLine := range lines {
		if i != lineIndex && !at.shouldSkipLine(otherLine) {
			otherTokens := at.tokenizeBasic(otherLine)
			if len(otherTokens) > 0 {
				otherLineCounts = append(otherLineCounts, len(otherTokens))
			}
		}
	}

	if len(otherLineCounts) == 0 {
		return false
	}

	// Find the most common token count
	countFreq := make(map[int]int)
	for _, count := range otherLineCounts {
		countFreq[count]++
	}

	mostCommonCount := 0
	maxFreq := 0
	for count, freq := range countFreq {
		if freq > maxFreq {
			maxFreq = freq
			mostCommonCount = count
		}
	}

	// If current line has more tokens than the most common count, try merging
	return len(originalTokens) > mostCommonCount && mostCommonCount >= MinTokenWidth
}

// tokenizeWithLeftAlignmentPriority attempts to merge tokens to achieve better left alignment
func (at *AdaptiveTokenizer) tokenizeWithLeftAlignmentPriority(lines []string, lineIndex int, originalTokens []Token) []Token {
	if lineIndex >= len(lines) || len(originalTokens) < MinTokenWidth {
		return nil
	}

	line := lines[lineIndex]

	// Get target column positions from other lines
	targetColumns := at.identifyTargetColumns(lines, lineIndex)
	if len(targetColumns) < MinTokenWidth {
		return nil
	}

	// Try to merge tokens to match target columns
	mergedTokens := at.mergeTokensToColumns(originalTokens, targetColumns, line)

	// Validate that merging improves alignment
	if len(mergedTokens) > 0 && at.validateMergedAlignment(mergedTokens, targetColumns) {
		return mergedTokens
	}

	return nil
}

// identifyTargetColumns analyzes other lines to identify the target column positions
func (at *AdaptiveTokenizer) identifyTargetColumns(lines []string, excludeLineIndex int) []int {
	columnPositions := make(map[int]int) // position -> frequency

	for i, line := range lines {
		if i == excludeLineIndex || at.shouldSkipLine(line) {
			continue
		}

		tokens := at.tokenizeBasic(line)
		for _, token := range tokens {
			columnPositions[token.Start]++
		}
	}

	// Sort positions by frequency and position
	type posFreq struct {
		pos  int
		freq int
	}

	var posList []posFreq
	for pos, freq := range columnPositions {
		posList = append(posList, posFreq{pos, freq})
	}

	// Sort by frequency (descending), then by position (ascending)
	sort.Slice(posList, func(i, j int) bool {
		if posList[i].freq != posList[j].freq {
			return posList[i].freq > posList[j].freq
		}
		return posList[i].pos < posList[j].pos
	})

	// Return the most frequent column positions
	var targetColumns []int
	for _, pf := range posList {
		if pf.freq >= 1 { // At least one other line uses this position
			targetColumns = append(targetColumns, pf.pos)
		}
	}

	return targetColumns
}

// mergeTokensToColumns attempts to merge original tokens to match target column positions
func (at *AdaptiveTokenizer) mergeTokensToColumns(originalTokens []Token, targetColumns []int, line string) []Token {
	if len(originalTokens) == 0 || len(targetColumns) == 0 {
		return nil
	}

	var mergedTokens []Token
	currentMerge := strings.Builder{}
	mergeStart := -1
	tokenIndex := 0

	for _, targetPos := range targetColumns {
		// Find tokens that should be merged into this column
		found := false

		// Look for tokens that start at or near this target position
		for tokenIndex < len(originalTokens) {
			token := originalTokens[tokenIndex]

			// If this token starts at the target position, use it as column start
			if token.Start == targetPos {
				if currentMerge.Len() > 0 {
					// Finish previous merge
					mergedTokens = append(mergedTokens, Token{
						Text:  strings.TrimSpace(currentMerge.String()),
						Start: mergeStart,
						End:   originalTokens[tokenIndex-1].End,
					})
					currentMerge.Reset()
				}

				// Start new column with this token
				currentMerge.WriteString(token.Text)
				mergeStart = token.Start
				tokenIndex++
				found = true
				break
			} else if token.Start < targetPos {
				// This token belongs to previous column or needs merging
				if currentMerge.Len() == 0 {
					mergeStart = token.Start
				} else {
					// Add space between merged tokens
					currentMerge.WriteString(" ")
				}
				currentMerge.WriteString(token.Text)
				tokenIndex++
			} else {
				// token.Start > targetPos, finish current merge and move to next
				break
			}
		}

		if !found && currentMerge.Len() > 0 {
			// No token found at target position, but we have accumulated tokens
			// This suggests the column structure doesn't match
			break
		}
	}

	// Handle remaining tokens
	for tokenIndex < len(originalTokens) {
		token := originalTokens[tokenIndex]
		if currentMerge.Len() == 0 {
			mergeStart = token.Start
		} else {
			currentMerge.WriteString(" ")
		}
		currentMerge.WriteString(token.Text)
		tokenIndex++
	}

	// Finish final merge
	if currentMerge.Len() > 0 {
		mergedTokens = append(mergedTokens, Token{
			Text:  strings.TrimSpace(currentMerge.String()),
			Start: mergeStart,
			End:   originalTokens[len(originalTokens)-1].End,
		})
	}

	return mergedTokens
}

// validateMergedAlignment checks if merged tokens provide better alignment
func (at *AdaptiveTokenizer) validateMergedAlignment(mergedTokens []Token, targetColumns []int) bool {
	if len(mergedTokens) != len(targetColumns) {
		return false
	}

	// Check if merged tokens align well with target columns
	for i, token := range mergedTokens {
		if i < len(targetColumns) {
			targetPos := targetColumns[i]
			if abs(token.Start-targetPos) > at.config.MaxColumnVariance {
				return false
			}
		}
	}

	return true
}
