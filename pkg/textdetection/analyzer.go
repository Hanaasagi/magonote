package textdetection

import (
	"errors"
	"math"
	"strings"
)

// ============================================================================
// Enhanced Analysis Utilities
// ============================================================================

// AnalysisResult contains comprehensive analysis information for a table candidate
type AnalysisResult struct {
	Confidence        float64           `json:"confidence"`
	Columns           []int             `json:"columns"`
	QualityMetrics    *QualityMetrics   `json:"quality_metrics"`
	AlignmentData     []ColumnAlignment `json:"alignment_data"`
	TokenDistribution map[int]int       `json:"token_distribution"`
}

// TableAnalyzer provides enhanced analysis capabilities for table detection
type TableAnalyzer struct {
	config DetectionConfig
}

// NewTableAnalyzer creates a new enhanced table analyzer
func NewTableAnalyzer(config DetectionConfig) *TableAnalyzer {
	return &TableAnalyzer{config: config}
}

// AnalyzeCandidate performs comprehensive analysis on a candidate table
func (ta *TableAnalyzer) AnalyzeCandidate(lines []string, startLine, endLine int) (*AnalysisResult, error) {
	if endLine >= len(lines) || startLine > endLine {
		return nil, errors.New("invalid line range")
	}

	candidateLines := lines[startLine : endLine+1]

	// Use existing grid detector for analysis
	detector := NewGridDetector(
		WithMinLines(ta.config.MinLines),
		WithMinColumns(ta.config.MinColumns),
		WithAlignmentThreshold(ta.config.AlignmentThreshold),
		WithConfidenceThreshold(ta.config.ConfidenceThreshold),
		WithMaxColumnVariance(ta.config.MaxColumnVariance),
		WithTokenizationMode(ta.config.TokenizationMode),
	)

	segments := detector.DetectGrids(candidateLines)
	if len(segments) == 0 {
		return &AnalysisResult{Confidence: 0.0}, nil
	}

	// Get the best segment
	bestSegment := segments[0]
	for _, segment := range segments {
		if segment.Confidence > bestSegment.Confidence {
			bestSegment = segment
		}
	}

	// Calculate quality metrics
	qualityMetrics := ta.calculateQualityMetrics(bestSegment, candidateLines)

	return &AnalysisResult{
		Confidence:        bestSegment.Confidence,
		Columns:           bestSegment.Columns,
		QualityMetrics:    qualityMetrics,
		AlignmentData:     bestSegment.Metadata.AlignmentData,
		TokenDistribution: ta.analyzeTokenDistribution(bestSegment),
	}, nil
}

// calculateQualityMetrics computes detailed quality assessment
func (ta *TableAnalyzer) calculateQualityMetrics(segment GridSegment, originalLines []string) *QualityMetrics {
	if segment.Metadata == nil || len(segment.Metadata.OriginalTokens) == 0 {
		return &QualityMetrics{
			AlignmentScore:   segment.Confidence,
			ConsistencyScore: segment.Confidence,
			CompactnessScore: segment.Confidence,
		}
	}

	// Calculate alignment score
	alignmentScore := ta.calculateAlignmentScore(segment.Metadata.OriginalTokens, segment.Columns)

	// Calculate consistency score
	consistencyScore := ta.calculateConsistencyScore(segment.Metadata.OriginalTokens)

	// Calculate compactness score
	compactnessScore := ta.calculateCompactnessScore(segment)

	// Calculate token count standard deviation
	tokenCountStdDev := ta.calculateTokenCountStdDev(segment.Metadata.OriginalTokens)

	// Calculate average column spacing
	avgColumnSpacing := ta.calculateAvgColumnSpacing(segment.Columns)

	return &QualityMetrics{
		AlignmentScore:   alignmentScore,
		ConsistencyScore: consistencyScore,
		CompactnessScore: compactnessScore,
		TokenCountStdDev: tokenCountStdDev,
		AvgColumnSpacing: avgColumnSpacing,
	}
}

