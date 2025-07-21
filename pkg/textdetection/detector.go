package textdetection

import (
	"fmt"
)

// ============================================================================
// Main Detector API Implementation
// ============================================================================

// Detector provides the main interface for table detection with improved API
type Detector struct {
	config     DetectionConfig
	strategies []DetectionStrategy
	analyzer   *TableAnalyzer
	extractor  *WordExtractor
}

// NewDetector creates a new detector with the specified configuration
func NewDetector(opts ...DetectorOption) *Detector {
	config := DefaultConfig()

	// Apply options
	for _, opt := range opts {
		opt(&config)
	}

	detector := &Detector{
		config:    config,
		analyzer:  NewTableAnalyzer(config),
		extractor: NewWordExtractor(),
	}

	// Initialize strategies
	detector.initializeStrategies()

	return detector
}

// DetectorOption defines options for configuring the detector
type DetectorOption func(*DetectionConfig)

// WithMinLinesOption sets the minimum lines required for detection
func WithMinLinesOption(minLines int) DetectorOption {
	return func(config *DetectionConfig) {
		config.MinLines = minLines
	}
}

// WithMinColumnsOption sets the minimum columns required for detection
func WithMinColumnsOption(minColumns int) DetectorOption {
	return func(config *DetectionConfig) {
		config.MinColumns = minColumns
	}
}

// WithAlignmentThresholdOption sets the alignment threshold
func WithAlignmentThresholdOption(threshold float64) DetectorOption {
	return func(config *DetectionConfig) {
		config.AlignmentThreshold = threshold
	}
}

// WithConfidenceThresholdOption sets the confidence threshold
func WithConfidenceThresholdOption(threshold float64) DetectorOption {
	return func(config *DetectionConfig) {
		config.ConfidenceThreshold = threshold
	}
}

// WithMaxColumnVarianceOption sets the maximum column variance
func WithMaxColumnVarianceOption(variance int) DetectorOption {
	return func(config *DetectionConfig) {
		config.MaxColumnVariance = variance
	}
}

// WithTokenizationModeOption sets the tokenization mode
func WithTokenizationModeOption(mode TokenizationMode) DetectorOption {
	return func(config *DetectionConfig) {
		config.TokenizationMode = mode
	}
}

// initializeStrategies sets up detection strategies
func (d *Detector) initializeStrategies() {
	// Add dual-round strategy as the primary strategy
	d.strategies = append(d.strategies, NewDualRoundStrategy(d.config))

	// Add single-round strategies as fallbacks
	d.strategies = append(d.strategies, NewSingleRoundStrategy(d.config, SingleSpaceMode))
	d.strategies = append(d.strategies, NewSingleRoundStrategy(d.config, MultiSpaceMode))
}

// DetectTables implements the main detection interface
func (d *Detector) DetectTables(lines []string) ([]Table, error) {
	if len(lines) < d.config.MinLines {
		return nil, nil
	}

	var allTables []Table
	var bestStrategy DetectionStrategy
	var bestResults []Table
	highestConfidence := 0.0

	// Try each strategy and keep the best results
	for _, strategy := range d.strategies {
		tables, err := strategy.DetectTables(lines)
		if err != nil {
			continue
		}

		// Calculate total confidence for this strategy
		totalConfidence := 0.0
		for _, table := range tables {
			totalConfidence += table.Confidence
		}

		if totalConfidence > highestConfidence || len(bestResults) == 0 {
			highestConfidence = totalConfidence
			bestStrategy = strategy
			bestResults = tables
		}
	}

	if len(bestResults) > 0 {
		// Enhance results with quality metrics
		for i := range bestResults {
			d.enhanceTableWithMetadata(&bestResults[i], lines, bestStrategy)
		}
		allTables = append(allTables, bestResults...)
	}

	return allTables, nil
}

