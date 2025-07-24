package textdetection

import (
	"fmt"
	"strings"
)

// ============================================================================
// New API Data Types
// ============================================================================

// Cell represents a detected table cell with its content and position information
type Cell struct {
	Text      string `json:"text"`       // The text content of the cell
	Row       int    `json:"row"`        // Row index within the table (0-based)
	Column    int    `json:"column"`     // Column index within the table (0-based)
	LineIndex int    `json:"line_index"` // Original line index in the input
	StartPos  int    `json:"start_pos"`  // Start position of the cell in the line
	EndPos    int    `json:"end_pos"`    // End position of the cell in the line
}

// String returns a string representation of the cell
func (c Cell) String() string {
	return fmt.Sprintf("Cell[%d,%d]: %q at [%d-%d] in line %d",
		c.Row, c.Column, c.Text, c.StartPos, c.EndPos, c.LineIndex)
}

// IsEmpty returns true if the cell has no text content
func (c Cell) IsEmpty() bool {
	return strings.TrimSpace(c.Text) == ""
}

// Length returns the length of the cell text
func (c Cell) Length() int {
	return len(c.Text)
}

// Table represents a detected table with enhanced metadata and cell information
type Table struct {
	StartLine  int              `json:"start_line"`  // Starting line number in original text
	EndLine    int              `json:"end_line"`    // Ending line number in original text
	NumRows    int              `json:"num_rows"`    // Number of rows in the table
	NumColumns int              `json:"num_columns"` // Number of columns in the table
	Confidence float64          `json:"confidence"`  // Detection confidence score (0.0-1.0)
	Mode       TokenizationMode `json:"mode"`        // Tokenization mode used for detection
	Cells      [][]Cell         `json:"cells"`       // 2D array of cells [row][column]
	Metadata   *TableMetadata   `json:"metadata"`    // Additional metadata about the table
}

// IsValid returns true if the table has valid structure
func (t Table) IsValid() bool {
	return t.NumRows > 0 && t.NumColumns > 0 && len(t.Cells) == t.NumRows
}

// GetColumnPositions returns the column start positions from metadata, if available
func (t Table) GetColumnPositions() []int {
	if t.Metadata != nil {
		return t.Metadata.ColumnPositions
	}
	return nil
}

// GetCell safely returns a cell at the given row and column indices
func (t Table) GetCell(row, col int) (*Cell, error) {
	if row < 0 || row >= t.NumRows {
		return nil, fmt.Errorf("row index %d out of range [0-%d]", row, t.NumRows-1)
	}
	if col < 0 || col >= len(t.Cells[row]) {
		return nil, fmt.Errorf("column index %d out of range [0-%d] for row %d", col, len(t.Cells[row])-1, row)
	}
	return &t.Cells[row][col], nil
}

// GetRow returns all cells in the specified row
func (t Table) GetRow(row int) ([]Cell, error) {
	if row < 0 || row >= t.NumRows {
		return nil, fmt.Errorf("row index %d out of range [0-%d]", row, t.NumRows-1)
	}
	return t.Cells[row], nil
}

// GetColumn returns all cells in the specified column
func (t Table) GetColumn(col int) ([]Cell, error) {
	if col < 0 || col >= t.NumColumns {
		return nil, fmt.Errorf("column index %d out of range [0-%d]", col, t.NumColumns-1)
	}

	var columnCells []Cell
	for row := 0; row < t.NumRows; row++ {
		if col < len(t.Cells[row]) {
			columnCells = append(columnCells, t.Cells[row][col])
		}
	}
	return columnCells, nil
}

// GetHeaderRow returns the first row as header cells, if the table has rows
func (t Table) GetHeaderRow() ([]Cell, error) {
	if t.NumRows == 0 {
		return nil, fmt.Errorf("table has no rows")
	}
	return t.GetRow(0)
}

// GetRowTexts returns the text content of all cells in a row as a slice of strings
func (t Table) GetRowTexts(row int) ([]string, error) {
	cells, err := t.GetRow(row)
	if err != nil {
		return nil, err
	}

	texts := make([]string, len(cells))
	for i, cell := range cells {
		texts[i] = cell.Text
	}
	return texts, nil
}

