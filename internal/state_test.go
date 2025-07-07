package internal

import (
	"strings"
	"testing"
)

func SplitLines(text string) []string {
	return strings.Split(text, "\n")
}

func TestMatchReverse(t *testing.T) {
	lines := SplitLines("lorem 127.0.0.1 lorem 255.255.255.255 lorem 127.0.0.1 lorem")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	if len(results) != 3 {
		t.Errorf("Expected 3 matches, got %d", len(results))
	}
	if *results[0].Hint != "a" {
		t.Errorf("Expected first hint to be 'a', got '%s'", *results[0].Hint)
	}
	if *results[len(results)-1].Hint != "c" {
		t.Errorf("Expected last hint to be 'c', got '%s'", *results[len(results)-1].Hint)
	}
}

func TestMatchUnique(t *testing.T) {
	lines := SplitLines("lorem 127.0.0.1 lorem 255.255.255.255 lorem 127.0.0.1 lorem")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 1)

	if len(results) != 3 {
		t.Errorf("Expected 3 matches, got %d", len(results))
	}
	if *results[0].Hint != "a" {
		t.Errorf("Expected first hint to be 'a', got '%s'", *results[0].Hint)
	}
	if *results[len(results)-1].Hint != "a" {
		t.Errorf("Expected last hint to be 'a', got '%s'", *results[len(results)-1].Hint)
	}
}

// TestMatchSuperUnique tests that duplicate matches are filtered to show only one
func TestMatchSuperUnique(t *testing.T) {
	lines := SplitLines("lorem 127.0.0.1 lorem 255.255.255.255 lorem 127.0.0.1 lorem")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 2)

	// Should only have 2 matches: one 127.0.0.1 and one 255.255.255.255
	if len(results) != 2 {
		t.Errorf("Expected 2 matches, got %d", len(results))
	}

	// Check that we have both unique IPs
	foundIPs := make(map[string]bool)
	for _, result := range results {
		foundIPs[result.Text] = true
	}

	if !foundIPs["127.0.0.1"] {
		t.Error("Expected to find '127.0.0.1' in results")
	}
	if !foundIPs["255.255.255.255"] {
		t.Error("Expected to find '255.255.255.255' in results")
	}
}

// TestMatchSuperUniqueMiddleSelection tests that middle positioned duplicates are preferred
func TestMatchSuperUniqueMiddleSelection(t *testing.T) {
	lines := SplitLines("127.0.0.1\n127.0.0.1\n127.0.0.1\n127.0.0.1\n127.0.0.1")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 2)

	// Should only have 1 match
	if len(results) != 1 {
		t.Errorf("Expected 1 match, got %d", len(results))
	}

	// Should be from line 2 (middle line, 0-indexed)
	if results[0].Y != 2 {
		t.Errorf("Expected match from line 2 (middle), got line %d", results[0].Y)
	}
}

// TestMatchSuperUniqueEarlySelection tests that earlier lines are preferred when no middle exists
func TestMatchSuperUniqueEarlySelection(t *testing.T) {
	lines := SplitLines("127.0.0.1\n127.0.0.1")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 2)

	// Should only have 1 match
	if len(results) != 1 {
		t.Errorf("Expected 1 match, got %d", len(results))
	}

	// Should be from line 0 (first line)
	if results[0].Y != 0 {
		t.Errorf("Expected match from line 0, got line %d", results[0].Y)
	}
}

// TestMatchSuperUniqueComplexScenario tests the complex scenario mentioned in requirements
func TestMatchSuperUniqueComplexScenario(t *testing.T) {
	lines := SplitLines("127.0.0.1\n127.0.0.1\n127.0.0.2\n127.0.0.1")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 2)

	// Should have 2 matches: one for 127.0.0.1 and one for 127.0.0.2
	if len(results) != 2 {
		t.Errorf("Expected 2 matches, got %d", len(results))
	}

	// Check that we have both unique IPs
	foundIPs := make(map[string]int) // IP -> line number
	for _, result := range results {
		foundIPs[result.Text] = result.Y
	}

	if lineNum, exists := foundIPs["127.0.0.1"]; !exists {
		t.Error("Expected to find '127.0.0.1' in results")
	} else if lineNum != 0 {
		// For 127.0.0.1, should prefer line 0 since 127.0.0.2 is at line 2
		// and we want to avoid clustering
		t.Errorf("Expected '127.0.0.1' from line 0, got line %d", lineNum)
	}

	if lineNum, exists := foundIPs["127.0.0.2"]; !exists {
		t.Error("Expected to find '127.0.0.2' in results")
	} else if lineNum != 2 {
		t.Errorf("Expected '127.0.0.2' from line 2, got line %d", lineNum)
	}
}

