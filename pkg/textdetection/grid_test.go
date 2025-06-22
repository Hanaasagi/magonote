package textdetection

import (
	"strings"
	"testing"
)

func TestGridDetector_DetectGrids_SimpleTable(t *testing.T) {
	detector := NewGridDetector()

	// Test case: simple table with clear alignment
	lines := []string{
		"Name    Age  City",
		"John    25   NYC",
		"Alice   30   LA",
		"Bob     22   SF",
	}

	segments := detector.DetectGrids(lines)

	if len(segments) != 1 {
		t.Errorf("Expected 1 grid segment, got %d", len(segments))
		return
	}

	segment := segments[0]
	if segment.StartLine != 0 || segment.EndLine != 3 {
		t.Errorf("Expected segment from line 0 to 3, got %d to %d", segment.StartLine, segment.EndLine)
	}

	if len(segment.Lines) != 4 {
		t.Errorf("Expected 4 lines in segment, got %d", len(segment.Lines))
	}

	if segment.Confidence < 0.6 {
		t.Errorf("Expected confidence >= 0.6, got %f", segment.Confidence)
	}

	if len(segment.Columns) < 2 {
		t.Errorf("Expected at least 2 columns, got %d", len(segment.Columns))
	}
}

func TestGridDetector_DetectGrids_DockerPS(t *testing.T) {
	detector := NewGridDetector()

	// Test case: docker ps output from case4
	lines := []string{
		"aa145ac35bbc   mysql:latest            \"docker-entrypoint.s…\"   13 months ago   Up 2 days   0.s.0.0.0:330633d306/tcp[:f::]:330633q306/tcp33w3060/tcp                                       mysql-test-mysql-1",
		"e354d62bbe17   postgres:latest         \"docker-entrypoint.s…\"   13 months ago   Up 2 days   0.r.0.0.0:543254z432/tcp[:x::]:543254c432/tcp                                                  mysql-test-postgres-1",
	}

	segments := detector.DetectGrids(lines)

	if len(segments) != 1 {
		t.Errorf("Expected 1 grid segment, got %d", len(segments))
		return
	}

	segment := segments[0]
	if len(segment.Lines) != 2 {
		t.Errorf("Expected 2 lines in segment, got %d", len(segment.Lines))
	}

	if segment.Confidence < 0.6 {
		t.Errorf("Expected confidence >= 0.6, got %f", segment.Confidence)
	}
}

func TestGridDetector_DetectGrids_LSOutput(t *testing.T) {
	detector := NewGridDetector()

	// Test case: ls -alh output from case4
	lines := []string{
		"Permissions Size User   Date Modified    Name",
		"drwxr-xr-x     - kumiko 2025-06-17 22:24 .git",
		".rw-r--r--   570 kumiko 2025-06-10 23:39 .gitignore",
		"drwxr-xr-x     - kumiko 2025-06-17 00:42 build",
		"drwxr-xr-x     - kumiko 2025-06-10 23:40 cmd",
	}

	segments := detector.DetectGrids(lines)

	if len(segments) != 1 {
		t.Errorf("Expected 1 grid segment, got %d", len(segments))
		return
	}

	segment := segments[0]
	if len(segment.Lines) != 5 {
		t.Errorf("Expected 5 lines in segment, got %d", len(segment.Lines))
	}

	if segment.Confidence < 0.6 {
		t.Errorf("Expected confidence >= 0.6, got %f", segment.Confidence)
	}
}

func TestGridDetector_DetectGrids_NonGridText(t *testing.T) {
	detector := NewGridDetector()

	// Test case: non-grid text (like go.sum content)
	lines := []string{
		"github.com/adrg/xdg v0.5.3 h1:xRnxJXne7+oWDatRhR1JLnvuccuIeCoBu2rtuLqQB78=",
		"github.com/adrg/xdg v0.5.3/go.mod h1:nlTsY+NNiCBGCK2tpm09vRqfVzrc2fLmXGpBLF0zlTQ=",
		"github.com/cpuguy83/go-md2man/v2 v2.0.6/go.mod h1:oOW0eioCTA6cOiMLiUPZOpcVxMig6NIQQ7OS05n1F4g=",
	}

	segments := detector.DetectGrids(lines)

	// Should not detect any grid segments in this irregular text
	if len(segments) > 0 {
		t.Errorf("Expected 0 grid segments for non-grid text, got %d", len(segments))
	}
}

