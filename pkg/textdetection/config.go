package textdetection

// Algorithm Configuration Constants
// These constants define the behavior of the grid detection algorithm

// Detection Thresholds
const (
	// DefaultMinLines is the minimum number of lines required to form a grid
	DefaultMinLines = 2

	// DefaultMinColumns is the minimum number of columns required to form a grid
	DefaultMinColumns = 2

	// DefaultAlignmentThreshold is the threshold for column alignment consistency (0.0-1.0)
	DefaultAlignmentThreshold = 0.7

	// DefaultConfidenceThreshold is the minimum confidence to consider as grid (0.0-1.0)
	DefaultConfidenceThreshold = 0.6

	// DefaultMaxColumnVariance is the maximum allowed variance in column positions
	DefaultMaxColumnVariance = 2
)

// Dual-Round Detection Configuration
const (
	// FirstRoundConfidenceThreshold is the confidence threshold for multi-space tokenization
	FirstRoundConfidenceThreshold = 0.4

	// FirstRoundMaxColumnVariance is the variance tolerance for multi-space tokenization
	FirstRoundMaxColumnVariance = 3

	// SecondRoundConfidenceThreshold is the confidence threshold for single-space tokenization
	SecondRoundConfidenceThreshold = 0.6

	// SecondRoundMaxColumnVariance is the variance tolerance for single-space tokenization
	SecondRoundMaxColumnVariance = 2
)

// Tokenization Configuration
const (
	// MinSpacesForMultiSpaceMode defines how many consecutive spaces trigger separation in MultiSpaceMode
	MinSpacesForMultiSpaceMode = 2

	// MinSpacesForSingleSpaceMode defines how many consecutive spaces trigger separation in SingleSpaceMode
	MinSpacesForSingleSpaceMode = 1

	// MinTokenWidth is the minimum width required for column detection
	MinTokenWidth = 2

	// CompoundTokenMinWidth is the minimum width for compound tokens in MultiSpaceMode
	CompoundTokenMinWidth = 3
)

// Confidence Scoring Weights
const (
	// ColumnBonusWeight is the weight for having a reasonable number of columns
	ColumnBonusWeight = 0.2

	// OptimalColumnBonusWeight is the additional weight for having 3-7 columns
	OptimalColumnBonusWeight = 0.3

	// LineBonusWeight is the weight per additional line in the segment
	LineBonusWeight = 0.02

	// MaxLineBonus is the maximum bonus for line count
	MaxLineBonus = 0.1

	// MaxColumnBonus is the maximum bonus for column count
	MaxColumnBonus = 0.2

	// MultiSpaceBonus is the bonus for successfully handling compound tokens
	MultiSpaceBonus = 0.15

	// SingleSpaceBonus is the bonus for good granular detection
	SingleSpaceBonus = 0.1

	// OversegmentationPenaltyRate is the penalty per excessive column
	OversegmentationPenaltyRate = 0.05

	// MaxOversegmentationPenalty is the maximum penalty for over-segmentation
	MaxOversegmentationPenalty = 0.3

	// VariancePenaltyThreshold is the threshold above which variance causes penalty
	VariancePenaltyThreshold = 0.5
)

// Gap Analysis Configuration
const (
	// MinGapForMajorColumn is the minimum gap to consider as a major column boundary
	MinGapForMajorColumn = 3

	// MinGapForProjection is the minimum gap for projection analysis
	MinGapForProjection = 4

	// MinSpacingBetweenMajorColumns is the minimum spacing between major columns
	MinSpacingBetweenMajorColumns = 8

	// MaxColumnsForOptimization is the threshold above which column optimization is applied
	MaxColumnsForOptimization = 10

	// MaxColumnsAllowed is the maximum number of columns allowed before optimization
	MaxColumnsAllowed = 12
)

// Alignment Analysis Configuration
const (
	// MinAlignmentRatio is the minimum ratio of aligned boundaries for compatibility
	MinAlignmentRatio = 0.75

	// MaxBoundaryRatio is the maximum ratio of boundary counts for compatibility
	MaxBoundaryRatio = 1.5

	// MinBoundariesForAnalysis is the minimum number of boundaries needed for analysis
	MinBoundariesForAnalysis = 3

	// MaxBoundariesForAnalysis is the maximum number of boundaries for analysis
	MaxBoundariesForAnalysis = 10

	// OverlapRatioThreshold is the minimum overlap ratio for layout similarity
	OverlapRatioThreshold = 0.3

	// MinTokenRatioForSimilarity is the minimum token ratio for layout similarity
	MinTokenRatioForSimilarity = 0.2

	// MinTokenRatioForMultiSpace is the token ratio for MultiSpace mode
	MinTokenRatioForMultiSpace = 0.15
)

// Merging and Optimization Configuration
const (
	// MaxConsecutiveGap is the maximum gap between consecutive segments for merging
	MaxConsecutiveGap = 3

	// MergingColumnAlignmentRatio is the required alignment ratio for merging segments
	MergingColumnAlignmentRatio = 0.7

	// MergingColumnToleranceMultiplier multiplies maxColumnVariance for merging tolerance
	MergingColumnToleranceMultiplier = 2

	// MergingBonus is the confidence bonus for successful segment merging
	MergingBonus = 0.1

	// OptimalColumnMin is the minimum number of columns considered optimal
	OptimalColumnMin = 3

	// OptimalColumnMax is the maximum number of columns considered optimal
	OptimalColumnMax = 8

	// OptimizationBonus is the bonus for achieving reasonable column count
	OptimizationBonus = 0.2

	// DockerPSOptimizationBonus is the special bonus for Docker PS optimization
	DockerPSOptimizationBonus = 0.15

	// ColumnReductionBonusRate is the bonus rate for column count reduction
	ColumnReductionBonusRate = 0.3

	// SignificantReductionThreshold is the threshold for significant column reduction
	SignificantReductionThreshold = 0.4

	// ReductionAcceptanceMultiplier is the multiplier for acceptance threshold during optimization
	ReductionAcceptanceMultiplier = 0.9
)

// Heuristic Thresholds
const (
	// ShortTokenRatioThreshold is the threshold for short token ratio in compound token detection
	ShortTokenRatioThreshold = 0.3

	// SingleSpaceRatioThreshold is the threshold for single space ratio in compound token detection
	SingleSpaceRatioThreshold = 0.2

	// MinStartFrequencyRatio is the minimum frequency ratio for start positions
	MinStartFrequencyRatio = 0.5

	// MinFrequencyForMajorColumn is the minimum frequency for major column detection
	MinFrequencyRatio = 0.33

	// PositionToleranceForAlignment is the tolerance for position alignment
	PositionToleranceForAlignment = 5

	// MaxColumnsForMajorColumnDetection is the maximum columns before optimization
	MaxColumnsForMajorColumnDetection = 8
)

// Quality and Validation Thresholds
const (
	// MinWordLength is the minimum length for extracted words
	MinWordLength = 3

	// MaxLinesForSmallData is the threshold for small data handling
	MaxLinesForSmallData = 3

	// MinSpacingConflictThreshold is the minimum spacing to avoid conflicts
	MinSpacingConflictThreshold = 2

	// MaxDistanceFromMiddle is the maximum distance preference from middle line
	MaxDistanceFromMiddle = 2
)

// Constants for numeric limits and bounds
const (
	// MinConfidenceScore is the absolute minimum confidence score
	MinConfidenceScore = 0.0

	// MaxConfidenceScore is the absolute maximum confidence score
	MaxConfidenceScore = 2.0

	// DefaultConfidenceScore is used when confidence cannot be calculated
	DefaultConfidenceScore = 1.0
)