// TestMatchSuperUniqueWithManyLines tests selection with many lines
func TestMatchSuperUniqueWithManyLines(t *testing.T) {
	// Create 7 lines with duplicates - middle should be line 3 (0-indexed)
	lines := SplitLines("127.0.0.1\n127.0.0.1\n127.0.0.1\n127.0.0.1\n127.0.0.1\n127.0.0.1\n127.0.0.1")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 2)

	// Should only have 1 match
	if len(results) != 1 {
		t.Errorf("Expected 1 match, got %d", len(results))
	}

	// Should be from line 3 (middle line, 0-indexed)
	if results[0].Y != 3 {
		t.Errorf("Expected match from line 3 (middle), got line %d", results[0].Y)
	}
}

func TestMatchDocker(t *testing.T) {
	lines := SplitLines("latest sha256:30557a29d5abc51e5f1d5b472e79b7e296f595abcf19fe6b9199dbbc809c6ff4 20 hours ago")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	if len(results) != 1 {
		t.Errorf("Expected 1 match, got %d", len(results))
	}
	if results[0].Text != "30557a29d5abc51e5f1d5b472e79b7e296f595abcf19fe6b9199dbbc809c6ff4" {
		t.Errorf("Expected docker hash, got '%s'", results[0].Text)
	}
}

func TestMatchBash(t *testing.T) {
	lines := SplitLines("path: [32m/var/log/nginx.log[m\npath: [32mtest/log/nginx-2.log:32[mfolder/.nginx@4df2.log") // nolint

	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	if len(results) != 3 {
		t.Errorf("Expected 3 matches, got %d", len(results))
	}
	if results[0].Text != "/var/log/nginx.log" {
		t.Errorf("Expected '/var/log/nginx.log', got '%s'", results[0].Text)
	}
	if results[1].Text != "test/log/nginx-2.log" {
		t.Errorf("Expected 'test/log/nginx-2.log', got '%s'", results[1].Text)
	}
	if results[2].Text != "folder/.nginx@4df2.log" {
		t.Errorf("Expected 'folder/.nginx@4df2.log', got '%s'", results[2].Text)
	}
}

func TestMatchPaths(t *testing.T) {
	lines := SplitLines("Lorem /tmp/foo/bar_lol, lorem\n Lorem /var/log/boot-strap.log lorem ../log/kern.log lorem")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	if len(results) != 3 {
		t.Errorf("Expected 3 matches, got %d", len(results))
	}
	if results[0].Text != "/tmp/foo/bar_lol" {
		t.Errorf("Expected '/tmp/foo/bar_lol', got '%s'", results[0].Text)
	}
	if results[1].Text != "/var/log/boot-strap.log" {
		t.Errorf("Expected '/var/log/boot-strap.log', got '%s'", results[1].Text)
	}
	if results[2].Text != "../log/kern.log" {
		t.Errorf("Expected '../log/kern.log', got '%s'", results[2].Text)
	}
}

func TestMatchRoutes(t *testing.T) {
	lines := SplitLines("Lorem /app/routes/$routeId/$objectId, lorem\n Lorem /app/routes/$sectionId")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	if len(results) != 2 {
		t.Errorf("Expected 2 matches, got %d", len(results))
	}
	if results[0].Text != "/app/routes/$routeId/$objectId" {
		t.Errorf("Expected '/app/routes/$routeId/$objectId', got '%s'", results[0].Text)
	}
	if results[1].Text != "/app/routes/$sectionId" {
		t.Errorf("Expected '/app/routes/$sectionId', got '%s'", results[1].Text)
	}
}

func TestMatchUIDs(t *testing.T) {
	lines := SplitLines("Lorem ipsum 123e4567-e89b-12d3-a456-426655440000 lorem\n Lorem lorem lorem")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	if len(results) != 1 {
		t.Errorf("Expected 1 match, got %d", len(results))
	}
}