// GetColumnTexts returns the text content of all cells in a column as a slice of strings
func (t Table) GetColumnTexts(col int) ([]string, error) {
	cells, err := t.GetColumn(col)
	if err != nil {
		return nil, err
	}

	texts := make([]string, len(cells))
	for i, cell := range cells {
		texts[i] = cell.Text
	}
	return texts, nil
}

// LineCount returns the number of lines this table spans
func (t Table) LineCount() int {
	return t.EndLine - t.StartLine + 1
}

// String returns a string representation of the table
func (t Table) String() string {
	mode := map[TokenizationMode]string{
		SingleSpaceMode: "SingleSpace",
		MultiSpaceMode:  "MultiSpace",
	}[t.Mode]

	return fmt.Sprintf("Table[%d-%d]: %d×%d, mode=%s, confidence=%.3f",
		t.StartLine, t.EndLine, t.NumRows, t.NumColumns, mode, t.Confidence)
}

// TableMetadata contains detailed information about how a table was detected
type TableMetadata struct {
	DetectionStrategy string            `json:"detection_strategy"` // Strategy used ("dual_round", "single_round", etc.)
	TokenizationMode  TokenizationMode  `json:"tokenization_mode"`  // Mode used for tokenization
	ColumnPositions   []int             `json:"column_positions"`   // Character positions where columns start
	AlignmentData     []ColumnAlignment `json:"alignment_data"`     // Alignment information for each column
	QualityMetrics    *QualityMetrics   `json:"quality_metrics"`    // Quality assessment metrics
}

// QualityMetrics provides detailed quality assessment of the detected table
type QualityMetrics struct {
	AlignmentScore   float64 `json:"alignment_score"`    // How well columns are aligned (0.0-1.0)
	ConsistencyScore float64 `json:"consistency_score"`  // How consistent row structures are (0.0-1.0)
	CompactnessScore float64 `json:"compactness_score"`  // How compact the table structure is (0.0-1.0)
	TokenCountStdDev float64 `json:"token_count_stddev"` // Standard deviation of token counts per row
	AvgColumnSpacing float64 `json:"avg_column_spacing"` // Average spacing between columns
}

// ============================================================================
// Strategy Interfaces
// ============================================================================

// DetectionStrategy defines the interface for different grid detection strategies
type DetectionStrategy interface {
	// DetectTables analyzes text lines and returns detected tables
	DetectTables(lines []string) ([]Table, error)

	// GetName returns the name of this detection strategy
	GetName() string

	// GetConfiguration returns the current configuration
	GetConfiguration() DetectionConfig
}

// NewTokenizationStrategy defines the interface for different tokenization approaches
// (Named differently to avoid conflict with existing interface)
type NewTokenizationStrategy interface {
	// Tokenize splits a line into tokens with position information
	Tokenize(line string, lineIndex int, context []string) ([]Token, error)

	// GetMode returns the tokenization mode this strategy implements
	GetMode() TokenizationMode

	// ShouldApply determines if this strategy should be applied to the given context
	ShouldApply(line string, lineIndex int, context []string) bool
}

// LayoutAnalyzer defines the interface for analyzing line layouts
type LayoutAnalyzer interface {
	// AnalyzeLayout determines the layout structure of a set of lines
	AnalyzeLayout(lines []string) ([]LineLayout, error)

	// CompareSimilarity checks if two layouts are similar enough to be part of the same table
	CompareSimilarity(layout1, layout2 LineLayout) bool
}

// ConfidenceScorer defines the interface for calculating detection confidence
type ConfidenceScorer interface {
	// CalculateConfidence computes a confidence score for a detected table
	CalculateConfidence(table Table, originalLines []string) (float64, error)

	// CalculateQualityMetrics computes detailed quality metrics
	CalculateQualityMetrics(table Table, originalLines []string) (*QualityMetrics, error)
}

// ============================================================================
// Configuration Types
// ============================================================================

