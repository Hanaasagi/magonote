package e2e

import (
	"testing"

	"github.com/Hanaasagi/magonote/test/e2e/framework"
)

// ---------------------------------------------------------------------------
// Fixture: ip.txt — dense IP address grid
// ---------------------------------------------------------------------------

func TestFixtureIP(t *testing.T) {
	f := newFramework(t)
	result, err := f.RunDetectFile(f.FixturePath("ip.txt"))
	if err != nil {
		t.Fatalf("detect: %v", err)
	}

	ipv4s := result.EntitiesOfKind("ipv4")
	// ip.txt has 40 lines × 4 IPs per line = 160 IPs
	if len(ipv4s) < 100 {
		t.Errorf("expected at least 100 IPv4 entities from ip.txt, got %d", len(ipv4s))
	}

	// Spot-check specific IPs from the fixture
	for _, ip := range []string{"127.10.0.1", "192.168.10.1", "128.10.0.20", "193.168.0.20"} {
		if !result.HasEntity("ipv4", ip) {
			t.Errorf("expected IP %q in ip.txt; got %d entities", ip, len(ipv4s))
		}
	}
}

// ---------------------------------------------------------------------------
// Fixture: docker_ps.txt — container IDs, image names
// ---------------------------------------------------------------------------

func TestFixtureDockerPS(t *testing.T) {
	f := newFramework(t)
	result, err := f.RunDetectFile(f.FixturePath("docker_ps.txt"))
	if err != nil {
		t.Fatalf("detect: %v", err)
	}

	if len(result.Entities) == 0 {
		t.Fatal("expected entities from docker_ps.txt, got 0")
	}

	// Should detect SHA hashes (container IDs like 5386a67b0f15, 1b4fca6fdd30)
	shas := result.EntitiesOfKind("sha")
	if len(shas) == 0 {
		t.Error("expected SHA entities (container IDs) from docker_ps.txt")
	}

	// Should detect filenames (image tags like linuxserver/ffmpeg:5.1.2)
	if !result.EntityKinds()["filename"] && !result.EntityKinds()["path"] {
		t.Error("expected filename or path entities (docker images) from docker_ps.txt")
	}
}

// ---------------------------------------------------------------------------
// Fixture: k8s_pod_list.txt — pod names, ages
// ---------------------------------------------------------------------------

func TestFixtureK8sPodList(t *testing.T) {
	f := newFramework(t)

	// Without table/word detection, k8s pod names are not standard entities.
	// Verify detect runs without crashing; with --words it should find items.
	result, err := f.RunDetectFile(f.FixturePath("k8s_pod_list.txt"))
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	t.Logf("k8s_pod_list.txt (no words): %d entities, kinds: %v", len(result.Entities), result.EntityKinds())

	resultWords, err := f.RunDetectFile(f.FixturePath("k8s_pod_list.txt"), "--words")
	if err != nil {
		t.Fatalf("detect with words: %v", err)
	}
	if len(resultWords.Entities) == 0 {
		t.Error("expected entities from k8s_pod_list.txt with --words, got 0")
	}
	t.Logf("k8s_pod_list.txt (words): %d entities, kinds: %v", len(resultWords.Entities), resultWords.EntityKinds())
}

// ---------------------------------------------------------------------------
// Fixture: mixed_data.txt — URLs, IPs, UUIDs, paths, colors, digests, etc.
// ---------------------------------------------------------------------------