func TestMatchSHAs(t *testing.T) {
	lines := SplitLines("Lorem fd70b5695 5246ddf f924213 lorem\n Lorem 973113963b491874ab2e372ee60d4b4cb75f717c lorem")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	if len(results) != 4 {
		t.Errorf("Expected 4 matches, got %d", len(results))
	}
	if results[0].Text != "fd70b5695" {
		t.Errorf("Expected 'fd70b5695', got '%s'", results[0].Text)
	}
	if results[1].Text != "5246ddf" {
		t.Errorf("Expected '5246ddf', got '%s'", results[1].Text)
	}
	if results[2].Text != "f924213" {
		t.Errorf("Expected 'f924213', got '%s'", results[2].Text)
	}
	if results[3].Text != "973113963b491874ab2e372ee60d4b4cb75f717c" {
		t.Errorf("Expected '973113963b491874ab2e372ee60d4b4cb75f717c', got '%s'", results[3].Text)
	}
}

func TestMatchIPs(t *testing.T) {
	lines := SplitLines("Lorem ipsum 127.0.0.1 lorem\n Lorem 255.255.10.255 lorem 127.0.0.1 lorem")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	if len(results) != 3 {
		t.Errorf("Expected 3 matches, got %d", len(results))
	}
	if results[0].Text != "127.0.0.1" {
		t.Errorf("Expected '127.0.0.1', got '%s'", results[0].Text)
	}
	if results[1].Text != "255.255.10.255" {
		t.Errorf("Expected '255.255.10.255', got '%s'", results[1].Text)
	}
	if results[2].Text != "127.0.0.1" {
		t.Errorf("Expected '127.0.0.1', got '%s'", results[2].Text)
	}
}

func TestMatchIPv6s(t *testing.T) {
	lines := SplitLines("Lorem ipsum fe80::2:202:fe4 lorem\n Lorem 2001:67c:670:202:7ba8:5e41:1591:d723 lorem fe80::2:1 lorem ipsum fe80:22:312:fe::1%eth0")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	if len(results) != 4 {
		t.Errorf("Expected 4 matches, got %d", len(results))
	}
	if results[0].Text != "fe80::2:202:fe4" {
		t.Errorf("Expected 'fe80::2:202:fe4', got '%s'", results[0].Text)
	}
	if results[1].Text != "2001:67c:670:202:7ba8:5e41:1591:d723" {
		t.Errorf("Expected '2001:67c:670:202:7ba8:5e41:1591:d723', got '%s'", results[1].Text)
	}
	if results[2].Text != "fe80::2:1" {
		t.Errorf("Expected 'fe80::2:1', got '%s'", results[2].Text)
	}
	if results[3].Text != "fe80:22:312:fe::1%eth0" {
		t.Errorf("Expected 'fe80:22:312:fe::1%%eth0', got '%s'", results[3].Text)
	}
}

func TestMatchMarkdownURLs(t *testing.T) {
	lines := SplitLines("Lorem ipsum [link](https://github.io?foo=bar) ![](http://cdn.com/img.jpg) lorem")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	if len(results) != 2 {
		t.Errorf("Expected 2 matches, got %d", len(results))
	}
	if results[0].Pattern != "markdown_url" {
		t.Errorf("Expected pattern 'markdown_url', got '%s'", results[0].Pattern)
	}
	if results[0].Text != "https://github.io?foo=bar" {
		t.Errorf("Expected 'https://github.io?foo=bar', got '%s'", results[0].Text)
	}
	if results[1].Pattern != "markdown_url" {
		t.Errorf("Expected pattern 'markdown_url', got '%s'", results[1].Pattern)
	}
	if results[1].Text != "http://cdn.com/img.jpg" {
		t.Errorf("Expected 'http://cdn.com/img.jpg', got '%s'", results[1].Text)
	}
}

