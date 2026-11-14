package e2e

import (
	"testing"

	"github.com/Hanaasagi/magonote/test/e2e/framework"
)

func newFramework(t *testing.T) *framework.Framework {
	t.Helper()
	f := framework.NewFramework()
	if err := f.BuildBinary(); err != nil {
		t.Fatalf("build binary: %v", err)
	}
	return f
}

// ---------------------------------------------------------------------------
// IPv4
// ---------------------------------------------------------------------------

func TestDetectIPv4(t *testing.T) {
	f := newFramework(t)

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"single", "server at 192.168.1.1 is up", []string{"192.168.1.1"}},
		{"multiple", "from 10.0.0.1 to 172.16.0.1", []string{"10.0.0.1", "172.16.0.1"}},
		{"with port", "listen on 0.0.0.0:8080", []string{"0.0.0.0:8080"}}, // detected as ipv4_port kind
		{"loopback", "localhost 127.0.0.1", []string{"127.0.0.1"}},
		{"broadcast", "broadcast 255.255.255.255", []string{"255.255.255.255"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := f.RunDetect(tt.input)
			if err != nil {
				t.Fatalf("detect: %v", err)
			}
			for _, want := range tt.expected {
				if !result.HasEntityText(want) {
					t.Errorf("expected entity %q not found; got %v", want, result.EntityTexts())
				}
			}
		})
	}

	// Verify IP with port is classified as ipv4_port, not plain ipv4
	t.Run("port_kind", func(t *testing.T) {
		result, err := f.RunDetect("listen on 0.0.0.0:8080")
		if err != nil {
			t.Fatalf("detect: %v", err)
		}
		if !result.HasEntity("ipv4_port", "0.0.0.0:8080") {
			t.Errorf("expected kind ipv4_port; got %v", result.EntityKinds())
		}
	})
}

// ---------------------------------------------------------------------------
// IPv6
// ---------------------------------------------------------------------------