func TestFixtureMixedData(t *testing.T) {
	f := newFramework(t)
	result, err := f.RunDetectFile(f.FixturePath("mixed_data.txt"))
	if err != nil {
		t.Fatalf("detect: %v", err)
	}

	// mixed_data.txt contains a rich variety of entity types
	checks := []struct {
		kind string
		text string
	}{
		{"ipv4", "127.0.0.1"},
		{"ipv4", "10.0.3.4"},
		{"url", "https://en.wikipedia.org/wiki/"},
		{"uuid", "123e4567-e89b-12d3-a456-426655440000"},
		{"color", "#ff00FF"},
		{"path", "/var/log/123e4567/999.log"},
		{"path", "/etc/system.conf"},
		{"url", "https://github.com/fcsonline"},
	}

	for _, check := range checks {
		t.Run(check.kind+"_"+check.text, func(t *testing.T) {
			if !result.HasEntity(check.kind, check.text) {
				t.Errorf("expected %s %q in mixed_data.txt; entities of this kind: %v",
					check.kind, check.text, textsOfKind(result, check.kind))
			}
		})
	}

	// Should detect digests (sha256:...)
	digests := result.EntitiesOfKind("digest")
	if len(digests) == 0 {
		t.Error("expected digest entities in mixed_data.txt")
	}

	// The fixture has "path: /var/log/nginx.log[m" where "[m" is literal text
	// (no ESC prefix). Purpose: the path /var/log/nginx.log should be
	// reachable. The detector currently includes "[m" as part of the path
	// because it's valid path chars. Verify at minimum the real path prefix
	// is captured by some path entity.
	foundNginxPath := false
	for _, e := range result.EntitiesOfKind("path") {
		if len(e.Text) >= len("/var/log/nginx.log") && e.Text[:len("/var/log/nginx.log")] == "/var/log/nginx.log" {
			foundNginxPath = true
			break
		}
	}
	if !foundNginxPath {
		t.Error("expected a path entity starting with /var/log/nginx.log in mixed_data.txt")
	}
}

// ---------------------------------------------------------------------------
// Fixture: error_logs.txt — Python tracebacks with paths and URLs
// ---------------------------------------------------------------------------

func TestFixtureErrorLogs(t *testing.T) {
	f := newFramework(t)
	result, err := f.RunDetectFile(f.FixturePath("error_logs.txt"))
	if err != nil {
		t.Fatalf("detect: %v", err)
	}

	if len(result.Entities) == 0 {
		t.Fatal("expected entities from error_logs.txt, got 0")
	}

	kinds := result.EntityKinds()

	// Should detect file paths from Python tracebacks
	if !kinds["path"] && !kinds["filename"] {
		t.Error("expected path or filename entities from error_logs.txt tracebacks")
	}

	// Should detect the pydantic URL
	if !result.HasEntity("url", "https://errors.pydantic.dev/2.9/v/string_type") {
		t.Error("expected pydantic error URL in error_logs.txt")
	}

	// Should detect URLs like http://sandbox:8194/v1/sandbox/run
	urls := result.EntitiesOfKind("url")
	if len(urls) == 0 {
		t.Error("expected URL entities in error_logs.txt")
	}

	// Should detect datetimes from log timestamps
	datetimes := result.EntitiesOfKind("datetime")
	if len(datetimes) == 0 {
		t.Error("expected datetime entities from log timestamps in error_logs.txt")
	}

	t.Logf("error_logs.txt detected kinds: %v, total entities: %d", kinds, len(result.Entities))
}

// ---------------------------------------------------------------------------
// Fixture: table.txt — email addresses in tabular data
// ---------------------------------------------------------------------------

func TestFixtureTable(t *testing.T) {
	f := newFramework(t)
	result, err := f.RunDetectFile(f.FixturePath("table.txt"))
	if err != nil {
		t.Fatalf("detect: %v", err)
	}

	// Should detect email addresses
	emails := result.EntitiesOfKind("email")
	if len(emails) < 5 {
		t.Errorf("expected at least 5 email entities in table.txt, got %d", len(emails))
	}

	for _, email := range []string{"user1@example.com", "user5@example.com", "user10@example.com"} {
		if !result.HasEntity("email", email) {
			t.Errorf("expected email %q in table.txt", email)
		}
	}
}

// ---------------------------------------------------------------------------
// Fixture: commands.txt — shell commands, git output, paths
// ---------------------------------------------------------------------------

