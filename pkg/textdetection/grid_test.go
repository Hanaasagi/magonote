package textdetection

import (
	"strings"
	"testing"
)

// Helper function to validate grid segment content and positions
func validateSegment(t *testing.T, segment GridSegment, expectedStartLine, expectedEndLine int, expectedMinColumns int, expectedMinConfidence float64) {
	t.Helper()

	if segment.StartLine != expectedStartLine {
		t.Errorf("Expected start line %d, got %d", expectedStartLine, segment.StartLine)
	}

	if segment.EndLine != expectedEndLine {
		t.Errorf("Expected end line %d, got %d", expectedEndLine, segment.EndLine)
	}

	if len(segment.Columns) < expectedMinColumns {
		t.Errorf("Expected at least %d columns, got %d", expectedMinColumns, len(segment.Columns))
	}

	if segment.Confidence < expectedMinConfidence {
		t.Errorf("Expected confidence >= %.2f, got %.2f", expectedMinConfidence, segment.Confidence)
	}

	expectedLineCount := expectedEndLine - expectedStartLine + 1
	if len(segment.Lines) != expectedLineCount {
		t.Errorf("Expected %d lines in segment, got %d", expectedLineCount, len(segment.Lines))
	}
}

// Test basic table structure detection with specific content validation
func TestGridDetector_BasicTable(t *testing.T) {
	detector := NewGridDetector()

	lines := []string{
		"Name    Age  City",
		"John    25   NYC",
		"Alice   30   LA",
		"Bob     22   SF",
	}

	segments := detector.DetectGrids(lines)

	if len(segments) != 1 {
		t.Fatalf("Expected 1 grid segment, got %d", len(segments))
	}

	segment := segments[0]
	validateSegment(t, segment, 0, 3, 3, 0.7)

	// Validate specific content
	expectedContent := []string{"Name    Age  City", "John    25   NYC", "Alice   30   LA", "Bob     22   SF"}
	for i, expectedLine := range expectedContent {
		if segment.Lines[i] != expectedLine {
			t.Errorf("Line %d: expected '%s', got '%s'", i, expectedLine, segment.Lines[i])
		}
	}

	// Validate column positions make sense
	if segment.Columns[0] != 0 {
		t.Errorf("Expected first column at position 0, got %d", segment.Columns[0])
	}
}

// Test Docker PS output from x.go example - this is a perfect grid
func TestGridDetector_DockerPSOutput(t *testing.T) {
	detector := NewGridDetector(
		WithMinLines(3),
		WithMinColumns(3),
		WithConfidenceThreshold(0.8),
	)

	dockerLines := `
(py38) ➜  magonote git:(master) ✗ docker ps -a
CONTAINER ID   IMAGE                              COMMAND                  CREATED         STATUS                       PORTS     NAMES
5386a67b0f15   linuxserver/ffmpeg:5.1.2           "bash"                   13 months ago   Exited (255) 13 months ago             sad_austin
4c473036e5dc   linuxserver/ffmpeg:5.1.2           "bash"                   13 months ago   Exited (255) 13 months ago             exciting_nash
604575e35657   linuxserver/ffmpeg:5.1.2           "bash"                   13 months ago   Exited (127) 13 months ago             modest_haibt`

	lines := strings.Split(dockerLines, "\n")
	segments := detector.DetectGrids(lines)

	if len(segments) != 1 {
		t.Fatalf("Expected 1 grid segment for docker ps output, got %d", len(segments))
	}

	segment := segments[0]

	// Docker output should have high confidence and many columns
	validateSegment(t, segment, 2, 5, 5, 0.8)

	// Validate header line is present
	headerLine := segment.Lines[0]
	if !strings.Contains(headerLine, "CONTAINER ID") || !strings.Contains(headerLine, "IMAGE") {
		t.Errorf("Expected header line to contain CONTAINER ID and IMAGE, got: %s", headerLine)
	}

	// Validate data lines contain container IDs
	for i := 1; i < len(segment.Lines); i++ {
		line := segment.Lines[i]
		if len(line) < 12 { // Container IDs are 12 characters
			t.Errorf("Line %d appears too short for container data: %s", i, line)
		}
	}
}

// Test LS output from x.go example
func TestGridDetector_LSOutput(t *testing.T) {
	detector := NewGridDetector(
		WithMinLines(3),
		WithMinColumns(3),
		WithConfidenceThreshold(0.6),
	)

	lsLines := `
(py38) ➜  magonote git:(master) ✗ ls -la
-rw-r--r--   1 user  staff   1234 Jan 15 10:30 file1.txt
-rw-r--r--   1 user  staff   5678 Jan 16 11:45 file2.txt
drwxr-xr-x   1 user  staff    512 Jan 17 12:00 folder1
-rwxr-xr-x   1 user  staff   9999 Jan 18 13:15 script.sh`

	lines := strings.Split(lsLines, "\n")
	segments := detector.DetectGrids(lines)

	if len(segments) != 1 {
		t.Fatalf("Expected 1 grid segment for ls output, got %d", len(segments))
	}

	segment := segments[0]
	validateSegment(t, segment, 2, 5, 6, 0.6)

	// Validate permissions column
	for _, line := range segment.Lines {
		if len(line) < 10 {
			t.Errorf("Line too short to contain permissions: %s", line)
		}
		perms := line[:10]
		if !strings.Contains(perms, "r") && !strings.Contains(perms, "-") {
			t.Errorf("First column should contain file permissions, got: %s", perms)
		}
	}
}

