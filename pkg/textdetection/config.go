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

// Word Extraction Configuration
const (
	// MinWordLength is the minimum length of words to extract
	MinWordLength = 2
)