// enhanceTableWithMetadata adds comprehensive metadata to detected tables
func (d *Detector) enhanceTableWithMetadata(table *Table, lines []string, strategy DetectionStrategy) {
	if table.Metadata == nil {
		table.Metadata = &TableMetadata{
			DetectionStrategy: strategy.GetName(),
			TokenizationMode:  table.Mode,
		}
	}

	// Calculate quality metrics if not already present
	if table.Metadata.QualityMetrics == nil {
		// Convert table back to GridSegment for analysis
		segment := ConvertTableToGridSegment(*table)
		if segment.Metadata != nil {
			analyzer := NewTableAnalyzer(d.config)
			table.Metadata.QualityMetrics = analyzer.calculateQualityMetrics(segment, lines)
		}
	}
}

// ============================================================================
// Detection Strategy Implementations
// ============================================================================

// DualRoundStrategy implements the dual-round detection approach
type DualRoundStrategy struct {
	config            DetectionConfig
	firstRoundConfig  DetectionConfig
	secondRoundConfig DetectionConfig
}

// NewDualRoundStrategy creates a new dual-round detection strategy
func NewDualRoundStrategy(baseConfig DetectionConfig) *DualRoundStrategy {
	firstRoundConfig := baseConfig
	firstRoundConfig.TokenizationMode = MultiSpaceMode
	firstRoundConfig.ConfidenceThreshold = FirstRoundConfidenceThreshold
	firstRoundConfig.MaxColumnVariance = FirstRoundMaxColumnVariance

	secondRoundConfig := baseConfig
	secondRoundConfig.TokenizationMode = SingleSpaceMode
	secondRoundConfig.ConfidenceThreshold = SecondRoundConfidenceThreshold
	secondRoundConfig.MaxColumnVariance = SecondRoundMaxColumnVariance

	return &DualRoundStrategy{
		config:            baseConfig,
		firstRoundConfig:  firstRoundConfig,
		secondRoundConfig: secondRoundConfig,
	}
}

// DetectTables implements DetectionStrategy interface
func (drs *DualRoundStrategy) DetectTables(lines []string) ([]Table, error) {
	// Use existing DualRoundDetector for the actual detection
	detector := NewDualRoundDetector(
		WithMinLines(drs.config.MinLines),
		WithMinColumns(drs.config.MinColumns),
		WithAlignmentThreshold(drs.config.AlignmentThreshold),
	)

	segments := detector.DetectGrids(lines)
	if len(segments) == 0 {
		return nil, nil
	}

	// Convert GridSegments to Tables
	var tables []Table
	extractor := NewWordExtractor()

	for _, segment := range segments {
		table := ConvertGridSegmentToTable(segment)

		// Extract cells
		cells := extractor.ExtractCells(segment)
		table.Cells = cells
		table.NumRows = len(cells)
		if len(cells) > 0 {
			table.NumColumns = len(cells[0])
		}

		tables = append(tables, table)
	}

	return tables, nil
}

// GetName returns the strategy name
func (drs *DualRoundStrategy) GetName() string {
	return "dual_round"
}

// GetConfiguration returns the strategy configuration
func (drs *DualRoundStrategy) GetConfiguration() DetectionConfig {
	return drs.config
}

// ============================================================================
// Single Round Strategy Implementation
// ============================================================================

// SingleRoundStrategy implements single-round detection with specified tokenization mode
type SingleRoundStrategy struct {
	config DetectionConfig
	mode   TokenizationMode
}

// NewSingleRoundStrategy creates a new single-round detection strategy
func NewSingleRoundStrategy(config DetectionConfig, mode TokenizationMode) *SingleRoundStrategy {
	strategyConfig := config
	strategyConfig.TokenizationMode = mode

	return &SingleRoundStrategy{
		config: strategyConfig,
		mode:   mode,
	}
}