func TestDetectIPv6(t *testing.T) {
	f := newFramework(t)

	tests := []struct {
		name     string
		input    string
		expected string
		kind     string
	}{
		{"bracketed", "listen [::1]:8080", "[::1]:8080", "ipv6_port"},
		{"full", "addr 2001:db8::1 up", "2001:db8::1", "ipv6"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := f.RunDetect(tt.input)
			if err != nil {
				t.Fatalf("detect: %v", err)
			}
			if !result.HasEntity(tt.kind, tt.expected) {
				t.Errorf("expected %s %q not found; got %v", tt.kind, tt.expected, result.EntityTexts())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// URL
// ---------------------------------------------------------------------------

func TestDetectURL(t *testing.T) {
	f := newFramework(t)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"https", "visit https://example.com/path", "https://example.com/path"},
		{"http", "link http://localhost:3000/api", "http://localhost:3000/api"},
		{"with query", "go to https://example.com/search?q=test&page=1", "https://example.com/search?q=test&page=1"},
		{"ftp", "download ftp://files.example.com/data.tar.gz", "ftp://files.example.com/data.tar.gz"},
		{"git scp", "clone git@github.com:user/repo.git", "git@github.com:user/repo.git"},
		{"ssh", "connect ssh://user@host:22/path", "ssh://user@host:22/path"},
		{"long path", "see https://example.com/very/long/path/with/many/segments", "https://example.com/very/long/path/with/many/segments"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := f.RunDetect(tt.input)
			if err != nil {
				t.Fatalf("detect: %v", err)
			}
			if !result.HasEntity("url", tt.expected) {
				t.Errorf("expected URL %q not found; got %v", tt.expected, result.EntityTexts())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Email
// ---------------------------------------------------------------------------

func TestDetectEmail(t *testing.T) {
	f := newFramework(t)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "contact user@example.com for info", "user@example.com"},
		{"with plus", "send to user+tag@domain.org", "user+tag@domain.org"},
		{"subdomain", "mail admin@sub.domain.co.uk", "admin@sub.domain.co.uk"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := f.RunDetect(tt.input)
			if err != nil {
				t.Fatalf("detect: %v", err)
			}
			if !result.HasEntity("email", tt.expected) {
				t.Errorf("expected email %q not found; got %v", tt.expected, result.EntityTexts())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// File Paths
// ---------------------------------------------------------------------------

func TestDetectPath(t *testing.T) {
	f := newFramework(t)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"absolute", "edit /etc/nginx/nginx.conf", "/etc/nginx/nginx.conf"},
		{"relative dot", "see ./src/main.go", "./src/main.go"},
		{"relative dotdot", "from ../config/app.toml", "../config/app.toml"},
		{"home tilde", "config at ~/.config/magonote/config.toml", "~/.config/magonote/config.toml"},
		{"with line number", "error at /app/api/main.py:42", "/app/api/main.py:42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := f.RunDetect(tt.input)
			if err != nil {
				t.Fatalf("detect: %v", err)
			}
			if !result.HasEntityText(tt.expected) {
				t.Errorf("expected path %q not found; got %v", tt.expected, result.EntityTexts())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Filenames
// ---------------------------------------------------------------------------

func TestDetectFilename(t *testing.T) {
	f := newFramework(t)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"go file", "modified main.go today", "main.go"},
		{"python", "error in utils.py", "utils.py"},
		{"yaml", "edit config.yaml", "config.yaml"},
		{"json", "parse data.json", "data.json"},
		{"rust", "compile lib.rs", "lib.rs"},
		{"with line", "error in button.tsx:42 column 5", "button.tsx:42"},
		{"dotfile", "check .gitignore", ".gitignore"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := f.RunDetect(tt.input)
			if err != nil {
				t.Fatalf("detect: %v", err)
			}
			if !result.HasEntityText(tt.expected) {
				t.Errorf("expected filename %q not found; got %v", tt.expected, result.EntityTexts())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// UUID
// ---------------------------------------------------------------------------

func TestDetectUUID(t *testing.T) {
	f := newFramework(t)

	input := "id: 550e8400-e29b-41d4-a716-446655440000 and 123e4567-e89b-12d3-a456-426655440000"
	result, err := f.RunDetect(input)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}

	for _, uuid := range []string{
		"550e8400-e29b-41d4-a716-446655440000",
		"123e4567-e89b-12d3-a456-426655440000",
	} {
		if !result.HasEntity("uuid", uuid) {
			t.Errorf("expected UUID %q not found; got %v", uuid, result.EntityTexts())
		}
	}
}

// ---------------------------------------------------------------------------
// SHA / Git Hash
// ---------------------------------------------------------------------------

func TestDetectSHA(t *testing.T) {
	f := newFramework(t)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"full sha", "commit 1234567890abcdef1234567890abcdef12345678 done", "1234567890abcdef1234567890abcdef12345678"},
		{"short sha", "merge f79010f2 into main", "f79010f2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := f.RunDetect(tt.input)
			if err != nil {
				t.Fatalf("detect: %v", err)
			}
			if !result.HasEntity("sha", tt.expected) {
				t.Errorf("expected SHA %q not found; got %v", tt.expected, result.EntityTexts())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Docker Digest
// ---------------------------------------------------------------------------

func TestDetectDigest(t *testing.T) {
	f := newFramework(t)

	input := "image sha256:30557a29d5abc51e5f1d5b472e79b7e296f595abcf19fe6b9199dbbc809c6ff4"
	result, err := f.RunDetect(input)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if !result.HasEntity("digest", "sha256:30557a29d5abc51e5f1d5b472e79b7e296f595abcf19fe6b9199dbbc809c6ff4") {
		t.Errorf("expected digest not found; got %v", result.EntityTexts())
	}
}

// ---------------------------------------------------------------------------
// Color
// ---------------------------------------------------------------------------

func TestDetectColor(t *testing.T) {
	f := newFramework(t)

	input := "color #FF0000 and #00ff00 and #ff00FF"
	result, err := f.RunDetect(input)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	for _, c := range []string{"#FF0000", "#00ff00", "#ff00FF"} {
		if !result.HasEntity("color", c) {
			t.Errorf("expected color %q not found; got %v", c, result.EntityTexts())
		}
	}
}

// ---------------------------------------------------------------------------
// Date and Datetime
// ---------------------------------------------------------------------------

func TestDetectDateTime(t *testing.T) {
	f := newFramework(t)

	tests := []struct {
		name     string
		input    string
		expected string
		kind     string
	}{
		{"iso datetime", "at 2024-01-15T10:30:45Z", "2024-01-15T10:30:45Z", "datetime"},
		{"datetime with offset", "at 2024-01-15T10:30:45+08:00", "2024-01-15T10:30:45+08:00", "datetime"},
		{"space datetime", "at 2025-04-15 03:00:29", "2025-04-15 03:00:29", "datetime"},
		{"date only", "on 2024-01-15 happened", "2024-01-15", "date"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := f.RunDetect(tt.input)
			if err != nil {
				t.Fatalf("detect: %v", err)
			}
			if !result.HasEntity(tt.kind, tt.expected) {
				t.Errorf("expected %s %q not found; got %v", tt.kind, tt.expected, result.EntityTexts())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Hex Address
// ---------------------------------------------------------------------------

func TestDetectHexAddress(t *testing.T) {
	f := newFramework(t)

	input := "pointer at 0x7fff5fbff8c0"
	result, err := f.RunDetect(input)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if !result.HasEntity("hex_address", "0x7fff5fbff8c0") {
		t.Errorf("expected hex address not found; got %v", result.EntityTexts())
	}
}

// ---------------------------------------------------------------------------
// MAC Address
// ---------------------------------------------------------------------------

func TestDetectMAC(t *testing.T) {
	f := newFramework(t)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"colon separated", "hw addr 00:1A:2B:3C:4D:5E", "00:1A:2B:3C:4D:5E"},
		{"dash separated", "mac AA-BB-CC-DD-EE-FF", "AA-BB-CC-DD-EE-FF"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := f.RunDetect(tt.input)
			if err != nil {
				t.Fatalf("detect: %v", err)
			}
			if !result.HasEntity("mac", tt.expected) {
				t.Errorf("expected MAC %q not found; got %v", tt.expected, result.EntityTexts())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test Names (Go / Rust)
// ---------------------------------------------------------------------------

func TestDetectTestNames(t *testing.T) {
	f := newFramework(t)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"go pass", "--- PASS: TestDockerPsOutput (0.00s)", "TestDockerPsOutput"},
		{"go fail", "--- FAIL: TestBrokenThing (0.12s)", "TestBrokenThing"},
		{"rust ok", "test lexer::function::test_function ... ok", "lexer::function::test_function"},
		{"rust fail", "test lexer::comment::test_comment ... FAILED", "lexer::comment::test_comment"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := f.RunDetect(tt.input)
			if err != nil {
				t.Fatalf("detect: %v", err)
			}
			if !result.HasEntity("test_name", tt.expected) {
				t.Errorf("expected test name %q not found; got %v", tt.expected, result.EntityTexts())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Git diff paths
// ---------------------------------------------------------------------------

func TestDetectGitDiffPaths(t *testing.T) {
	f := newFramework(t)

	input := `diff --git a/internal/state.go b/internal/state.go
--- a/internal/state.go
+++ b/internal/state.go`

	result, err := f.RunDetect(input)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if !result.HasEntityText("internal/state.go") {
		t.Errorf("expected diff path not found; got %v", result.EntityTexts())
	}
}

// ---------------------------------------------------------------------------
// Custom regex patterns
// ---------------------------------------------------------------------------

func TestDetectCustomRegex(t *testing.T) {
	f := newFramework(t)

	input := "order ORD-12345 and ORD-67890 processed"
	result, err := f.RunDetect(input, "-x", `ORD-\d+`)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}

	for _, want := range []string{"ORD-12345", "ORD-67890"} {
		if !result.HasEntity("custom", want) {
			t.Errorf("expected custom match %q not found; got %v", want, result.EntityTexts())
		}
	}
}

// ---------------------------------------------------------------------------
// Word detection
// ---------------------------------------------------------------------------

func TestDetectWords(t *testing.T) {
	f := newFramework(t)

	input := "hello world"
	result, err := f.RunDetect(input, "--words")
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(result.Entities) < 2 {
		t.Errorf("expected at least 2 word entities, got %d: %v", len(result.Entities), result.EntityTexts())
	}
	// All entities should be of kind "word"
	for _, e := range result.Entities {
		if e.Kind != "word" {
			t.Errorf("expected kind 'word', got %q for %q", e.Kind, e.Text)
		}
	}
	if !result.HasEntity("word", "hello") {
		t.Errorf("expected word 'hello'; got %v", result.EntityTexts())
	}
	if !result.HasEntity("word", "world") {
		t.Errorf("expected word 'world'; got %v", result.EntityTexts())
	}
}

// ---------------------------------------------------------------------------
// Mixed content — multiple entity types in one input
// ---------------------------------------------------------------------------

func TestDetectMixedEntities(t *testing.T) {
	f := newFramework(t)

	input := `server 192.168.1.1 running
visit https://example.com
email admin@test.com
file /var/log/app.log
uuid 550e8400-e29b-41d4-a716-446655440000
color #FF0000`

	result, err := f.RunDetect(input)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}

	kinds := result.EntityKinds()
	requiredKinds := []string{"ipv4", "url", "email", "path", "uuid", "color"}
	for _, k := range requiredKinds {
		if !kinds[k] {
			t.Errorf("expected kind %q among detected kinds %v", k, kinds)
		}
	}
}

// ---------------------------------------------------------------------------
// Empty and edge cases
// ---------------------------------------------------------------------------

func TestDetectEmpty(t *testing.T) {
	f := newFramework(t)

	result, err := f.RunDetect("")
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(result.Entities) != 0 {
		t.Errorf("expected 0 entities for empty input, got %d", len(result.Entities))
	}
}

func TestDetectNoMatches(t *testing.T) {
	f := newFramework(t)

	result, err := f.RunDetect("just plain text with no patterns")
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if len(result.Entities) != 0 {
		t.Errorf("expected 0 entities, got %d: %v", len(result.Entities), result.EntityTexts())
	}
}

func TestDetectUnicodeContent(t *testing.T) {
	f := newFramework(t)

	input := "中文内容 192.168.1.1 日本語テスト https://example.com"
	result, err := f.RunDetect(input)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if !result.HasEntity("ipv4", "192.168.1.1") {
		t.Errorf("expected IPv4 in unicode context; got %v", result.EntityTexts())
	}
	if !result.HasEntity("url", "https://example.com") {
		t.Errorf("expected URL in unicode context; got %v", result.EntityTexts())
	}
}

func TestDetectEmojiContent(t *testing.T) {
	f := newFramework(t)

	input := "🚀 server at 172.0.0.1:8080 is running 💻"
	result, err := f.RunDetect(input)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if !result.HasEntityText("172.0.0.1:8080") {
		t.Errorf("expected IP with port near emojis; got %v", result.EntityTexts())
	}
}

func TestDetectANSIEscapes(t *testing.T) {
	f := newFramework(t)

	input := "normal \x1b[31mred 192.168.1.1\x1b[0m normal"
	result, err := f.RunDetect(input)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if !result.HasEntity("ipv4", "192.168.1.1") {
		t.Errorf("expected IP through ANSI escapes; got %v", result.EntityTexts())
	}
}

// ---------------------------------------------------------------------------
// Negative tests — invalid patterns must NOT be detected
// ---------------------------------------------------------------------------

func TestDetectRejectsInvalidIPv4(t *testing.T) {
	f := newFramework(t)

	tests := []struct {
		name  string
		input string
	}{
		{"out of range octets", "address 999.999.999.999 here"},
		{"five octets", "number 1.2.3.4.5 here"},
		{"too short", "version 1.2 here"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := f.RunDetect(tt.input)
			if err != nil {
				t.Fatalf("detect: %v", err)
			}
			ipv4s := result.EntitiesOfKind("ipv4")
			ipv4ports := result.EntitiesOfKind("ipv4_port")
			if len(ipv4s)+len(ipv4ports) > 0 {
				var texts []string
				for _, e := range append(ipv4s, ipv4ports...) {
					texts = append(texts, e.Text)
				}
				t.Errorf("should not detect IPv4 in %q, but got: %v", tt.input, texts)
			}
		})
	}
}

func TestDetectRejectsInvalidDate(t *testing.T) {
	f := newFramework(t)

	tests := []struct {
		name  string
		input string
	}{
		{"month 13", "date is 2024-13-01 invalid"},
		{"day 32", "date is 2024-01-32 invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := f.RunDetect(tt.input)
			if err != nil {
				t.Fatalf("detect: %v", err)
			}
			dates := result.EntitiesOfKind("date")
			datetimes := result.EntitiesOfKind("datetime")
			if len(dates)+len(datetimes) > 0 {
				var texts []string
				for _, e := range append(dates, datetimes...) {
					texts = append(texts, e.Text)
				}
				t.Errorf("should not detect date in %q, but got: %v", tt.input, texts)
			}
		})
	}
}

func TestDetectURLTrimsTrailingPunctuation(t *testing.T) {
	f := newFramework(t)

	input := "see https://example.com/path. for details"
	result, err := f.RunDetect(input)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}

	// The trailing '.' should be stripped — the detected URL should be clean
	if result.HasEntity("url", "https://example.com/path.") {
		t.Error("URL should not include trailing period")
	}
	if !result.HasEntity("url", "https://example.com/path") {
		t.Errorf("expected trimmed URL; got %v", result.EntityTexts())
	}
}

func TestDetectRejectsMixedCaseUUID(t *testing.T) {
	f := newFramework(t)

	// UUIDs with mixed case (both upper and lower hex) should be rejected
	input := "id 550e8400-E29B-41d4-a716-446655440000"
	result, err := f.RunDetect(input)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if result.HasEntity("uuid", "550e8400-E29B-41d4-a716-446655440000") {
		t.Error("mixed-case UUID should be rejected by validator")
	}
}

// ---------------------------------------------------------------------------
// Span accuracy
// ---------------------------------------------------------------------------

func TestDetectSpanColumns(t *testing.T) {
	f := newFramework(t)

	input := "中 https://example.com"
	result, err := f.RunDetect(input)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	urls := result.EntitiesOfKind("url")
	if len(urls) != 1 {
		t.Fatalf("expected 1 URL, got %d", len(urls))
	}
	// "中" is 2 display columns wide, then space is 1, so URL starts at column 3
	if urls[0].Span.StartCol != 3 {
		t.Errorf("expected URL start_col=3 (after wide char), got %d", urls[0].Span.StartCol)
	}
}