// Test mixed content detection - should separate different grid types
func TestGridDetector_MixedContent(t *testing.T) {
	detector := NewGridDetector(
		WithMinLines(2),
		WithMinColumns(3),
		WithConfidenceThreshold(0.6),
	)

	mixedContent := `
(py38) ➜  magonote git:(master) ✗ docker ps
aa145ac35bbc   mysql:latest      "docker-entrypoint.s…"   13 months ago   Up 2 days
e354d62bbe17   postgres:latest   "docker-entrypoint.s…"   13 months ago   Up 2 days

(py38) ➜  magonote git:(master) ✗ ls -la
-rw-r--r--   1 user  staff   1234 file1.txt
drwxr-xr-x   1 user  staff    512 folder1

(py38) ➜  magonote git:(master) ✗ cat README.md
# This is a readme file
Some random text that should not be detected as grid`

	lines := strings.Split(mixedContent, "\n")
	segments := detector.DetectGrids(lines)

	if len(segments) < 2 {
		t.Fatalf("Expected at least 2 grid segments for mixed content, got %d", len(segments))
	}

	// First segment should be docker ps output
	dockerSegment := segments[0]
	if !strings.Contains(dockerSegment.Lines[0], "mysql") {
		t.Errorf("First segment should contain docker output, got: %s", dockerSegment.Lines[0])
	}

	// Second segment should be ls output
	lsSegment := segments[1]
	if !strings.Contains(lsSegment.Lines[0], "-rw-r--r--") {
		t.Errorf("Second segment should contain ls output, got: %s", lsSegment.Lines[0])
	}

	// Verify segments are properly separated
	if dockerSegment.EndLine >= lsSegment.StartLine {
		t.Errorf("Grid segments should not overlap: docker ends at %d, ls starts at %d",
			dockerSegment.EndLine, lsSegment.StartLine)
	}
}

// Test structured data that looks like a grid (go.sum format)
func TestGridDetector_StructuredData(t *testing.T) {
	detector := NewGridDetector()

	lines := []string{
		"github.com/adrg/xdg v0.5.3 h1:xRnxJXne7+oWDatRhR1JLnvuccuIeCoBu2rtuLqQB78=",
		"github.com/adrg/xdg v0.5.3/go.mod h1:nlTsY+NNiCBGCK2tpm09vRqfVzrc2fLmXGpBLF0zlTQ=",
		"github.com/cpuguy83/go-md2man/v2 v2.0.6/go.mod h1:oOW0eioCTA6cOiMLiUPZOpcVxMig6NIQQ7OS05n1F4g=",
	}

	segments := detector.DetectGrids(lines)

	// go.sum has a structured format that can be detected as a grid
	if len(segments) != 1 {
		t.Fatalf("Expected 1 grid segment for go.sum format, got %d", len(segments))
	}

	segment := segments[0]
	// The algorithm detects the first two similar lines as a grid (which is reasonable)
	validateSegment(t, segment, 0, 1, 2, 0.7)

	// Validate it detected the package name and hash columns
	for _, line := range segment.Lines {
		if !strings.Contains(line, "github.com") {
			t.Errorf("Expected github.com in package line: %s", line)
		}
		if !strings.Contains(line, "h1:") && !strings.Contains(line, "v0.5.3") {
			t.Errorf("Expected version or hash in line: %s", line)
		}
	}
}

// Test edge case: realistic minimum grid requirements
func TestGridDetector_MinimumGrid(t *testing.T) {
	detector := NewGridDetector(
		WithMinLines(2),
		WithMinColumns(2),
		WithConfidenceThreshold(0.5),
	)

	// More realistic minimal grid with clear column structure
	lines := []string{
		"Name  Value",
		"foo   123",
		"bar   456",
	}

	segments := detector.DetectGrids(lines)

	if len(segments) != 1 {
		t.Fatalf("Expected 1 grid segment for realistic minimal grid, got %d", len(segments))
	}

	segment := segments[0]
	validateSegment(t, segment, 0, 2, 2, 0.5)

	// Validate content
	expectedContent := []string{"Name  Value", "foo   123", "bar   456"}
	for i, expectedLine := range expectedContent {
		if segment.Lines[i] != expectedLine {
			t.Errorf("Line %d: expected '%s', got '%s'", i, expectedLine, segment.Lines[i])
		}
	}
}