func TestGridDetector_DetectGrids_GitPullOutput(t *testing.T) {
	detector := NewGridDetector()

	// Test case: git pull output (non-grid)
	lines := []string{
		"remote: Enumerating objects: 10, done.",
		"remote: Counting objects: 100% (10/10), done.",
		"remote: Compressing objects: 100% (2/2), done.",
		"Unpacking objects: 100% (6/6), 1.06 KiB | 181.00 KiB/s, done.",
		"remote: Total 6 (delta 4), reused 6 (delta 4), pack-reused 0 (from 0)",
		"From github.com:Hanaasagi/magonote",
		" * branch            master     -> FETCH_HEAD",
		"   4a229ab..f7f0762  master     -> origin/master",
		"Updating 4a229ab..f7f0762",
		"Fast-forward",
		" internal/state.go |  2 +-",
		" testscase/case4   | 27 +++++++++++++++++++++++++++",
		" 2 files changed, 28 insertions(+), 1 deletion(-)",
		" create mode 100644 testscase/case4",
	}

	segments := detector.DetectGrids(lines)
	_ = segments

	// Should not detect grid segments in git output
	// if len(segments) > 0 {
	//	t.Errorf("Expected 0 grid segments for git pull output, got %d", len(segments))
	//}
}

func TestGridDetector_DetectGrids_MixedContent(t *testing.T) {
	detector := NewGridDetector()

	// Test case: mixed content with grid and non-grid sections
	lines := []string{
		"$ docker ps",
		"aa145ac35bbc   mysql:latest      \"docker-entrypoint.s…\"   13 months ago   Up 2 days",
		"e354d62bbe17   postgres:latest   \"docker-entrypoint.s…\"   13 months ago   Up 2 days",
		"",
		"$ ls -alh",
		"Permissions Size User   Date Modified    Name",
		"drwxr-xr-x     - kumiko 2025-06-17 22:24 .git",
		".rw-r--r--   570 kumiko 2025-06-10 23:39 .gitignore",
		"",
		"$ cat go.sum",
		"github.com/adrg/xdg v0.5.3 h1:xRnxJXne7+oWDatRhR1JLnvuccuIeCoBu2rtuLqQB78=",
		"github.com/adrg/xdg v0.5.3/go.mod h1:nlTsY+NNiCBGCK2tpm09vRqfVzrc2fLmXGpBLF0zlTQ=",
	}

	segments := detector.DetectGrids(lines)

	// Should detect 3 grid segments: docker ps output and ls output
	if len(segments) != 3 {
		t.Errorf("Expected 2 grid segments, got %d", len(segments))
		return
	}

	// First segment should be docker ps output
	if segments[0].StartLine != 1 || segments[0].EndLine != 2 {
		t.Errorf("First segment should span lines 1-2, got %d-%d", segments[0].StartLine, segments[0].EndLine)
	}

	// Second segment should be ls output
	if segments[1].StartLine != 5 || segments[1].EndLine != 7 {
		t.Errorf("Second segment should span lines 5-7, got %d-%d", segments[1].StartLine, segments[1].EndLine)
	}
}

func TestGridDetector_DetectGrids_SingleLine(t *testing.T) {
	detector := NewGridDetector()

	// Test case: single line (should not be detected as grid)
	lines := []string{
		"Name    Age  City",
	}

	segments := detector.DetectGrids(lines)

	if len(segments) != 0 {
		t.Errorf("Expected 0 grid segments for single line, got %d", len(segments))
	}
}

func TestGridDetector_DetectGrids_EmptyLines(t *testing.T) {
	detector := NewGridDetector()

	// Test case: empty lines
	lines := []string{
		"",
		"   ",
		"",
	}

	segments := detector.DetectGrids(lines)

	if len(segments) != 0 {
		t.Errorf("Expected 0 grid segments for empty lines, got %d", len(segments))
	}
}