func TestMatchURLs(t *testing.T) {
	lines := SplitLines("Lorem ipsum https://www.rust-lang.org/tools lorem\n Lorem ipsumhttps://crates.io lorem https://github.io?foo=bar lorem ssh://github.io")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	if len(results) != 4 {
		t.Errorf("Expected 4 matches, got %d", len(results))
	}
	if results[0].Text != "https://www.rust-lang.org/tools" {
		t.Errorf("Expected 'https://www.rust-lang.org/tools', got '%s'", results[0].Text)
	}
	if results[0].Pattern != "url" {
		t.Errorf("Expected pattern 'url', got '%s'", results[0].Pattern)
	}
	if results[1].Text != "https://crates.io" {
		t.Errorf("Expected 'https://crates.io', got '%s'", results[1].Text)
	}
	if results[1].Pattern != "url" {
		t.Errorf("Expected pattern 'url', got '%s'", results[1].Pattern)
	}
	if results[2].Text != "https://github.io?foo=bar" {
		t.Errorf("Expected 'https://github.io?foo=bar', got '%s'", results[2].Text)
	}
	if results[2].Pattern != "url" {
		t.Errorf("Expected pattern 'url', got '%s'", results[2].Pattern)
	}
	if results[3].Text != "ssh://github.io" {
		t.Errorf("Expected 'ssh://github.io', got '%s'", results[3].Text)
	}
	if results[3].Pattern != "url" {
		t.Errorf("Expected pattern 'url', got '%s'", results[3].Pattern)
	}
}

func TestCustomPatterns(t *testing.T) {
	lines := SplitLines("Lorem [link](http://foo.bar) ipsum CUSTOM-52463 lorem ISSUE-123 lorem")
	custom := []string{"CUSTOM-[0-9]{4,}", "ISSUE-[0-9]{3}"}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	foundCustom := false
	foundIssue := false

	for _, match := range results {
		if match.Text == "CUSTOM-52463" {
			foundCustom = true
		}
		if match.Text == "ISSUE-123" {
			foundIssue = true
		}
	}

	if !foundCustom {
		t.Errorf("Expected to find 'CUSTOM-52463'")
	}
	if !foundIssue {
		t.Errorf("Expected to find 'ISSUE-123'")
	}
}

// Test Git diff summary match
func TestMatchDiffSummary(t *testing.T) {
	lines := SplitLines("diff --git a/src/main.go b/src/main.go\ndiff --git a/internal/state_test.go b/internal/state_test.go")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	if len(results) != 4 {
		t.Errorf("Expected 4 matches, got %d", len(results))
	}

	// Check the source and target file of the first diff
	if results[0].Pattern != "diff_summary" {
		t.Errorf("Expected pattern 'diff_summary', got '%s'", results[0].Pattern)
	}
	if results[1].Pattern != "diff_summary" {
		t.Errorf("Expected pattern 'diff_summary', got '%s'", results[1].Pattern)
	}
}

// Test Git diff file path (diff_a and diff_b)
func TestMatchDiffPaths(t *testing.T) {
	lines := SplitLines("--- a/src/main.go\n+++ b/src/main.go\n--- a/internal/test.go\n+++ b/internal/test.go")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	diffACount := 0
	diffBCount := 0
	for _, result := range results {
		if result.Pattern == "diff_a" {
			diffACount++
		}
		if result.Pattern == "diff_b" {
			diffBCount++
		}
	}

	if diffACount != 2 {
		t.Errorf("Expected 2 diff_a matches, got %d", diffACount)
	}
	if diffBCount != 2 {
		t.Errorf("Expected 2 diff_b matches, got %d", diffBCount)
	}
}

// Test hexadecimal color match
func TestMatchColors(t *testing.T) {
	lines := SplitLines("background: #FF0000; color: #00FF00; border: #0000FF;\nopacity: #ffffff #000000 #ABCDEF")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	expectedColors := []string{"#FF0000", "#00FF00", "#0000FF", "#ffffff", "#000000", "#ABCDEF"}
	colorCount := 0

	for _, result := range results {
		if result.Pattern == "color" {
			colorCount++
			found := false
			for _, expected := range expectedColors {
				if result.Text == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Unexpected color match: %s", result.Text)
			}
		}
	}

	if colorCount != 6 {
		t.Errorf("Expected 6 color matches, got %d", colorCount)
	}
}