// calculateAlignmentScore measures how well columns are aligned
func (ta *TableAnalyzer) calculateAlignmentScore(tokens [][]Token, columns []int) float64 {
	if len(tokens) == 0 || len(columns) == 0 {
		return 0.0
	}

	totalAlignment := 0.0
	totalComparisons := 0

	for colIdx, expectedPos := range columns {
		alignmentSum := 0.0
		validRows := 0

		for _, rowTokens := range tokens {
			if colIdx < len(rowTokens) {
				actualPos := rowTokens[colIdx].Start
				deviation := abs(actualPos - expectedPos)

				// Convert deviation to alignment score (closer = higher score)
				maxDeviation := ta.config.MaxColumnVariance
				if deviation <= maxDeviation {
					alignmentSum += 1.0 - (float64(deviation) / float64(maxDeviation))
					validRows++
				}
			}
		}

		if validRows > 0 {
			totalAlignment += alignmentSum / float64(validRows)
			totalComparisons++
		}
	}

	if totalComparisons == 0 {
		return 0.0
	}

	return totalAlignment / float64(totalComparisons)
}

// calculateConsistencyScore measures row structure consistency
func (ta *TableAnalyzer) calculateConsistencyScore(tokens [][]Token) float64 {
	if len(tokens) <= 1 {
		return 1.0
	}

	// Calculate token count consistency
	tokenCounts := make(map[int]int)
	for _, rowTokens := range tokens {
		tokenCounts[len(rowTokens)]++
	}

	// Find most common token count
	maxCount := 0
	for _, frequency := range tokenCounts {
		if frequency > maxCount {
			maxCount = frequency
		}
	}

	// Consistency is the ratio of rows with the most common token count
	consistency := float64(maxCount) / float64(len(tokens))

	// Apply penalty for too much variation
	if len(tokenCounts) > 2 {
		consistency *= 0.9 // Slight penalty for variation
	}

	return consistency
}

// calculateCompactnessScore measures table structure compactness
func (ta *TableAnalyzer) calculateCompactnessScore(segment GridSegment) float64 {
	if len(segment.Columns) <= 1 {
		return 1.0
	}

	// Calculate average gap between columns
	totalGap := 0
	gapCount := 0

	for i := 1; i < len(segment.Columns); i++ {
		gap := segment.Columns[i] - segment.Columns[i-1]
		totalGap += gap
		gapCount++
	}

	if gapCount == 0 {
		return 1.0
	}

	avgGap := float64(totalGap) / float64(gapCount)

	// Ideal gap is between 3-8 characters
	idealMinGap := 3.0
	idealMaxGap := 8.0

	var score float64
	if avgGap >= idealMinGap && avgGap <= idealMaxGap {
		score = 1.0
	} else if avgGap < idealMinGap {
		score = avgGap / idealMinGap
	} else {
		score = idealMaxGap / avgGap
	}

	return min(0.0, min(1.0, score))
}

// calculateTokenCountStdDev calculates standard deviation of token counts per row
func (ta *TableAnalyzer) calculateTokenCountStdDev(tokens [][]Token) float64 {
	if len(tokens) <= 1 {
		return 0.0
	}

	// Calculate mean
	sum := 0
	for _, rowTokens := range tokens {
		sum += len(rowTokens)
	}
	mean := float64(sum) / float64(len(tokens))

	// Calculate variance
	variance := 0.0
	for _, rowTokens := range tokens {
		diff := float64(len(rowTokens)) - mean
		variance += diff * diff
	}
	variance /= float64(len(tokens))

	return math.Sqrt(variance)
}

// calculateAvgColumnSpacing calculates average spacing between columns
func (ta *TableAnalyzer) calculateAvgColumnSpacing(columns []int) float64 {
	if len(columns) <= 1 {
		return 0.0
	}

	totalSpacing := 0
	for i := 1; i < len(columns); i++ {
		totalSpacing += columns[i] - columns[i-1]
	}

	return float64(totalSpacing) / float64(len(columns)-1)
}

