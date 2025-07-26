package tabledetection

// Detection Configuration Constants
// These constants define the behavior of the table detection algorithm

// Core Detection Parameters
const (
	// DefaultMinLines is the minimum number of lines required to form a table
	DefaultMinLines = 2

	// DefaultMinColumns is the minimum number of columns required to form a table
	DefaultMinColumns = 2

	// DefaultAlignmentThreshold is the threshold for column alignment consistency (0.0-1.0)
	DefaultAlignmentThreshold = 0.7

	// DefaultConfidenceThreshold is the minimum confidence to consider as table (0.0-1.0)
	DefaultConfidenceThreshold = 0.6

	// DefaultMaxColumnVariance is the maximum allowed variance in column positions
	DefaultMaxColumnVariance = 2
)

// Dual-Round Detection Configuration
const (
	// FirstRoundConfidenceThreshold is the confidence threshold for multi-space tokenization
	// (more lenient to capture compound tokens like "File Name")
	FirstRoundConfidenceThreshold = 0.4

	// FirstRoundMaxColumnVariance is the variance tolerance for multi-space tokenization
	// (more tolerant to handle uneven spacing in compound tokens)
	FirstRoundMaxColumnVariance = 3

	// SecondRoundConfidenceThreshold is the confidence threshold for single-space tokenization
	// (stricter to ensure quality of granular detection)
	SecondRoundConfidenceThreshold = 0.6

	// SecondRoundMaxColumnVariance is the variance tolerance for single-space tokenization
	// (stricter alignment requirements for fine-grained tokens)
	SecondRoundMaxColumnVariance = 2
)

// Tokenization Configuration
const (
	// MinTokenWidth is the minimum width required for token analysis
	// Used in projection analysis and alignment detection
	MinTokenWidth = 2

	// MinBoundariesForAnalysis is the minimum number of boundaries needed for alignment analysis
	// Used in tokenizer projection and boundary detection
	MinBoundariesForAnalysis = 3

	// MinSpacesForSingleSpaceMode defines how many consecutive spaces trigger separation in SingleSpaceMode
	MinSpacesForSingleSpaceMode = 1

	// MinSpacesForMultiSpaceMode defines how many consecutive spaces trigger separation in MultiSpaceMode
	MinSpacesForMultiSpaceMode = 2

	// CompoundTokenMinWidth is the minimum width for compound tokens in MultiSpaceMode
	CompoundTokenMinWidth = 3

	// MaxBoundaryRatio is the maximum ratio of boundary counts for compatibility analysis
	MaxBoundaryRatio = 1.5
)

// Word Extraction Configuration
const (
	// MinWordLength is the minimum length for extracted words
	MinWordLength = 3
)
