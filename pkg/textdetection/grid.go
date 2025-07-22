package textdetection

import (
	"sort"
	"strings"
)

// TokenizationMode defines the strategy for splitting text into tokens
type TokenizationMode int

const (
	// SingleSpaceMode splits on any whitespace (current behavior)
	SingleSpaceMode TokenizationMode = iota
	// MultiSpaceMode splits only on 2+ consecutive spaces
	MultiSpaceMode
)

// Public types and interfaces

// GridSegment represents a segment of text that has grid-like alignment
type GridSegment struct {
	Lines      []string         // The lines that form this grid segment
	StartLine  int              // Starting line number in the original text
	EndLine    int              // Ending line number in the original text
	Columns    []int            // Column positions where alignment occurs
	Confidence float64          // Confidence score of this being a grid (0.0 to 1.0)
	Mode       TokenizationMode // Which tokenization mode was used
	Metadata   *SegmentMetadata // Additional information about this segment
}

// SegmentMetadata contains detailed information about how a segment was detected
type SegmentMetadata struct {
	TokenizationMode TokenizationMode
	OriginalTokens   [][]Token // Tokens for each line
	AlignmentData    []ColumnAlignment
	DetectionSource  string // "first_round", "second_round", "merged"
}

// ColumnAlignment contains alignment information for a single column
type ColumnAlignment struct {
	Position    int     // Column start position
	Width       int     // Average column width
	Alignment   string  // "left", "right", "center"
	Consistency float64 // How consistent this column's alignment is (0.0-1.0)
}

// DualRoundDetector performs two-round grid detection with different tokenization strategies
type DualRoundDetector struct {
	firstRoundDetector  *GridDetector
	secondRoundDetector *GridDetector
	mergeStrategy       MergeStrategy
}

// MergeStrategy defines how to combine results from two detection rounds
type MergeStrategy interface {
	MergeResults(firstRound, secondRound []GridSegment, originalLines []string) []GridSegment
}

// DefaultMergeStrategy implements a balanced approach to merging detection results
type DefaultMergeStrategy struct{}