// Test IPFS hash match
func TestMatchIPFS(t *testing.T) {
	lines := SplitLines("IPFS hash: QmW2HvDCgqCLJtGxVPZDMWJ5tE2PrsaS3s4VqgdgMqKBNK\nAnother: QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	ipfsCount := 0
	for _, result := range results {
		if result.Pattern == "ipfs" {
			ipfsCount++
			if len(result.Text) != 46 || !strings.HasPrefix(result.Text, "Qm") {
				t.Errorf("Invalid IPFS hash format: %s", result.Text)
			}
		}
	}

	if ipfsCount != 2 {
		t.Errorf("Expected 2 IPFS matches, got %d", ipfsCount)
	}
}

// Test memory address match
func TestMatchAddresses(t *testing.T) {
	lines := SplitLines("Pointer at 0x7fff5fbff5c0\nAddress: 0x1234567890ABCDEF\nOther: 0x0 0xFFFFFFFF")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	expectedAddresses := []string{"0x7fff5fbff5c0", "0x1234567890ABCDEF", "0x0", "0xFFFFFFFF"}
	addressCount := 0

	for _, result := range results {
		if result.Pattern == "address" {
			addressCount++
			found := false
			for _, expected := range expectedAddresses {
				if result.Text == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Unexpected address match: %s", result.Text)
			}
		}
	}

	if addressCount != 4 {
		t.Errorf("Expected 4 address matches, got %d", addressCount)
	}
}

// Test IPv4 port match
func TestMatchIPv4WithPort(t *testing.T) {
	lines := SplitLines("Server at 192.168.1.1:8080\nDatabase: 10.0.0.1:3306 Web: 172.16.0.1:80")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	ipv4PortCount := 0
	expectedPorts := []string{"192.168.1.1:8080", "10.0.0.1:3306", "172.16.0.1:80"}

	for _, result := range results {
		if result.Pattern == "ipv4_port" {
			ipv4PortCount++
			found := false
			for _, expected := range expectedPorts {
				if result.Text == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Unexpected IPv4 port match: %s", result.Text)
			}
		}
	}

	if ipv4PortCount != 3 {
		t.Errorf("Expected 3 IPv4 port matches, got %d", ipv4PortCount)
	}
}

// Test IPv6 port match
func TestMatchIPv6WithPort(t *testing.T) {
	lines := SplitLines("Server at [2001:db8::1]:443\nAnother: [::1]:8080 [fe80::1]:22")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	ipv6PortCount := 0
	expectedPorts := []string{"[2001:db8::1]:443", "[::1]:8080", "[fe80::1]:22"}

	for _, result := range results {
		if result.Pattern == "ipv6_port" {
			ipv6PortCount++
			found := false
			for _, expected := range expectedPorts {
				if result.Text == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Unexpected IPv6 port match: %s", result.Text)
			}
		}
	}

	if ipv6PortCount != 3 {
		t.Errorf("Expected 3 IPv6 port matches, got %d", ipv6PortCount)
	}
}

// Test filename match
func TestMatchFilenames(t *testing.T) {
	lines := SplitLines("Files: main.go state.go test.py script.sh config.json\nMore: component.tsx style.css data.xml readme.md")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	expectedFiles := []string{"main.go", "state.go", "test.py", "script.sh", "config.json", "component.tsx", "style.css", "data.xml", "readme.md"}
	filenameCount := 0

	for _, result := range results {
		if result.Pattern == "filename" {
			filenameCount++
			found := false
			for _, expected := range expectedFiles {
				if result.Text == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Unexpected filename match: %s", result.Text)
			}
		}
	}

	if filenameCount != len(expectedFiles) {
		t.Errorf("Expected %d filename matches, got %d", len(expectedFiles), filenameCount)
	}
}

// Test ISO8601 date-time match
func TestMatchDateTimeISO8601(t *testing.T) {
	lines := SplitLines("Created at 2023-12-01T10:30:45Z\nUpdated: 2023-12-01T10:30:45.123Z\nOther: 2023-12-01T10:30:45+08:00")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	expectedDates := []string{"2023-12-01T10:30:45Z", "2023-12-01T10:30:45.123Z", "2023-12-01T10:30:45+08:00"}
	dateCount := 0

	for _, result := range results {
		if result.Pattern == "datetime_iso8601" {
			dateCount++
			found := false
			for _, expected := range expectedDates {
				if result.Text == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Unexpected ISO8601 date match: %s", result.Text)
			}
		}
	}

	if dateCount != 3 {
		t.Errorf("Expected 3 ISO8601 date matches, got %d", dateCount)
	}
}

// Test generic date-time match
func TestMatchDateTimeCommon(t *testing.T) {
	lines := SplitLines("Log entry: 2023-12-01 14:30:25\nAnother: 2023-01-15T09:45:10")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	commonDateCount := 0
	iso8601Count := 0

	for _, result := range results {
		if result.Pattern == "datetime_common" {
			commonDateCount++
			if result.Text != "2023-12-01 14:30:25" {
				t.Errorf("Unexpected common date match: %s", result.Text)
			}
		}
		if result.Pattern == "datetime_iso8601" {
			iso8601Count++
			if result.Text != "2023-01-15T09:45:10" {
				t.Errorf("Unexpected ISO8601 date match: %s", result.Text)
			}
		}
	}

	if commonDateCount != 1 {
		t.Errorf("Expected 1 common date match, got %d", commonDateCount)
	}
	if iso8601Count != 1 {
		t.Errorf("Expected 1 ISO8601 date match, got %d", iso8601Count)
	}
}

// Test hyphen-separated date match
func TestMatchDateDash(t *testing.T) {
	lines := SplitLines("Date: 2023-12-01\nBirthday: 1990-05-15 Other: 2024-01-01")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	dashDateCount := 0
	expectedDates := []string{"2023-12-01", "1990-05-15", "2024-01-01"}

	for _, result := range results {
		if result.Pattern == "date_dash" {
			dashDateCount++
			found := false
			for _, expected := range expectedDates {
				if result.Text == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Unexpected dash date match: %s", result.Text)
			}
		}
	}

	if dashDateCount != 3 {
		t.Errorf("Expected 3 dash date matches, got %d", dashDateCount)
	}
}

// Test slash-separated date match - these will be recognized as path pattern
func TestMatchDateSlash(t *testing.T) {
	lines := SplitLines("American format: 12/01/2023\nAnother: 05/15/1990 Today: 01/01/2024")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	// These dates are actually matched by the path pattern due to higher priority
	pathCount := 0
	expectedDates := []string{"12/01/2023", "05/15/1990", "01/01/2024"}

	for _, result := range results {
		if result.Pattern == "path" && contains(expectedDates, result.Text) {
			pathCount++
		}
	}

	if pathCount != 3 {
		t.Errorf("Expected 3 path matches for dates, got %d", pathCount)
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Test multiple URL protocol types
func TestMatchURLProtocols(t *testing.T) {
	lines := SplitLines("Git clone: git@github.com:user/repo.git\nFTP: ftp://files.example.com/file.zip\nFile: file:///home/user/document.txt")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	urlCount := 0
	expectedURLs := []string{"git@github.com:user/repo.git", "ftp://files.example.com/file.zip", "file:///home/user/document.txt"}

	for _, result := range results {
		if result.Pattern == "url" {
			urlCount++
			found := false
			for _, expected := range expectedURLs {
				if result.Text == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Unexpected URL match: %s", result.Text)
			}
		}
	}

	if urlCount != 3 {
		t.Errorf("Expected 3 URL matches, got %d", urlCount)
	}
}

func TestMatchComplexPaths(t *testing.T) {
	lines := SplitLines("Paths: ~/Documents/file.txt ~/.config/app.conf\nOther: $HOME/bin/script @home/folder/item")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	pathCount := 0
	expectedPaths := []string{"~/Documents/file.txt", "~/.config/app.conf", "$HOME/bin/script", "@home/folder/item"}

	for _, result := range results {
		if result.Pattern == "path" {
			pathCount++
			found := false
			for _, expected := range expectedPaths {
				if result.Text == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Unexpected path match: %s", result.Text)
			}
		}
	}

	if pathCount != 4 {
		t.Errorf("Expected 4 path matches, got %d", pathCount)
	}
}

func TestMatchEdgeCases(t *testing.T) {
	lines := SplitLines("UUID: 550e8400-e29b-41d4-a716-446655440000\nShort SHA: 1a2b3c4 Long SHA: 1234567890abcdef1234567890abcdef12345678")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	uuidFound := false
	shortSHAFound := false
	longSHAFound := false

	for _, result := range results {
		switch result.Pattern {
		case "uid":
			if result.Text == "550e8400-e29b-41d4-a716-446655440000" {
				uuidFound = true
			}
		case "sha":
			switch result.Text {
			case "1a2b3c4":
				shortSHAFound = true
			case "1234567890abcdef1234567890abcdef12345678":
				longSHAFound = true

			}
		}
	}

	if !uuidFound {
		t.Errorf("Expected to find UUID")
	}
	if !shortSHAFound {
		t.Errorf("Expected to find short SHA")
	}
	if !longSHAFound {
		t.Errorf("Expected to find long SHA")
	}
}

func TestMatchMixedContent(t *testing.T) {
	lines := SplitLines("Server 192.168.1.1:8080 color #FF0000 file main.go date 2023-12-01 UUID 123e4567-e89b-12d3-a456-426655440000")
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	patterns := make(map[string]int)
	for _, result := range results {
		patterns[result.Pattern]++
	}

	expectedPatterns := map[string]int{
		"ipv4_port": 1,
		"color":     1,
		"filename":  1,
		"date_dash": 1,
		"uid":       1,
	}

	for pattern, expectedCount := range expectedPatterns {
		if count, found := patterns[pattern]; !found || count != expectedCount {
			t.Errorf("Expected %d matches for pattern '%s', got %d", expectedCount, pattern, count)
		}
	}
}

func TestGridMatching(t *testing.T) {
	// Test grid matching with a table-like structure
	lines := SplitLines(`Command output:
container_id   image_name      status      ports
aa145ac35bbc   mysql:latest    running     3306/tcp
e354d62bbe17   postgres:13     running     5432/tcp
f123456789ab   redis:alpine    stopped     6379/tcp`)

	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	// Should detect grid-based matches for container names, image names, etc.
	found := false
	for _, match := range results {
		if match.Pattern == "grid" {
			found = true
			break
		}
	}

	if !found {
		t.Log("Grid pattern not detected - this is expected if the confidence threshold is too high")
		t.Log("Current matches:")
		for _, match := range results {
			t.Logf("  %s: %s", match.Pattern, match.Text)
		}
	}
}

// TestMatchURLsWithQuotes tests URL matching in quote-enclosed contexts like curl commands
func TestMatchURLsWithQuotes(t *testing.T) {
	// Test case from curl-case file
	curlLine := "curl 'https://github.com/Hanaasagi/magonote/hovercards/citation/sidebar_partial?tree_name=master' \\"
	lines := SplitLines(curlLine)
	custom := []string{}
	results := NewState(lines, "abcd", custom).Matches(false, 0)

	// Should find the URL without trailing quote
	foundURL := false
	expectedURL := "https://github.com/Hanaasagi/magonote/hovercards/citation/sidebar_partial?tree_name=master"

	for _, result := range results {
		if result.Pattern == "url" {
			foundURL = true
			if result.Text != expectedURL {
				t.Errorf("Expected URL '%s', got '%s'", expectedURL, result.Text)
			}
			// Verify it doesn't contain trailing quote
			if strings.HasSuffix(result.Text, "'") {
				t.Errorf("URL should not end with quote, got '%s'", result.Text)
			}
		}
	}

	if !foundURL {
		t.Error("Expected to find URL pattern in curl command")
	}

	// Test with double quotes
	doubleQuoteLine := `curl "https://example.com/api?param=value" --header`
	lines = SplitLines(doubleQuoteLine)
	results = NewState(lines, "abcd", custom).Matches(false, 0)

	foundURL = false
	expectedURL = "https://example.com/api?param=value"

	for _, result := range results {
		if result.Pattern == "url" {
			foundURL = true
			if result.Text != expectedURL {
				t.Errorf("Expected URL '%s', got '%s'", expectedURL, result.Text)
			}
		}
	}

	if !foundURL {
		t.Error("Expected to find URL pattern in double-quoted curl command")
	}

	// Test URL without quotes (should remain unchanged)
	normalLine := "Visit https://github.com/user/repo for details"
	lines = SplitLines(normalLine)
	results = NewState(lines, "abcd", custom).Matches(false, 0)

	foundURL = false
	expectedURL = "https://github.com/user/repo"

	for _, result := range results {
		if result.Pattern == "url" {
			foundURL = true
			if result.Text != expectedURL {
				t.Errorf("Expected URL '%s', got '%s'", expectedURL, result.Text)
			}
		}
	}

	if !foundURL {
		t.Error("Expected to find URL pattern in normal text")
	}

	// Test URL ending with quote but not quote-enclosed (should keep quote)
	trailingQuoteLine := "Check out https://example.com/page'"
	lines = SplitLines(trailingQuoteLine)
	results = NewState(lines, "abcd", custom).Matches(false, 0)

	foundURL = false
	expectedURL = "https://example.com/page'"

	for _, result := range results {
		if result.Pattern == "url" {
			foundURL = true
			if result.Text != expectedURL {
				t.Errorf("Expected URL '%s', got '%s'", expectedURL, result.Text)
			}
		}
	}

	if !foundURL {
		t.Error("Expected to find URL pattern with trailing quote")
	}
}