// analyzeTokenDistribution analyzes the distribution of token counts across rows
func (ta *TableAnalyzer) analyzeTokenDistribution(segment GridSegment) map[int]int {
	distribution := make(map[int]int)

	if segment.Metadata != nil && len(segment.Metadata.OriginalTokens) > 0 {
		for _, rowTokens := range segment.Metadata.OriginalTokens {
			count := len(rowTokens)
			distribution[count]++
		}
	}

	return distribution
}

// ============================================================================
// Enhanced Word Extraction
// ============================================================================

// WordExtractor provides enhanced word extraction with quality filtering
type WordExtractor struct {
	minWordLength int
	skipPatterns  []string
}

// NewWordExtractor creates a new word extractor with configuration
func NewWordExtractor() *WordExtractor {
	return &WordExtractor{
		minWordLength: MinWordLength,
		skipPatterns:  []string{"$", "#", "//", "/*", "*/"}, // Common non-content patterns
	}
}

// ExtractCells extracts cells from a GridSegment and returns them in the new Cell format
func (we *WordExtractor) ExtractCells(segment GridSegment) [][]Cell {
	if segment.Metadata == nil || len(segment.Metadata.OriginalTokens) == 0 {
		return we.extractCellsFromLines(segment)
	}

	return we.extractCellsFromTokens(segment)
}

// extractCellsFromTokens extracts cells when token metadata is available
func (we *WordExtractor) extractCellsFromTokens(segment GridSegment) [][]Cell {
	cells := make([][]Cell, len(segment.Metadata.OriginalTokens))

	for rowIdx, rowTokens := range segment.Metadata.OriginalTokens {
		cells[rowIdx] = make([]Cell, len(rowTokens))

		for colIdx, token := range rowTokens {
			cells[rowIdx][colIdx] = Cell{
				Text:      token.Text,
				Row:       rowIdx,
				Column:    colIdx,
				LineIndex: segment.StartLine + rowIdx,
				StartPos:  token.Start,
				EndPos:    token.End,
			}
		}
	}

	return cells
}

// extractCellsFromLines extracts cells when only lines are available (fallback)
func (we *WordExtractor) extractCellsFromLines(segment GridSegment) [][]Cell {
	// This is a fallback method for when detailed token information is not available
	// It uses simple word extraction as in the original ExtractValidWords function

	var allCells [][]Cell

	for lineIdx, line := range segment.Lines {
		// Simple word extraction using regex (similar to original)
		words := strings.Fields(line) // Basic split on whitespace

		var rowCells []Cell
		currentPos := 0

		for colIdx, word := range words {
			if len(word) >= we.minWordLength && !we.shouldSkipWord(word) {
				// Find actual position in line
				wordPos := strings.Index(line[currentPos:], word)
				if wordPos >= 0 {
					actualPos := currentPos + wordPos

					cell := Cell{
						Text:      word,
						Row:       lineIdx,
						Column:    colIdx,
						LineIndex: segment.StartLine + lineIdx,
						StartPos:  actualPos,
						EndPos:    actualPos + len(word) - 1,
					}

					rowCells = append(rowCells, cell)
					currentPos = actualPos + len(word)
				}
			}
		}

		if len(rowCells) > 0 {
			allCells = append(allCells, rowCells)
		}
	}

	return allCells
}

// shouldSkipWord determines if a word should be skipped during extraction
func (we *WordExtractor) shouldSkipWord(word string) bool {
	// Skip very short words
	if len(word) < we.minWordLength {
		return true
	}

	// Skip words that match skip patterns
	for _, pattern := range we.skipPatterns {
		if strings.Contains(word, pattern) {
			return true
		}
	}

	return false
}