// GridDetector detects grid-like segments in text
type GridDetector struct {
	minLines            int              // Minimum lines required to form a grid
	minColumns          int              // Minimum columns required to form a grid
	alignmentThreshold  float64          // Threshold for column alignment consistency
	confidenceThreshold float64          // Minimum confidence to consider as grid
	maxColumnVariance   int              // Maximum allowed variance in column positions
	tokenizationMode    TokenizationMode // How to split text into tokens
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

func WithTokenizationMode(mode TokenizationMode) GridOption {
	return func(g *GridDetector) {
		g.tokenizationMode = mode
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
		tokenizationMode:    SingleSpaceMode, // Default to original behavior
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// NewDualRoundDetector creates a new dual-round detector with optimized settings for each round
func NewDualRoundDetector(opts ...GridOption) *DualRoundDetector {
	// First round: Multi-space tokenization, more tolerant settings
	firstRoundOpts := append(opts,
		WithTokenizationMode(MultiSpaceMode),
		WithConfidenceThreshold(0.4), // Lower threshold for first round
		WithMaxColumnVariance(3),     // More tolerant variance
	)

	// Second round: Single-space tokenization, standard settings
	secondRoundOpts := append(opts,
		WithTokenizationMode(SingleSpaceMode),
		WithConfidenceThreshold(0.6), // Standard threshold
		WithMaxColumnVariance(2),     // Standard variance
	)

	return &DualRoundDetector{
		firstRoundDetector:  NewGridDetector(firstRoundOpts...),
		secondRoundDetector: NewGridDetector(secondRoundOpts...),
		mergeStrategy:       &DefaultMergeStrategy{},
	}
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

	// Intelligent segment merging and optimization
	segments = gd.mergeConsecutiveSegments(segments, lines)
	segments = gd.optimizeSegments(segments, lines)

	return segments
}

// DetectGrids performs dual-round grid detection and returns the optimal results
func (drd *DualRoundDetector) DetectGrids(lines []string) []GridSegment {
	// First round: Multi-space tokenization
	firstRoundResults := drd.firstRoundDetector.DetectGrids(lines)
	for i := range firstRoundResults {
		firstRoundResults[i].Mode = MultiSpaceMode
		if firstRoundResults[i].Metadata == nil {
			firstRoundResults[i].Metadata = &SegmentMetadata{}
		}
		firstRoundResults[i].Metadata.DetectionSource = "first_round"
		firstRoundResults[i].Metadata.TokenizationMode = MultiSpaceMode
	}

	// Second round: Single-space tokenization
	secondRoundResults := drd.secondRoundDetector.DetectGrids(lines)
	for i := range secondRoundResults {
		secondRoundResults[i].Mode = SingleSpaceMode
		if secondRoundResults[i].Metadata == nil {
			secondRoundResults[i].Metadata = &SegmentMetadata{}
		}
		secondRoundResults[i].Metadata.DetectionSource = "second_round"
		secondRoundResults[i].Metadata.TokenizationMode = SingleSpaceMode
	}

	// Merge results using the configured strategy
	return drd.mergeStrategy.MergeResults(firstRoundResults, secondRoundResults, lines)
}

// MergeResults implements the default strategy for combining detection results
func (dms *DefaultMergeStrategy) MergeResults(firstRound, secondRound []GridSegment, originalLines []string) []GridSegment {
	var result []GridSegment

	// If no results from either round, return empty
	if len(firstRound) == 0 && len(secondRound) == 0 {
		return result
	}

	// Simple case: only one round has results
	if len(firstRound) == 0 {
		return secondRound
	}
	if len(secondRound) == 0 {
		return firstRound
	}

	// Complex case: both rounds have results, need intelligent merging
	return dms.performIntelligentMerge(firstRound, secondRound, originalLines)
}

// performIntelligentMerge implements the core logic for choosing optimal results
func (dms *DefaultMergeStrategy) performIntelligentMerge(firstRound, secondRound []GridSegment, originalLines []string) []GridSegment {
	var result []GridSegment

	// For each potential grid region, choose the better detection
	coveredLines := make(map[int]bool)

	// Process segments by quality score (confidence * column_count)
	type ScoredSegment struct {
		segment GridSegment
		score   float64
		source  string
	}

	var allSegments []ScoredSegment

	// Score first round segments
	for _, seg := range firstRound {
		score := dms.calculateSegmentScore(seg, originalLines)
		allSegments = append(allSegments, ScoredSegment{
			segment: seg,
			score:   score,
			source:  "first_round",
		})
	}

	// Score second round segments
	for _, seg := range secondRound {
		score := dms.calculateSegmentScore(seg, originalLines)
		allSegments = append(allSegments, ScoredSegment{
			segment: seg,
			score:   score,
			source:  "second_round",
		})
	}

	// Sort by score (highest first)
	sort.Slice(allSegments, func(i, j int) bool {
		return allSegments[i].score > allSegments[j].score
	})

	// Greedily select non-overlapping segments with highest scores
	for _, scored := range allSegments {
		seg := scored.segment

		// Check if this segment overlaps with already selected segments
		hasOverlap := false
		for line := seg.StartLine; line <= seg.EndLine; line++ {
			if coveredLines[line] {
				hasOverlap = true
				break
			}
		}

		if !hasOverlap {
			// Accept this segment
			result = append(result, seg)

			// Mark lines as covered
			for line := seg.StartLine; line <= seg.EndLine; line++ {
				coveredLines[line] = true
			}
		}
	}

	return result
}

// calculateSegmentScore computes a quality score for a detected segment
func (dms *DefaultMergeStrategy) calculateSegmentScore(segment GridSegment, originalLines []string) float64 {
	// Base score from confidence
	score := segment.Confidence

	// Bonus for having a reasonable number of columns
	columnBonus := 0.0
	if len(segment.Columns) >= 2 && len(segment.Columns) <= 10 {
		// Sweet spot: 2-10 columns
		columnBonus = 0.2
		if len(segment.Columns) >= 3 && len(segment.Columns) <= 7 {
			// Even better: 3-7 columns
			columnBonus = 0.3
		}
	}
	score += columnBonus

	// Bonus for more lines (larger tables are generally better)
	lineBonus := min(0.2, float64(len(segment.Lines)-2)*0.02)
	score += lineBonus

	// Penalty for too many columns (over-segmentation)
	if len(segment.Columns) > 12 {
		oversegmentationPenalty := float64(len(segment.Columns)-12) * 0.05
		score -= min(0.3, oversegmentationPenalty)
	}

	// Bonus for first round if it successfully handles compound tokens
	if segment.Mode == MultiSpaceMode {
		if dms.hasCompoundTokens(segment, originalLines) {
			score += 0.15 // Bonus for handling compound tokens well
		}
	}

	// Bonus for second round if it provides good granularity
	if segment.Mode == SingleSpaceMode {
		if dms.hasGoodGranularity(segment, originalLines) {
			score += 0.1 // Bonus for good granular detection
		}
	}

	return max(0.0, min(2.0, score)) // Clamp to reasonable range
}

// hasCompoundTokens checks if the segment likely benefits from multi-space tokenization
// Uses statistical analysis of token patterns rather than content matching
func (dms *DefaultMergeStrategy) hasCompoundTokens(segment GridSegment, originalLines []string) bool {
	if len(segment.Lines) == 0 {
		return false
	}

	// Analyze token length distribution and spacing patterns
	// Multi-space tokenization is beneficial when we have:
	// 1. Mixed token lengths (some very short, some longer)
	// 2. Consistent internal spacing within logical units
	// 3. Clear separation between major columns

	totalTokens := 0
	shortTokens := 0      // tokens with 1-3 characters
	mediumTokens := 0     // tokens with 4-8 characters
	longTokens := 0       // tokens with 9+ characters
	singleCharSpaces := 0 // single-space gaps between tokens

	analyzer := newLayoutAnalyzer(&GridDetector{tokenizationMode: SingleSpaceMode})
	lineData := analyzer.analyzeLines(segment.Lines)

	for _, data := range lineData {
		totalTokens += len(data.tokens)

		for i, token := range data.tokens {
			switch {
			case len(token.Text) <= 3:
				shortTokens++
			case len(token.Text) <= 8:
				mediumTokens++
			default:
				longTokens++
			}

			// Check for single-space gaps (suggesting compound tokens)
			if i < len(data.tokens)-1 {
				gap := data.tokens[i+1].Start - token.End - 1
				if gap == 1 {
					singleCharSpaces++
				}
			}
		}
	}

	if totalTokens == 0 {
		return false
	}

	// Multi-space tokenization is beneficial when:
	// 1. High ratio of short tokens (suggesting over-segmentation)
	shortTokenRatio := float64(shortTokens) / float64(totalTokens)

	// 2. Presence of single-space gaps (suggesting compound tokens)
	singleSpaceRatio := float64(singleCharSpaces) / float64(max(1, totalTokens-len(lineData)))

	// 3. Mixed token length distribution (indicating heterogeneous content)
	hasLengthVariety := shortTokens > 0 && mediumTokens > 0 && longTokens > 0

	return shortTokenRatio > 0.3 && singleSpaceRatio > 0.2 && hasLengthVariety
}

// hasGoodGranularity checks if the segment benefits from fine-grained tokenization
func (dms *DefaultMergeStrategy) hasGoodGranularity(segment GridSegment, originalLines []string) bool {
	// Single-space tokenization is good when we have simple, well-separated data
	if len(segment.Columns) >= 3 && len(segment.Columns) <= 8 {
		// Check if the columns are well-spaced
		for i := 1; i < len(segment.Columns); i++ {
			gap := segment.Columns[i] - segment.Columns[i-1]
			if gap < 2 { // Very tight spacing suggests over-segmentation
				return false
			}
		}
		return true
	}
	return false
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

// Layout Analysis

type layoutAnalyzer struct {
	detector  *GridDetector
	tokenizer TokenizationStrategy
}

func newLayoutAnalyzer(detector *GridDetector) *layoutAnalyzer {
	return &layoutAnalyzer{
		detector: detector,
		tokenizer: NewAdaptiveTokenizer(DetectionConfig{
			MinLines:            detector.minLines,
			MinColumns:          detector.minColumns,
			AlignmentThreshold:  detector.alignmentThreshold,
			ConfidenceThreshold: detector.confidenceThreshold,
			MaxColumnVariance:   detector.maxColumnVariance,
			TokenizationMode:    detector.tokenizationMode,
		}),
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
	if len(layout1) == 0 && len(layout2) == 0 {
		return true
	}
	if len(layout1) == 0 || len(layout2) == 0 {
		return false
	}

	// For exact token count matches, use traditional precise matching (original behavior)
	if len(layout1) == len(layout2) {
		return am.traditionalTokenSimilarity(layout1, layout2, tokens1, tokens2)
	}

	// INTELLIGENT HEADER-DATA COMPATIBILITY: Apply column boundary analysis
	// only for cases where token counts differ (header vs data scenarios)
	// This solves the "header vs data" heterogeneity problem while preserving original behavior

	boundaries1 := am.extractColumnBoundaries(tokens1)
	boundaries2 := am.extractColumnBoundaries(tokens2)

	return am.areColumnBoundariesCompatible(boundaries1, boundaries2)
}

// extractColumnBoundaries identifies the significant column boundaries in a line
func (am *alignmentMatcher) extractColumnBoundaries(tokens []Token) []int {
	if len(tokens) == 0 {
		return []int{}
	}

	boundaries := []int{tokens[0].Start} // Always start with first token position

	for i := 1; i < len(tokens); i++ {
		prevEnd := tokens[i-1].End
		currentStart := tokens[i].Start
		gap := currentStart - prevEnd

		// Significant gap indicates a new column boundary
		// Use more conservative gap thresholds to avoid false positives
		minGap := 3 // Increased from 2 to be more conservative
		if am.detector.tokenizationMode == MultiSpaceMode {
			minGap = 5 // Increased from 4 for MultiSpace mode
		}

		if gap >= minGap {
			boundaries = append(boundaries, currentStart)
		}
	}

	return boundaries
}

// areColumnBoundariesCompatible checks if two sets of column boundaries represent compatible table structures
func (am *alignmentMatcher) areColumnBoundariesCompatible(boundaries1, boundaries2 []int) bool {
	if len(boundaries1) == 0 || len(boundaries2) == 0 {
		return false
	}

	// More conservative boundary count matching
	minBoundaries := min(len(boundaries1), len(boundaries2))
	maxBoundaries := max(len(boundaries1), len(boundaries2))

	// Tighter constraint: reject if difference is too large
	if float64(maxBoundaries) > float64(minBoundaries)*1.5 { // Reduced from 2x to 1.5x
		return false
	}

	// Only apply boundary compatibility for cases where it makes sense
	// i.e., when we have a reasonable number of boundaries (2-10 range)
	// Reduced minimum from 3 to 2 to handle smaller grids
	if minBoundaries < 2 || maxBoundaries > 10 {
		return false
	}

	// Check alignment of major boundaries
	alignedBoundaries := 0
	tolerance := am.detector.maxColumnVariance

	// For each boundary in the smaller set, find a corresponding boundary in the larger set
	smaller := boundaries1
	larger := boundaries2
	if len(boundaries2) < len(boundaries1) {
		smaller = boundaries2
		larger = boundaries1
	}

	for _, smallBoundary := range smaller {
		for _, largeBoundary := range larger {
			if abs(smallBoundary-largeBoundary) <= tolerance {
				alignedBoundaries++
				break
			}
		}
	}

	// Require moderate alignment ratio: 75% (balanced between precision and coverage)
	alignmentRatio := float64(alignedBoundaries) / float64(len(smaller))
	return alignmentRatio >= 0.75
}

// traditionalTokenSimilarity provides fallback for cases where column boundary analysis isn't applicable
func (am *alignmentMatcher) traditionalTokenSimilarity(layout1, layout2 LayoutVector, tokens1, tokens2 []Token) bool {
	// For cases with same token count, use the original precise matching
	if len(layout1) == len(layout2) {
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

	// For different token counts, apply the overlap analysis (existing logic)
	// But make it more lenient for table header-data scenarios
	positions1 := make(map[int]bool)
	positions2 := make(map[int]bool)

	for _, token := range tokens1 {
		for offset := -am.detector.maxColumnVariance; offset <= am.detector.maxColumnVariance; offset++ {
			positions1[token.Start+offset] = true
			positions1[token.End+offset] = true
		}
	}

	for _, token := range tokens2 {
		for offset := -am.detector.maxColumnVariance; offset <= am.detector.maxColumnVariance; offset++ {
			positions2[token.Start+offset] = true
			positions2[token.End+offset] = true
		}
	}

	overlapCount := 0
	totalPositions1 := len(tokens1) * 2
	totalPositions2 := len(tokens2) * 2

	for pos := range positions1 {
		if positions2[pos] {
			overlapCount++
		}
	}

	minTotalPositions := min(totalPositions1, totalPositions2)
	overlapRatio := float64(overlapCount) / float64(minTotalPositions)

	// More lenient ratio for table scenarios
	minTokenRatio := 0.2
	if am.detector.tokenizationMode == MultiSpaceMode {
		minTokenRatio = 0.15 // Even more tolerant for compound token scenarios
	}

	tokenRatio := float64(min(len(tokens1), len(tokens2))) / float64(max(len(tokens1), len(tokens2)))

	return overlapRatio >= 0.3 && tokenRatio >= minTokenRatio
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
				// Empty line terminates the current block to prevent merging separate tables
				break
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

// mergeConsecutiveSegments identifies and merges consecutive segments with compatible column structures
func (gd *GridDetector) mergeConsecutiveSegments(segments []GridSegment, originalLines []string) []GridSegment {
	if len(segments) <= 1 {
		return segments
	}

	var result []GridSegment
	current := segments[0]

	for i := 1; i < len(segments); i++ {
		next := segments[i]

		// Check if segments are consecutive (allowing small gaps)
		if gd.areSegmentsConsecutive(current, next, originalLines) {
			// Check if column structures are compatible for merging
			if gd.areColumnsCompatibleForMerging(current, next, originalLines) {
				// Merge the segments
				merged := gd.mergeSegments(current, next, originalLines)
				current = merged
				continue
			}
		}

		// Cannot merge, add current to result and move to next
		result = append(result, current)
		current = next
	}

	// Add the final segment
	result = append(result, current)
	return result
}

// areSegmentsConsecutive checks if two segments are consecutive in the text
func (gd *GridDetector) areSegmentsConsecutive(seg1, seg2 GridSegment, originalLines []string) bool {
	gap := seg2.StartLine - seg1.EndLine

	// Only allow direct continuation (gap == 1) to prevent merging across empty lines
	// This fixes the IP table panic issue where empty lines should separate tables
	if gap != 1 {
		return false
	}

	// No additional gap line checks needed since we require direct continuation
	return true
}

// areColumnsCompatibleForMerging checks if two segments have compatible column structures
func (gd *GridDetector) areColumnsCompatibleForMerging(seg1, seg2 GridSegment, originalLines []string) bool {
	if len(seg1.Columns) == 0 || len(seg2.Columns) == 0 {
		return false
	}

	// Check column count compatibility (allow some variance)
	minCols := min(len(seg1.Columns), len(seg2.Columns))
	maxCols := max(len(seg1.Columns), len(seg2.Columns))

	// If column counts are too different, don't merge
	if float64(maxCols) > float64(minCols)*1.5 {
		return false
	}

	// Check column position alignment for the common columns
	alignedColumns := 0
	tolerance := gd.maxColumnVariance * 2 // More tolerant for merging

	for i := 0; i < minCols; i++ {
		col1 := seg1.Columns[i]
		col2 := seg2.Columns[i]

		if abs(col1-col2) <= tolerance {
			alignedColumns++
		}
	}

	// Require at least 70% of columns to align
	alignmentRatio := float64(alignedColumns) / float64(minCols)
	return alignmentRatio >= 0.7
}

// mergeSegments combines two compatible segments into one
func (gd *GridDetector) mergeSegments(seg1, seg2 GridSegment, originalLines []string) GridSegment {
	// Combine lines directly since we only merge consecutive segments now
	var allLines []string
	allLines = append(allLines, seg1.Lines...)
	allLines = append(allLines, seg2.Lines...)

	// Determine optimal columns by analyzing all lines together
	analyzer := newLayoutAnalyzer(gd)
	lineData := analyzer.analyzeLines(allLines)

	// Build tokens for the merged segment
	var blockTokens [][]Token
	for _, data := range lineData {
		if len(data.tokens) > 0 {
			blockTokens = append(blockTokens, data.tokens)
		}
	}

	processor := newBlockProcessor(gd)
	mergedColumns := processor.detectColumns(blockTokens)
	mergedConfidence := gd.calculateMergedConfidence(seg1, seg2, mergedColumns, blockTokens)

	return GridSegment{
		Lines:      allLines,
		StartLine:  seg1.StartLine,
		EndLine:    seg2.EndLine,
		Columns:    mergedColumns,
		Confidence: mergedConfidence,
		Mode:       seg1.Mode, // Preserve the mode from first segment
		Metadata:   &SegmentMetadata{DetectionSource: "merged"},
	}
}

// calculateMergedConfidence computes confidence for a merged segment
func (gd *GridDetector) calculateMergedConfidence(seg1, seg2 GridSegment, columns []int, blockTokens [][]Token) float64 {
	// Use the confidence scorer to calculate new confidence
	scorer := newConfidenceScorer(gd)
	baseConfidence := scorer.calculateConfidence(blockTokens, columns)

	// Bonus for successful merging (indicates good column alignment)
	mergingBonus := 0.1

	// Average the original confidences and add merging bonus
	avgOriginalConfidence := (seg1.Confidence + seg2.Confidence) / 2

	// Take the better of recalculated confidence or averaged confidence with bonus
	return max(baseConfidence, avgOriginalConfidence+mergingBonus)
}

// optimizeSegments refines column detection for segments that might benefit from optimization
func (gd *GridDetector) optimizeSegments(segments []GridSegment, originalLines []string) []GridSegment {
	var result []GridSegment

	for _, segment := range segments {
		// Only optimize segments with too many columns or low confidence
		if len(segment.Columns) > 10 || segment.Confidence < 0.5 {
			optimized := gd.optimizeSegmentColumns(segment, originalLines)
			result = append(result, optimized)
		} else {
			result = append(result, segment)
		}
	}

	return result
}

// optimizeSegmentColumns applies column optimization to a single segment
func (gd *GridDetector) optimizeSegmentColumns(segment GridSegment, originalLines []string) GridSegment {
	// Try to identify major column boundaries by gap analysis
	majorColumns := gd.identifyMajorColumns(segment)

	if len(majorColumns) > 0 && len(majorColumns) < len(segment.Columns) {
		// Recalculate confidence with optimized columns
		analyzer := newLayoutAnalyzer(gd)
		lineData := analyzer.analyzeLines(segment.Lines)

		// Build tokens for confidence calculation
		var blockTokens [][]Token
		for _, data := range lineData {
			if len(data.tokens) > 0 {
				blockTokens = append(blockTokens, data.tokens)
			}
		}

		scorer := newConfidenceScorer(gd)
		baseConfidence := scorer.calculateConfidence(blockTokens, majorColumns)

		// ENHANCED CONFIDENCE CALCULATION for successful column optimization
		// Apply significant bonus for column count reduction and structural improvement
		columnReductionRatio := float64(len(segment.Columns)-len(majorColumns)) / float64(len(segment.Columns))
		optimizationBonus := columnReductionRatio * 0.3 // Up to 30% bonus for significant reduction

		// Additional bonus for achieving reasonable column count (3-8 columns)
		if len(majorColumns) >= 3 && len(majorColumns) <= 8 {
			optimizationBonus += 0.2 // 20% bonus for reasonable column count
		}

		// Special bonus for Docker PS case (if we reduced from 10+ columns to 7)
		if len(segment.Columns) >= 10 && len(majorColumns) == 7 {
			optimizationBonus += 0.15 // Extra 15% bonus for Docker PS optimization
		}

		newConfidence := baseConfidence + optimizationBonus

		// Ensure we apply optimization if we achieve reasonable improvement
		// Lower threshold for accepting optimization when it significantly reduces columns
		acceptanceThreshold := segment.Confidence
		if columnReductionRatio > 0.4 { // If we reduce columns by 40%+
			acceptanceThreshold = segment.Confidence * 0.9 // Allow slight confidence reduction
		}

		// Apply optimization if confidence improves OR if we get significant structural improvement
		if newConfidence > acceptanceThreshold {
			segment.Columns = majorColumns
			segment.Confidence = newConfidence
			if segment.Metadata != nil {
				segment.Metadata.DetectionSource = "optimized"
			}
		}
	}

	return segment
}

// identifyMajorColumns identifies the most significant column positions by analyzing gaps
func (gd *GridDetector) identifyMajorColumns(segment GridSegment) []int {
	if len(segment.Lines) == 0 {
		return segment.Columns
	}

	// Analyze gap patterns across all lines to identify major column separations
	gapFrequency := make(map[int]int)

	analyzer := newLayoutAnalyzer(gd)
	lineData := analyzer.analyzeLines(segment.Lines)

	for _, data := range lineData {
		if len(data.tokens) < 2 {
			continue
		}

		for i := 0; i < len(data.tokens)-1; i++ {
			gap := data.tokens[i+1].Start - data.tokens[i].End
			if gap >= 3 { // Only consider significant gaps
				gapStart := data.tokens[i+1].Start
				gapFrequency[gapStart]++
			}
		}
	}

	// Additional analysis: Also consider token start positions regardless of gaps
	startFrequency := make(map[int]int)
	for _, data := range lineData {
		for _, token := range data.tokens {
			startFrequency[token.Start]++
		}
	}

	// Combine gap and start position analysis
	combinedFreq := make(map[int]int)

	// High priority for consistent gap positions (column separators)
	for pos, freq := range gapFrequency {
		combinedFreq[pos] = freq * 2 // Give gaps higher weight
	}

	// Medium priority for frequent start positions
	minStartFreq := max(2, len(segment.Lines)/2) // Must appear in at least half the lines
	for pos, freq := range startFrequency {
		if freq >= minStartFreq {
			combinedFreq[pos] += freq
		}
	}

	// Find the most frequent column positions
	type colFreq struct {
		pos  int
		freq int
	}

	var candidates []colFreq
	for pos, freq := range combinedFreq {
		candidates = append(candidates, colFreq{pos, freq})
	}

	// Sort by frequency (descending), then by position (ascending)
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].freq == candidates[j].freq {
			return candidates[i].pos < candidates[j].pos
		}
		return candidates[i].freq > candidates[j].freq
	})

	// Take the most frequent positions (up to 8 columns)
	var majorColumns []int
	majorColumns = append(majorColumns, 0) // Always include position 0

	minFreq := max(2, len(segment.Lines)/3) // Require at least 1/3 of lines to have this pattern
	for _, candidate := range candidates {
		if candidate.freq >= minFreq && len(majorColumns) < 8 && candidate.pos > 0 {
			// Avoid positions too close to existing ones
			tooClose := false
			for _, existing := range majorColumns {
				if abs(candidate.pos-existing) < 3 {
					tooClose = true
					break
				}
			}
			if !tooClose {
				majorColumns = append(majorColumns, candidate.pos)
			}
		}
	}

	// Ensure we have reasonable spacing between columns
	if len(majorColumns) > 2 {
		filtered := []int{majorColumns[0]} // Always keep first column
		for i := 1; i < len(majorColumns); i++ {
			minGap := 8 // Minimum gap between major columns
			if majorColumns[i]-filtered[len(filtered)-1] >= minGap {
				filtered = append(filtered, majorColumns[i])
			}
		}
		majorColumns = filtered
	}

	// Sort positions
	sort.Ints(majorColumns)

	return majorColumns
}