// Test edge case: insufficient data
func TestGridDetector_InsufficientData(t *testing.T) {
	detector := NewGridDetector()

	testCases := []struct {
		name  string
		lines []string
	}{
		{"single line", []string{"Name Age City"}},
		{"empty lines", []string{"", "", ""}},
		{"irregular text", []string{"Hello", "This is just", "random text without structure"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			segments := detector.DetectGrids(tc.lines)
			if len(segments) > 0 {
				t.Errorf("Expected 0 grid segments for %s, got %d", tc.name, len(segments))
			}
		})
	}
}

// Test edge case: command line patterns that should be excluded
func TestGridDetector_CommandLineExclusion(t *testing.T) {
	detector := NewGridDetector()

	lines := []string{
		"$ docker ps",
		"CONTAINER ID   IMAGE     COMMAND",
		"abc123         nginx     nginx",
		"",
		"(py38) ➜  ~ git status",
		"On branch main",
		"nothing to commit",
	}

	segments := detector.DetectGrids(lines)

	// Should detect only the docker output, not the command lines
	if len(segments) != 1 {
		t.Fatalf("Expected 1 grid segment, got %d", len(segments))
	}

	segment := segments[0]

	// Should start after the command line
	if segment.StartLine == 0 {
		t.Errorf("Grid should not include command line starting with $")
	}

	// Should contain the header
	if !strings.Contains(segment.Lines[0], "CONTAINER ID") {
		t.Errorf("Grid should start with header line, got: %s", segment.Lines[0])
	}
}

// Test configurable parameters affect detection
func TestGridDetector_ConfigurableThresholds(t *testing.T) {
	strictDetector := NewGridDetector(
		WithMinLines(4),
		WithMinColumns(4),
		WithConfidenceThreshold(0.9),
	)

	lenientDetector := NewGridDetector(
		WithMinLines(2),
		WithMinColumns(2),
		WithConfidenceThreshold(0.3),
	)

	// Borderline case that should be detected by lenient but not strict
	lines := []string{
		"A  B  C",
		"1  2  3",
		"X  Y  Z",
	}

	strictSegments := strictDetector.DetectGrids(lines)
	lenientSegments := lenientDetector.DetectGrids(lines)

	// Strict detector should reject (not enough lines/columns)
	if len(strictSegments) > 0 {
		t.Errorf("Strict detector should reject borderline case, got %d segments", len(strictSegments))
	}

	// Lenient detector should accept
	if len(lenientSegments) != 1 {
		t.Errorf("Lenient detector should accept borderline case, got %d segments", len(lenientSegments))
	}
}

// Test real-world example from x.go
func TestGridDetector_RealWorldExample(t *testing.T) {
	detector := NewGridDetector(
		WithMinLines(2),
		WithMinColumns(3),
		WithConfidenceThreshold(0.6),
	)

	// Full content from x.go
	rawContent := `
(py38) ➜  magonote git:(master) ✗ docker ps -a
CONTAINER ID   IMAGE                              COMMAND                  CREATED         STATUS                       PORTS     NAMES
5386a67b0f15   linuxserver/ffmpeg:5.1.2           "bash"                   13 months ago   Exited (255) 13 months ago             sad_austin
4c473036e5dc   linuxserver/ffmpeg:5.1.2           "bash"                   13 months ago   Exited (255) 13 months ago             exciting_nash
604575e35657   linuxserver/ffmpeg:5.1.2           "bash"                   13 months ago   Exited (127) 13 months ago             modest_haibt
0cca8fa8a622   linuxserver/ffmpeg:5.1.2           "/ffmpegwrapper.sh sh"   13 months ago   Exited (0) 13 months ago               exciting_mirzakhani
(py38) ➜  magonote git:(master) ✗ ls
b.go  build  cmd                  fuck.go   go.mod  internal  magonote.tmux  main.go   pkg        start.sh  test   tt.sh
bug   case   config.example.toml  fuck2.go  go.sum  LICENSE   main           Makefile  README.md  t.sh      tools  x.go`

	lines := strings.Split(rawContent, "\n")
	segments := detector.DetectGrids(lines)

	// Should detect docker ps table and ls output
	if len(segments) < 1 {
		t.Fatalf("Expected at least 1 grid segment in real-world example, got %d", len(segments))
	}

	// First segment should be the docker ps output
	dockerSegment := segments[0]
	if !strings.Contains(dockerSegment.Lines[0], "CONTAINER ID") {
		t.Errorf("Expected docker ps header in first segment, got: %s", dockerSegment.Lines[0])
	}

	// Validate the docker segment has reasonable structure
	if len(dockerSegment.Columns) < 5 {
		t.Errorf("Docker ps output should have at least 5 columns, got %d", len(dockerSegment.Columns))
	}

	if dockerSegment.Confidence < 0.8 {
		t.Errorf("Docker ps output should have high confidence, got %.2f", dockerSegment.Confidence)
	}
}
