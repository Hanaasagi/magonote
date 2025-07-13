package textdetection

import (
	"sort"
	"strings"
	"unicode"
)

// Public types and interfaces

// GridSegment represents a segment of text that has grid-like alignment
type GridSegment struct {
	Lines      []string // The lines that form this grid segment
	StartLine  int      // Starting line number in the original text
	EndLine    int      // Ending line number in the original text
	Columns    []int    // Column positions where alignment occurs
	Confidence float64  // Confidence score of this being a grid (0.0 to 1.0)
}

// GridDetector detects grid-like segments in text
type GridDetector struct {
	minLines            int     // Minimum lines required to form a grid
	minColumns          int     // Minimum columns required to form a grid
	alignmentThreshold  float64 // Threshold for column alignment consistency
	confidenceThreshold float64 // Minimum confidence to consider as grid
	maxColumnVariance   int     // Maximum allowed variance in column positions
}

type GridOption func(*GridDetector)

func WithMinLines(n int) GridOption {
	return func(g *GridDetector) {
		g.minLines = n
	}
}

func WithMinColumns(n int) GridOption {
	return func(g *GridDetector) {
		g.minColumns = n
	}
}

func WithAlignmentThreshold(threshold float64) GridOption {
	return func(g *GridDetector) {
		g.alignmentThreshold = threshold
	}
}

func WithConfidenceThreshold(threshold float64) GridOption {
	return func(g *GridDetector) {
		g.confidenceThreshold = threshold
	}
}

func WithMaxColumnVariance(v int) GridOption {
	return func(g *GridDetector) {
		g.maxColumnVariance = v
	}
}

// NewGridDetector creates a new grid detector with default parameters
func NewGridDetector(opts ...GridOption) *GridDetector {
	g := &GridDetector{
		minLines:            2,
		minColumns:          2,
		alignmentThreshold:  0.7,
		confidenceThreshold: 0.6,
		maxColumnVariance:   2,
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// DetectGrids analyzes text lines and returns segments that appear to have grid-like alignment
func (gd *GridDetector) DetectGrids(lines []string) []GridSegment {
	if len(lines) < gd.minLines {
		return nil
	}

	// Tokenize each line and build layout vectors
	analyzer := newLayoutAnalyzer(gd)
	lineData := analyzer.analyzeLines(lines)

	// Find candidate blocks using sliding window
	blockFinder := newBlockFinder(gd)
	candidateBlocks := blockFinder.findCandidateBlocks(lines, lineData)

	// Process each candidate block
	processor := newBlockProcessor(gd)
	var segments []GridSegment
	for _, block := range candidateBlocks {
		if segment := processor.processBlock(block, lineData); segment != nil {
			segments = append(segments, *segment)
		}
	}

	return segments
}

// Internal types and structures

// Token represents a single token with its position information
type Token struct {
	Text  string
	Start int
	End   int
}

// LayoutVector represents the column layout of a line (column start positions)
type LayoutVector []int

// LineData contains analysis results for a single line
type LineData struct {
	tokens []Token
	layout LayoutVector
}

// CandidateBlock represents a potential grid block with similar layout
type CandidateBlock struct {
	StartLine int
	EndLine   int
	Lines     []string
}

// Tokenization Strategy

type TokenizationStrategy interface {
	tokenize(lines []string, lineIndex int) []Token
}

type adaptiveTokenizer struct {
	detector *GridDetector
}

func newAdaptiveTokenizer(detector *GridDetector) *adaptiveTokenizer {
	return &adaptiveTokenizer{detector: detector}
}

func (at *adaptiveTokenizer) tokenize(lines []string, lineIndex int) []Token {
	if lineIndex >= len(lines) {
		return nil
	}

	line := lines[lineIndex]
	basicTokens := at.tokenizeBasic(line)

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

func (at *adaptiveTokenizer) tokenizeBasic(line string) []Token {
	var tokens []Token
	var current strings.Builder
	var start int
	inToken := false

	for i, char := range line {
		if unicode.IsSpace(char) {
			if inToken {
				tokens = append(tokens, Token{
					Text:  current.String(),
					Start: start,
					End:   i - 1,
				})
				current.Reset()
				inToken = false
			}
		} else {
			if !inToken {
				start = i
				inToken = true
			}
			current.WriteRune(char)
		}
	}

	if inToken {
		tokens = append(tokens, Token{
			Text:  current.String(),
			Start: start,
			End:   len(line) - 1,
		})
	}

	return tokens
}

func (at *adaptiveTokenizer) shouldUseProjectionAnalysis(lines []string, lineIndex int, originalTokens []Token) bool {
	if len(lines) < 3 || lineIndex >= len(lines) {
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

	if len(boundaries) < 2 {
		return false
	}

	projectionTokens := at.tokenizeLineWithBoundaries(line, boundaries)

	return len(projectionTokens) > 0 &&
		len(projectionTokens) < len(originalTokens) &&
		len(projectionTokens) >= 3 &&
		len(originalTokens)-len(projectionTokens) <= 3 &&
		at.improvedAlignment(originalTokens, projectionTokens)
}

// hasRealAlignmentIssues detects if this line has genuine alignment problems
// that would benefit from projection analysis (like "Date Modified")
func (at *adaptiveTokenizer) hasRealAlignmentIssues(lines []string, lineIndex int, originalTokens []Token) bool {
	line := lines[lineIndex]

	// Count consecutive single-space token pairs
	consecutiveSingleSpaces := 0
	maxConsecutive := 0

	for i := 0; i < len(originalTokens)-1; i++ {
		currentEnd := originalTokens[i].End
		nextStart := originalTokens[i+1].Start

		if nextStart-currentEnd == 2 &&
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
	// (like "Date Modified" where Date and Modified are adjacent with single space)
	// and the projection analysis actually reduces columns significantly
	if maxConsecutive == 0 {
		return false
	}

	// Check if other lines have similar token counts
	// If header has 6 tokens but data lines have 3, they should be separate
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
		if float64(len(originalTokens)) > float64(avgDataTokens)*1.5 {
			return false
		}
	}

	return true
}

func (at *adaptiveTokenizer) shouldSkipLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed == "" || strings.HasPrefix(trimmed, "$")
}

func (at *adaptiveTokenizer) tokenizeWithProjection(lines []string, lineIndex int) []Token {
	projection := at.computeProjection(lines)
	boundaries := at.findBoundaries(projection)

	if len(boundaries) < 2 {
		return nil
	}

	return at.tokenizeLineWithBoundaries(lines[lineIndex], boundaries)
}

func (at *adaptiveTokenizer) computeProjection(lines []string) []int {
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
			}
		}
	}

	return projection
}