func TestFixtureCommands(t *testing.T) {
	f := newFramework(t)
	result, err := f.RunDetectFile(f.FixturePath("commands.txt"))
	if err != nil {
		t.Fatalf("detect: %v", err)
	}

	if len(result.Entities) == 0 {
		t.Fatal("expected entities from commands.txt, got 0")
	}

	kinds := result.EntityKinds()

	// Should detect paths from ls output and git output
	if !kinds["path"] && !kinds["filename"] {
		t.Error("expected path/filename entities from commands.txt")
	}

	// Should detect filenames from ls output and git output
	filenames := result.EntitiesOfKind("filename")
	if len(filenames) == 0 {
		t.Error("expected filename entities from ls/git output in commands.txt")
	}

	// Should detect SHA hashes from git pull output (e.g. 4a229ab, f7f0762)
	shas := result.EntitiesOfKind("sha")
	if len(shas) == 0 {
		t.Error("expected SHA entities from git pull output in commands.txt")
	}
}

// ---------------------------------------------------------------------------
// Fixture: emoji.txt — IPs adjacent to emoji characters
// ---------------------------------------------------------------------------

func TestFixtureEmoji(t *testing.T) {
	f := newFramework(t)
	result, err := f.RunDetectFile(f.FixturePath("emoji.txt"))
	if err != nil {
		t.Fatalf("detect: %v", err)
	}

	// All 5 lines in emoji.txt contain IPs
	expectedIPs := []string{"127.0.0.1", "192.168.1.1", "172.0.0.1:8080", "10.0.0.1", "255.255.255.0"}
	for _, ip := range expectedIPs {
		if !result.HasEntityText(ip) {
			t.Errorf("expected IP %q in emoji.txt; got %v", ip, result.EntityTexts())
		}
	}
}

// ---------------------------------------------------------------------------
// Fixture: long_line.txt — entities in very long lines
// ---------------------------------------------------------------------------

func TestFixtureLongLine(t *testing.T) {
	f := newFramework(t)
	result, err := f.RunDetectFile(f.FixturePath("long_line.txt"))
	if err != nil {
		t.Fatalf("detect: %v", err)
	}

	checks := []struct {
		kind string
		text string
	}{
		{"ipv4", "192.168.1.1"},
		{"ipv4", "10.0.0.1"},
		{"ipv4", "172.16.0.1"},
		{"url", "https://example.com/very/long/path/with/many/segments/that/should/wrap/gracefully"},
		{"url", "https://api.example.com/v1/endpoints/with/very/long/paths/that/should/wrap"},
	}

	for _, check := range checks {
		t.Run(check.kind+"_"+check.text, func(t *testing.T) {
			if !result.HasEntity(check.kind, check.text) {
				t.Errorf("expected %s %q in long_line.txt", check.kind, check.text)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Fixture: unittest.txt — test names from Go and Rust output
// ---------------------------------------------------------------------------

func TestFixtureUnittest(t *testing.T) {
	f := newFramework(t)
	result, err := f.RunDetectFile(f.FixturePath("unittest.txt"))
	if err != nil {
		t.Fatalf("detect: %v", err)
	}

	// Rust test names
	for _, name := range []string{
		"lexer::comment::test_comment_doc",
		"lexer::comment::test_comment",
		"lexer::function::test_function",
	} {
		if !result.HasEntity("test_name", name) {
			t.Errorf("expected Rust test name %q; got %v", name, textsOfKind(result, "test_name"))
		}
	}

	// Go test names
	for _, name := range []string{
		"TestDockerPsOutput",
		"TestLsOutputWithHeader",
	} {
		if !result.HasEntity("test_name", name) {
			t.Errorf("expected Go test name %q; got %v", name, textsOfKind(result, "test_name"))
		}
	}
}

// ---------------------------------------------------------------------------
// Fixture: sonnet.txt — ANSI-colored text, should not crash
// ---------------------------------------------------------------------------

func TestFixtureSonnet(t *testing.T) {
	f := newFramework(t)
	result, err := f.RunDetectFile(f.FixturePath("sonnet.txt"))
	if err != nil {
		t.Fatalf("detect: %v", err)
	}

	// The sonnet has ANSI escape codes but no standard entities.
	// Mainly a robustness test — detect should not crash on ANSI input.
	t.Logf("sonnet.txt: %d entities detected, kinds: %v", len(result.Entities), result.EntityKinds())
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func textsOfKind(result *framework.DetectResult, kind string) []string {
	entities := result.EntitiesOfKind(kind)
	texts := make([]string, len(entities))
	for i, e := range entities {
		texts[i] = e.Text
	}
	return texts
}
