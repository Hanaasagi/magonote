package textdetection

import (
	"strings"
	"unicode"
)

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
		alignmentThreshold:  0.7, // 70% of lines should align
		confidenceThreshold: 0.6, // 60% confidence minimum
		maxColumnVariance:   2,   // Allow 2 character variance in column positions
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

	var segments []GridSegment

	// Process text in chunks to identify potential grid segments
	for i := 0; i < len(lines); {
		segment := gd.analyzeSegment(lines, i)
		if segment != nil {
			segments = append(segments, *segment)
			i = segment.EndLine + 1
		} else {
			i++
		}
	}

	return segments
}

// analyzeSegment analyzes a potential grid segment starting from the given line
func (gd *GridDetector) analyzeSegment(lines []string, startIdx int) *GridSegment {
	// Skip empty lines at the start
	for startIdx < len(lines) && strings.TrimSpace(lines[startIdx]) == "" {
		startIdx++
	}

	if startIdx >= len(lines) {
		return nil
	}

	// Try to find consecutive lines that might form a grid
	potentialLines := []string{}
	lineIndices := []int{}

	for i := startIdx; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Skip empty lines
		if trimmed == "" {
			if len(potentialLines) > 0 {
				break // End of potential grid segment
			}
			continue
		}

		// Skip lines that are clearly not part of a grid (like commands starting with $)
		if strings.HasPrefix(trimmed, "$") {
			if len(potentialLines) > 0 {
				break // End of potential grid segment
			}
			continue
		}

		potentialLines = append(potentialLines, line)
		lineIndices = append(lineIndices, i)

		// Check if we have enough lines to analyze
		if len(potentialLines) >= gd.minLines {
			// Try to detect columns in this segment
			columns := gd.detectColumns(potentialLines)
			if len(columns) >= gd.minColumns {
				confidence := gd.calculateConfidence(potentialLines, columns)
				if confidence >= gd.confidenceThreshold {
					// Continue to see if we can extend this grid
					continue
				}
			}
		}
	}

	// Final analysis of the collected lines
	if len(potentialLines) >= gd.minLines {
		columns := gd.detectColumns(potentialLines)
		if len(columns) >= gd.minColumns {
			confidence := gd.calculateConfidence(potentialLines, columns)
			if confidence >= gd.confidenceThreshold {
				return &GridSegment{
					Lines:      potentialLines,
					StartLine:  lineIndices[0],
					EndLine:    lineIndices[len(lineIndices)-1],
					Columns:    columns,
					Confidence: confidence,
				}
			}
		}
	}

	return nil
}

// detectColumns identifies potential column boundaries in the given lines
func (gd *GridDetector) detectColumns(lines []string) []int {
	if len(lines) < 2 {
		return nil
	}

	// Find potential column positions by analyzing space patterns
	maxLen := 0
	for _, line := range lines {
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}

	if maxLen == 0 {
		return nil
	}

	// Count spaces at each position across all lines
	spaceCount := make([]int, maxLen)
	nonSpaceCount := make([]int, maxLen)

	for _, line := range lines {
		for i, char := range line {
			if unicode.IsSpace(char) {
				spaceCount[i]++
			} else {
				nonSpaceCount[i]++
			}
		}
	}

	// Identify positions where most lines have spaces (potential column boundaries)
	columns := []int{0} // Always start with position 0

	for i := 1; i < maxLen-1; i++ {
		// Look for positions where there's a transition from non-space to space
		// and where most lines have spaces
		spaceRatio := float64(spaceCount[i]) / float64(len(lines))
		prevNonSpaceRatio := float64(nonSpaceCount[i-1]) / float64(len(lines))

		if spaceRatio > gd.alignmentThreshold && prevNonSpaceRatio > 0.3 {
			// Look for the next non-space position as the actual column start
			for j := i + 1; j < maxLen; j++ {
				if float64(nonSpaceCount[j])/float64(len(lines)) > 0.3 {
					columns = append(columns, j)
					break
				}
			}
		}
	}

	// Remove columns that are too close to each other
	filteredColumns := []int{}
	for i, col := range columns {
		if i == 0 || col-filteredColumns[len(filteredColumns)-1] > 2 {
			filteredColumns = append(filteredColumns, col)
		}
	}

	return filteredColumns
}

// calculateConfidence calculates how confident we are that this is a grid
func (gd *GridDetector) calculateConfidence(lines []string, columns []int) float64 {
	if len(lines) < 2 || len(columns) < 2 {
		return 0.0
	}

	// Calculate alignment score for each column
	alignmentScores := make([]float64, len(columns))

	for colIdx, colPos := range columns {
		alignedLines := 0

		for _, line := range lines {
			if colPos < len(line) {
				// Check if there's actual content starting around this position
				found := false
				for i := max(0, colPos-gd.maxColumnVariance); i <= min(len(line)-1, colPos+gd.maxColumnVariance); i++ {
					if !unicode.IsSpace(rune(line[i])) && (i == 0 || unicode.IsSpace(rune(line[i-1]))) {
						found = true
						break
					}
				}
				if found {
					alignedLines++
				}
			}
		}

		alignmentScores[colIdx] = float64(alignedLines) / float64(len(lines))
	}

	// Calculate overall confidence as average alignment score
	totalScore := 0.0
	for _, score := range alignmentScores {
		totalScore += score
	}

	confidence := totalScore / float64(len(alignmentScores))

	// Bonus for having more columns (more structured data)
	columnBonus := min(0.2, float64(len(columns)-2)*0.05)
	confidence += columnBonus

	// Bonus for having more lines (more data consistency)
	lineBonus := min(0.1, float64(len(lines)-2)*0.02)
	confidence += lineBonus

	return min(1.0, confidence)
}