func (at *adaptiveTokenizer) findBoundaries(projection []int) []int {
	if len(projection) == 0 {
		return nil
	}

	boundaries := []int{0}
	inColumn := false
	columnStart := 0
	minWidth := 2

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

func (at *adaptiveTokenizer) tokenizeLineWithBoundaries(line string, boundaries []int) []Token {
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

func (at *adaptiveTokenizer) countSingleSpaceGaps(tokens []Token, line string) int {
	count := 0
	for i := 0; i < len(tokens)-1; i++ {
		currentEnd := tokens[i].End
		nextStart := tokens[i+1].Start

		if nextStart-currentEnd == 2 &&
			currentEnd+1 < len(line) &&
			line[currentEnd+1] == ' ' {
			count++
		}
	}
	return count
}

func (at *adaptiveTokenizer) improvedAlignment(originalTokens, projectionTokens []Token) bool {
	if len(projectionTokens) >= len(originalTokens) {
		return false
	}

	shortTokenCount := 0
	for _, token := range originalTokens {
		if len(token.Text) <= 2 {
			shortTokenCount++
		}
	}

	reducedCount := len(originalTokens) - len(projectionTokens)
	return reducedCount >= shortTokenCount/2
}

// shouldUseLeftAlignmentMerging determines if we should try to merge tokens for better left alignment
func (at *adaptiveTokenizer) shouldUseLeftAlignmentMerging(lines []string, lineIndex int, originalTokens []Token) bool {
	if len(lines) < 2 || lineIndex >= len(lines) || len(originalTokens) < 2 {
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
	return len(originalTokens) > mostCommonCount && mostCommonCount >= 2
}

// tokenizeWithLeftAlignmentPriority attempts to merge tokens to achieve better left alignment
func (at *adaptiveTokenizer) tokenizeWithLeftAlignmentPriority(lines []string, lineIndex int, originalTokens []Token) []Token {
	if lineIndex >= len(lines) || len(originalTokens) < 2 {
		return nil
	}

	line := lines[lineIndex]

	// Get target column positions from other lines
	targetColumns := at.identifyTargetColumns(lines, lineIndex)
	if len(targetColumns) < 2 {
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
func (at *adaptiveTokenizer) identifyTargetColumns(lines []string, excludeLineIndex int) []int {
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
func (at *adaptiveTokenizer) mergeTokensToColumns(originalTokens []Token, targetColumns []int, line string) []Token {
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
func (at *adaptiveTokenizer) validateMergedAlignment(mergedTokens []Token, targetColumns []int) bool {
	if len(mergedTokens) != len(targetColumns) {
		return false
	}

	// Check if merged tokens align well with target columns
	for i, token := range mergedTokens {
		if i < len(targetColumns) {
			targetPos := targetColumns[i]
			if abs(token.Start-targetPos) > at.detector.maxColumnVariance {
				return false
			}
		}
	}

	return true
}

// Layout Analysis

type layoutAnalyzer struct {
	detector  *GridDetector
	tokenizer TokenizationStrategy
}

func newLayoutAnalyzer(detector *GridDetector) *layoutAnalyzer {
	return &layoutAnalyzer{
		detector:  detector,
		tokenizer: newAdaptiveTokenizer(detector),
	}
}

func (la *layoutAnalyzer) analyzeLines(lines []string) []LineData {
	lineData := make([]LineData, len(lines))

	for i, line := range lines {
		if la.shouldSkipLine(line) {
			continue
		}

		tokens := la.tokenizer.tokenize(lines, i)
		lineData[i] = LineData{
			tokens: tokens,
			layout: la.buildLayout(tokens),
		}
	}

	return lineData
}

func (la *layoutAnalyzer) shouldSkipLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed == "" || strings.HasPrefix(trimmed, "$")
}

func (la *layoutAnalyzer) buildLayout(tokens []Token) LayoutVector {
	layout := make(LayoutVector, len(tokens))
	for i, token := range tokens {
		layout[i] = token.Start
	}
	return layout
}

// Alignment Matcher

type alignmentMatcher struct {
	detector *GridDetector
}

func newAlignmentMatcher(detector *GridDetector) *alignmentMatcher {
	return &alignmentMatcher{detector: detector}
}

func (am *alignmentMatcher) areLayoutsSimilar(layout1, layout2 LayoutVector, tokens1, tokens2 []Token) bool {
	if len(layout1) != len(layout2) || len(layout1) == 0 {
		return len(layout1) == len(layout2)
	}

	mismatchCount := 0
	for i := 0; i < len(layout1); i++ {
		leftDiff := abs(tokens1[i].Start - tokens2[i].Start)
		rightDiff := abs(tokens1[i].End - tokens2[i].End)

		if leftDiff <= am.detector.maxColumnVariance || rightDiff <= am.detector.maxColumnVariance {
			continue
		}

		mismatchCount++
		if mismatchCount > 1 {
			return false
		}
	}

	return true
}

// Block Finder

type blockFinder struct {
	detector *GridDetector
	matcher  *alignmentMatcher
}

func newBlockFinder(detector *GridDetector) *blockFinder {
	return &blockFinder{
		detector: detector,
		matcher:  newAlignmentMatcher(detector),
	}
}

func (bf *blockFinder) findCandidateBlocks(lines []string, lineData []LineData) []CandidateBlock {
	var blocks []CandidateBlock

	for i := 0; i < len(lines); {
		if len(lineData[i].layout) == 0 {
			i++
			continue
		}

		block := CandidateBlock{
			StartLine: i,
			EndLine:   i,
			Lines:     []string{lines[i]},
		}

		// Extend block with similar layouts
		for j := i + 1; j < len(lines); j++ {
			if len(lineData[j].layout) == 0 {
				continue
			}

			if bf.matcher.areLayoutsSimilar(lineData[i].layout, lineData[j].layout,
				lineData[i].tokens, lineData[j].tokens) {
				block.EndLine = j
				block.Lines = append(block.Lines, lines[j])
			} else {
				break
			}
		}

		if len(block.Lines) >= bf.detector.minLines {
			blocks = append(blocks, block)
		}

		i = block.EndLine + 1
	}

	return blocks
}

// Block Processor

type blockProcessor struct {
	detector *GridDetector
	scorer   *confidenceScorer
	filter   *heuristicFilter
}

func newBlockProcessor(detector *GridDetector) *blockProcessor {
	return &blockProcessor{
		detector: detector,
		scorer:   newConfidenceScorer(detector),
		filter:   newHeuristicFilter(),
	}
}

func (bp *blockProcessor) processBlock(block CandidateBlock, lineData []LineData) *GridSegment {
	if len(block.Lines) < bp.detector.minLines {
		return nil
	}

	// Extract block tokens
	blockTokens := bp.extractBlockTokens(block, lineData)
	if len(blockTokens) < bp.detector.minLines {
		return nil
	}

	// Detect optimal column alignment
	columns := bp.detectColumns(blockTokens)
	if len(columns) < bp.detector.minColumns {
		return nil
	}

	// Calculate confidence
	confidence := bp.scorer.calculateConfidence(blockTokens, columns)
	if confidence < bp.detector.confidenceThreshold {
		return nil
	}

	// Apply heuristic filters
	if bp.filter.shouldFilterOut(block.Lines) {
		return nil
	}

	return &GridSegment{
		Lines:      block.Lines,
		StartLine:  block.StartLine,
		EndLine:    block.EndLine,
		Columns:    columns,
		Confidence: confidence,
	}
}

func (bp *blockProcessor) extractBlockTokens(block CandidateBlock, lineData []LineData) [][]Token {
	var blockTokens [][]Token
	for i := block.StartLine; i <= block.EndLine; i++ {
		if i < len(lineData) && len(lineData[i].tokens) > 0 {
			blockTokens = append(blockTokens, lineData[i].tokens)
		}
	}
	return blockTokens
}

func (bp *blockProcessor) detectColumns(blockTokens [][]Token) []int {
	if len(blockTokens) == 0 {
		return nil
	}

	maxColumns := 0
	for _, tokens := range blockTokens {
		if len(tokens) > maxColumns {
			maxColumns = len(tokens)
		}
	}

	columns := make([]int, 0, maxColumns)
	for col := 0; col < maxColumns; col++ {
		position := bp.findOptimalPosition(blockTokens, col)
		if position >= 0 {
			columns = append(columns, position)
		}
	}

	return columns
}

func (bp *blockProcessor) findOptimalPosition(blockTokens [][]Token, col int) int {
	leftPositions := []int{}
	rightPositions := []int{}

	for _, tokens := range blockTokens {
		if col < len(tokens) {
			leftPositions = append(leftPositions, tokens[col].Start)
			rightPositions = append(rightPositions, tokens[col].End)
		}
	}

	if len(leftPositions) < 2 {
		return -1
	}

	leftScore := bp.calculatePositionScore(leftPositions)
	rightScore := bp.calculatePositionScore(rightPositions)

	if leftScore >= rightScore {
		return bp.getMedianPosition(leftPositions)
	}
	return bp.getMedianPosition(rightPositions)
}

func (bp *blockProcessor) calculatePositionScore(positions []int) float64 {
	if len(positions) < 2 {
		return 0.0
	}

	mean := 0.0
	for _, pos := range positions {
		mean += float64(pos)
	}
	mean /= float64(len(positions))

	variance := 0.0
	for _, pos := range positions {
		diff := float64(pos) - mean
		variance += diff * diff
	}
	variance /= float64(len(positions))

	maxVariance := float64(bp.detector.maxColumnVariance * bp.detector.maxColumnVariance)
	if variance <= maxVariance {
		return 1.0 - (variance / (maxVariance * 4))
	}

	return 0.0
}

func (bp *blockProcessor) getMedianPosition(positions []int) int {
	if len(positions) == 0 {
		return 0
	}

	sorted := make([]int, len(positions))
	copy(sorted, positions)
	sort.Ints(sorted)
	return sorted[len(sorted)/2]
}

// Confidence Scorer

type confidenceScorer struct {
	detector *GridDetector
}

func newConfidenceScorer(detector *GridDetector) *confidenceScorer {
	return &confidenceScorer{detector: detector}
}

func (cs *confidenceScorer) calculateConfidence(blockTokens [][]Token, columns []int) float64 {
	if len(blockTokens) == 0 || len(columns) == 0 {
		return 0.0
	}

	// Calculate alignment consistency for each column
	totalScore := 0.0
	validColumns := 0
	alignmentVariances := []float64{}

	for colIdx, expectedPos := range columns {
		score := cs.calculateColumnScore(blockTokens, colIdx, expectedPos)
		variance := cs.calculateColumnVariance(blockTokens, colIdx, expectedPos)

		if score > 0 {
			totalScore += score
			validColumns++
			alignmentVariances = append(alignmentVariances, variance)
		}
	}

	if validColumns == 0 {
		return 0.0
	}

	confidence := totalScore / float64(validColumns)

	// Penalize high alignment variance (weak alignment)
	avgVariance := 0.0
	for _, variance := range alignmentVariances {
		avgVariance += variance
	}
	avgVariance /= float64(len(alignmentVariances))

	// Strong penalty for high variance - weak alignment should get low confidence
	maxAcceptableVariance := float64(cs.detector.maxColumnVariance)
	if avgVariance > maxAcceptableVariance {
		variancePenalty := min(0.5, (avgVariance-maxAcceptableVariance)/(maxAcceptableVariance*2))
		confidence -= variancePenalty
	}

	// Additional penalty for inconsistent column counts across rows
	consistency := cs.calculateRowConsistency(blockTokens)
	confidence *= consistency

	// Apply bonuses only if base confidence is reasonable
	if confidence > 0.4 {
		confidence += cs.calculateColumnBonus(len(columns))
		confidence += cs.calculateLineBonus(len(blockTokens))
	}

	return max(0.0, min(1.0, confidence))
}

func (cs *confidenceScorer) calculateColumnScore(blockTokens [][]Token, colIdx int, expectedPos int) float64 {
	alignedLines := 0

	for _, tokens := range blockTokens {
		if colIdx < len(tokens) {
			leftDiff := abs(tokens[colIdx].Start - expectedPos)
			rightDiff := abs(tokens[colIdx].End - expectedPos)

			if leftDiff <= cs.detector.maxColumnVariance || rightDiff <= cs.detector.maxColumnVariance {
				alignedLines++
			}
		}
	}

	return float64(alignedLines) / float64(len(blockTokens))
}

func (cs *confidenceScorer) calculateColumnVariance(blockTokens [][]Token, colIdx int, expectedPos int) float64 {
	var positions []int

	for _, tokens := range blockTokens {
		if colIdx < len(tokens) {
			// Use the better aligned position (left or right)
			leftDiff := abs(tokens[colIdx].Start - expectedPos)
			rightDiff := abs(tokens[colIdx].End - expectedPos)

			if leftDiff <= rightDiff {
				positions = append(positions, tokens[colIdx].Start)
			} else {
				positions = append(positions, tokens[colIdx].End)
			}
		}
	}

	if len(positions) < 2 {
		return 0.0
	}

	// Calculate variance
	mean := 0.0
	for _, pos := range positions {
		mean += float64(pos)
	}
	mean /= float64(len(positions))

	variance := 0.0
	for _, pos := range positions {
		diff := float64(pos) - mean
		variance += diff * diff
	}
	variance /= float64(len(positions))

	return variance
}

func (cs *confidenceScorer) calculateRowConsistency(blockTokens [][]Token) float64 {
	if len(blockTokens) < 2 {
		return 1.0
	}

	// Check consistency of token counts across rows
	tokenCounts := make(map[int]int)
	for _, tokens := range blockTokens {
		tokenCounts[len(tokens)]++
	}

	// Find the most common token count
	maxCount := 0
	for _, count := range tokenCounts {
		if count > maxCount {
			maxCount = count
		}
	}

	// Consistency is the ratio of rows with the most common token count
	consistency := float64(maxCount) / float64(len(blockTokens))

	// Additional penalty if there's too much variation in token counts
	if len(tokenCounts) > 2 {
		consistency *= 0.8 // Penalize high variation
	}

	return consistency
}

func (cs *confidenceScorer) calculateColumnBonus(columnCount int) float64 {
	return min(0.2, float64(columnCount-cs.detector.minColumns)*0.05)
}

func (cs *confidenceScorer) calculateLineBonus(lineCount int) float64 {
	return min(0.1, float64(lineCount-cs.detector.minLines)*0.02)
}

// Heuristic Filter

type heuristicFilter struct{}

func newHeuristicFilter() *heuristicFilter {
	return &heuristicFilter{}
}

func (hf *heuristicFilter) shouldFilterOut(lines []string) bool {
	// TODO:
	return false
}

func abs[T int | int8 | int16 | int32 | int64 | float32 | float64](x T) T {
	if x < 0 {
		return -x
	}
	return x
}