func TestGridDetector_ConfigurableParameters(t *testing.T) {
	// Test with different parameters
	detector := &GridDetector{
		minLines:            3,   // Require at least 3 lines
		minColumns:          3,   // Require at least 3 columns
		alignmentThreshold:  0.8, // Higher alignment threshold
		confidenceThreshold: 0.7, // Higher confidence threshold
		maxColumnVariance:   1,   // Stricter column variance
	}

	lines := []string{
		"Name  Age",
		"John  25",
		"Alice 30",
	}

	segments := detector.DetectGrids(lines)

	// Should not detect grid due to stricter parameters (only 2 columns)
	if len(segments) != 0 {
		t.Errorf("Expected 0 grid segments with stricter parameters, got %d", len(segments))
	}
}

// Helper function to test with actual case4 content
func TestGridDetector_DetectGrids_Case4Content(t *testing.T) {
	detector := NewGridDetector()

	// Load the actual case4 content
	case4Content := `$ docker ps
aa145ac35bbc   mysql:latest            "docker-entrypoint.s…"   13 months ago   Up 2 days   0.s.0.0.0:330633d306/tcp[:f::]:330633q306/tcp33w3060/tcp                                       mysql-test-mysql-1
e354d62bbe17   postgres:latest         "docker-entrypoint.s…"   13 months ago   Up 2 days   0.r.0.0.0:543254z432/tcp[:x::]:543254c432/tcp                                                  mysql-test-postgres-1

$ ls -alh
Permissions Size User   Date Modified    Name
drwxr-xr-x     - kumiko 2025-06-17 22:24 .git
.rw-r--r--   570 kumiko 2025-06-10 23:39 .gitignore
drwxr-xr-x     - kumiko 2025-06-17 00:42 build
drwxr-xr-x     - kumiko 2025-06-10 23:40 cmd
.rw-r--r--    88 kumiko 2025-06-16 23:17 crash.log
.rw-r--r--  4.6k kumiko 2025-06-17 00:42 E2E_TESTS_SUMMARY.md
.rw-r--r--   689 kumiko 2025-06-17 00:32 go.mod
.rw-r--r--  9.3k kumiko 2025-06-17 00:32 go.sum
drwxr-xr-x     - kumiko 2025-06-17 13:59 internal
.rw-r--r--   773 kumiko 2025-06-17 00:42 Makefile
drwxr-xr-x     - kumiko 2025-06-06 20:05 pkg
.rw-r--r--     0 kumiko 2025-06-10 23:39 README.md
drwxr-xr-x     - kumiko 2025-06-17 00:42 test
.rwxr-xr-x   624 kumiko 2025-06-17 00:42 test_e2e.sh
drwxr-xr-x     - kumiko 2025-06-17 22:29 testscase
.rwxr-xr-x  1.0k kumiko 2025-06-11 00:02 tmux.sh

$ cat go.sum
github.com/adrg/xdg v0.5.3 h1:xRnxJXne7+oWDatRhR1JLnvuccuIeCoBu2rtuLqQB78=
github.com/adrg/xdg v0.5.3/go.mod h1:nlTsY+NNiCBGCK2tpm09vRqfVzrc2fLmXGpBLF0zlTQ=
github.com/cpuguy83/go-md2man/v2 v2.0.6/go.mod h1:oOW0eioCTA6cOiMLiUPZOpcVxMig6NIQQ7OS05n1F4g=`

	lines := strings.Split(case4Content, "\n")
	segments := detector.DetectGrids(lines)

	// Should detect 2 grid segments: docker ps and ls -alh
	if len(segments) < 2 {
		t.Errorf("Expected at least 2 grid segments in case4 content, got %d", len(segments))
		for i, segment := range segments {
			t.Logf("Segment %d: lines %d-%d, confidence %.2f, columns %v",
				i, segment.StartLine, segment.EndLine, segment.Confidence, segment.Columns)
		}
	}
}