// DetectionConfig holds configuration parameters for grid detection
type DetectionConfig struct {
	MinLines            int              `json:"min_lines"`            // Minimum lines required to form a grid
	MinColumns          int              `json:"min_columns"`          // Minimum columns required to form a grid
	AlignmentThreshold  float64          `json:"alignment_threshold"`  // Threshold for column alignment consistency
	ConfidenceThreshold float64          `json:"confidence_threshold"` // Minimum confidence to consider as grid
	MaxColumnVariance   int              `json:"max_column_variance"`  // Maximum allowed variance in column positions
	TokenizationMode    TokenizationMode `json:"tokenization_mode"`    // Tokenization strategy to use
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() DetectionConfig {
	return DetectionConfig{
		MinLines:            DefaultMinLines,
		MinColumns:          DefaultMinColumns,
		AlignmentThreshold:  DefaultAlignmentThreshold,
		ConfidenceThreshold: DefaultConfidenceThreshold,
		MaxColumnVariance:   DefaultMaxColumnVariance,
		TokenizationMode:    SingleSpaceMode,
	}
}

// ============================================================================
// Internal Data Types
// ============================================================================

// LineLayout represents the layout structure of a single line
type LineLayout struct {
	Tokens          []Token `json:"tokens"`           // Tokens found in this line
	ColumnPositions []int   `json:"column_positions"` // Column start positions
	LineIndex       int     `json:"line_index"`       // Index of this line in the original text
}

// CandidateTable represents a potential table during detection
type CandidateTable struct {
	StartLine  int          `json:"start_line"` // Starting line index
	EndLine    int          `json:"end_line"`   // Ending line index
	Lines      []string     `json:"lines"`      // Text lines that form this table
	Layouts    []LineLayout `json:"layouts"`    // Layout information for each line
	Confidence float64      `json:"confidence"` // Initial confidence score
}

// ============================================================================
// Utility Functions
// ============================================================================

// ConvertGridSegmentToTable converts a legacy GridSegment to the new Table format
func ConvertGridSegmentToTable(segment GridSegment) Table {
	table := Table{
		StartLine:  segment.StartLine,
		EndLine:    segment.EndLine,
		Confidence: segment.Confidence,
		Mode:       segment.Mode,
		NumRows:    len(segment.Lines),
		NumColumns: len(segment.Columns),
	}

	// Convert metadata
	if segment.Metadata != nil {
		table.Metadata = &TableMetadata{
			DetectionStrategy: segment.Metadata.DetectionSource,
			TokenizationMode:  segment.Metadata.TokenizationMode,
			ColumnPositions:   segment.Columns,
			AlignmentData:     segment.Metadata.AlignmentData,
		}
	}

	// Convert to cell structure
	table.Cells = make([][]Cell, table.NumRows)

	// Extract cells using token information if available
	if segment.Metadata != nil && len(segment.Metadata.OriginalTokens) == len(segment.Lines) {
		for rowIdx, tokens := range segment.Metadata.OriginalTokens {
			table.Cells[rowIdx] = make([]Cell, len(tokens))
			for colIdx, token := range tokens {
				table.Cells[rowIdx][colIdx] = Cell{
					Text:      token.Text,
					Row:       rowIdx,
					Column:    colIdx,
					LineIndex: segment.StartLine + rowIdx,
					StartPos:  token.Start,
					EndPos:    token.End,
				}
			}
		}
	}

	return table
}

// ConvertTableToGridSegment converts a new Table back to legacy GridSegment format
func ConvertTableToGridSegment(table Table) GridSegment {
	segment := GridSegment{
		Lines:      make([]string, table.NumRows),
		StartLine:  table.StartLine,
		EndLine:    table.EndLine,
		Confidence: table.Confidence,
		Mode:       table.Mode,
	}

	// Convert metadata
	if table.Metadata != nil {
		segment.Metadata = &SegmentMetadata{
			TokenizationMode: table.Metadata.TokenizationMode,
			DetectionSource:  table.Metadata.DetectionStrategy,
			AlignmentData:    table.Metadata.AlignmentData,
		}
		segment.Columns = table.Metadata.ColumnPositions

		// Convert cells back to tokens
		segment.Metadata.OriginalTokens = make([][]Token, table.NumRows)
		for rowIdx, row := range table.Cells {
			segment.Metadata.OriginalTokens[rowIdx] = make([]Token, len(row))
			for colIdx, cell := range row {
				segment.Metadata.OriginalTokens[rowIdx][colIdx] = Token{
					Text:  cell.Text,
					Start: cell.StartPos,
					End:   cell.EndPos,
				}
			}
		}
	}

	return segment
}