// DetectTables implements DetectionStrategy interface
func (srs *SingleRoundStrategy) DetectTables(lines []string) ([]Table, error) {
	// Use existing GridDetector for the actual detection
	detector := NewGridDetector(
		WithMinLines(srs.config.MinLines),
		WithMinColumns(srs.config.MinColumns),
		WithAlignmentThreshold(srs.config.AlignmentThreshold),
		WithConfidenceThreshold(srs.config.ConfidenceThreshold),
		WithMaxColumnVariance(srs.config.MaxColumnVariance),
		WithTokenizationMode(srs.mode),
	)

	segments := detector.DetectGrids(lines)
	if len(segments) == 0 {
		return nil, nil
	}

	// Convert GridSegments to Tables
	var tables []Table
	extractor := NewWordExtractor()

	for _, segment := range segments {
		table := ConvertGridSegmentToTable(segment)

		// Extract cells
		cells := extractor.ExtractCells(segment)
		table.Cells = cells
		table.NumRows = len(cells)
		if len(cells) > 0 {
			table.NumColumns = len(cells[0])
		}

		tables = append(tables, table)
	}

	return tables, nil
}

// GetName returns the strategy name
func (srs *SingleRoundStrategy) GetName() string {
	modeStr := "SingleSpace"
	if srs.mode == MultiSpaceMode {
		modeStr = "MultiSpace"
	}
	return fmt.Sprintf("single_round_%s", modeStr)
}

// GetConfiguration returns the strategy configuration
func (srs *SingleRoundStrategy) GetConfiguration() DetectionConfig {
	return srs.config
}

// ============================================================================
// Backward Compatibility Types
// ============================================================================

// GridWord represents a word extracted from a grid segment (for backward compatibility)
type GridWord struct {
	Text    string `json:"text"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
	LineIdx int    `json:"line_idx"`
}

// ============================================================================
// Backward Compatibility Functions
// ============================================================================

// ExtractValidWords maintains backward compatibility with the original API
func ExtractValidWords(segment GridSegment) []GridWord {
	extractor := NewWordExtractor()
	cells := extractor.ExtractCells(segment)

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

// ============================================================================
// Enhanced API Functions
// ============================================================================

// DetectTablesWithCells provides enhanced API that returns cell-level information
func DetectTablesWithCells(lines []string, opts ...DetectorOption) ([]Table, error) {
	detector := NewDetector(opts...)
	return detector.DetectTables(lines)
}

// DetectGridsLegacy provides backward compatibility with the original DetectGrids function
func DetectGridsLegacy(lines []string, opts ...GridOption) []GridSegment {
	// Convert new options to old GridDetector options
	detector := NewGridDetector(opts...)
	return detector.DetectGrids(lines)
}

// ExtractTableCells extracts structured cell data from detected tables
func ExtractTableCells(tables []Table) [][][]Cell {
	var allCells [][][]Cell
	for _, table := range tables {
		allCells = append(allCells, table.Cells)
	}
	return allCells
}

// GetTableQualityMetrics returns quality metrics for all detected tables
func GetTableQualityMetrics(tables []Table) []*QualityMetrics {
	var metrics []*QualityMetrics
	for _, table := range tables {
		if table.Metadata != nil {
			metrics = append(metrics, table.Metadata.QualityMetrics)
		} else {
			metrics = append(metrics, nil)
		}
	}
	return metrics
}

// ============================================================================
// Utility Functions
// ============================================================================

// ValidateDetectionConfig validates a detection configuration
func ValidateDetectionConfig(config DetectionConfig) error {
	if config.MinLines < 1 {
		return fmt.Errorf("MinLines must be at least 1, got %d", config.MinLines)
	}
	if config.MinColumns < 1 {
		return fmt.Errorf("MinColumns must be at least 1, got %d", config.MinColumns)
	}
	if config.AlignmentThreshold < 0.0 || config.AlignmentThreshold > 1.0 {
		return fmt.Errorf("AlignmentThreshold must be between 0.0 and 1.0, got %f", config.AlignmentThreshold)
	}
	if config.ConfidenceThreshold < 0.0 || config.ConfidenceThreshold > 1.0 {
		return fmt.Errorf("ConfidenceThreshold must be between 0.0 and 1.0, got %f", config.ConfidenceThreshold)
	}
	if config.MaxColumnVariance < 0 {
		return fmt.Errorf("MaxColumnVariance must be non-negative, got %d", config.MaxColumnVariance)
	}
	return nil
}
