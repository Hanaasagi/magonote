package textdetection

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/Hanaasagi/magonote/pkg/textdetection/entity"
	"github.com/Hanaasagi/magonote/pkg/textdetection/profile"
	"github.com/Hanaasagi/magonote/pkg/textdetection/table"
)

func summaries(entities []entity.Entity) []string {
	result := make([]string, len(entities))
	for i, item := range entities {
		result[i] = string(item.Kind) + ":" + item.Text
	}
	return result
}

func TestDetectSemanticEntities(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"url and email", "curl 'https://example.com/api?x=1' user@example.com", []string{"url:https://example.com/api?x=1", "email:user@example.com"}},
		{"scp URL", "git@github.com:user/repo.git", []string{"url:git@github.com:user/repo.git"}},
		{"markdown URL", "[link](https://github.io?foo=bar)", []string{"url:https://github.io?foo=bar"}},
		{"network", "127.0.0.1:8080 192.168.1.1 [2001:db8::1]:443 fe80::1%eth0", []string{"ipv4_port:127.0.0.1:8080", "ipv4:192.168.1.1", "ipv6_port:[2001:db8::1]:443", "ipv6:fe80::1%eth0"}},
		{"dates", "12/01/2023 2023-12-01T10:30:45Z 2023-12-01 14:30:25", []string{"date:12/01/2023", "datetime:2023-12-01T10:30:45Z", "datetime:2023-12-01 14:30:25"}},
		{"paths", "/tmp/foo/bar_lol, ~/Documents/file.txt $HOME/bin/script @home/folder/item", []string{"path:/tmp/foo/bar_lol", "path:~/Documents/file.txt", "path:$HOME/bin/script", "path:@home/folder/item"}},
		{"filename with location", "main.go:12:4", []string{"filename:main.go:12:4"}},
		{"digest and SHA", "sha256:30557a29d5abc51e5f1d5b472e79b7e296f595abcf19fe6b9199dbbc809c6ff4 fd70b5695", []string{"digest:sha256:30557a29d5abc51e5f1d5b472e79b7e296f595abcf19fe6b9199dbbc809c6ff4", "sha:fd70b5695"}},
		{"diff paths", "diff --git a/src/main.go b/src/main.go", []string{"path:src/main.go", "path:src/main.go"}},
		{"test names", "test pkg/foo.TestBar ... ok\n--- PASS: TestFoo", []string{"test_name:pkg/foo.TestBar", "test_name:TestFoo"}},
		{"other semantic kinds", "550e8400-e29b-41d4-a716-446655440000 #FF0000 0x0 QmW2HvDCgqCLJtGxVPZDMWJ5tE2PrsaS3s4VqgdgMqKBNK", []string{"uuid:550e8400-e29b-41d4-a716-446655440000", "color:#FF0000", "hex_address:0x0", "ipfs:QmW2HvDCgqCLJtGxVPZDMWJ5tE2PrsaS3s4VqgdgMqKBNK"}},
		{"false positives", "ERROR:foo 10:30:45 Running:0 999.1.1.1 1/1 3306/tcp Mozilla/5.0 text/html 1234567 webapp-editor-7fdbfbf4b-k68b7", []string{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Detect(test.input, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if got := summaries(result.Entities); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("entities = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestDetectMarkdownDestinations(t *testing.T) {
	tests := []struct{ name, input, want string }{
		{"URL title", `[x](https://example.com "title")`, "url:https://example.com"},
		{"path title", `[x](docs/guide.md "title")`, "path:docs/guide.md"},
		{"balanced parentheses", `[x](relative_(one).md)`, "path:relative_(one).md"},
		{"escaped parentheses", `[x](relative_\(one\).md)`, "path:relative_(one).md"},
		{"angle destination", `[x](<https://example.com/a> "title")`, "url:https://example.com/a"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, _ := Detect(test.input, Options{})
			if got := summaries(result.Entities); !reflect.DeepEqual(got, []string{test.want}) {
				t.Fatalf("entities = %#v, want %s", got, test.want)
			}
		})
	}
}

func TestDetectAdjacentMarkdownLinksRemainIndependent(t *testing.T) {
	result, _ := Detect("[one](https://example.com/a),[two](https://example.org/b)", Options{})
	want := []string{"url:https://example.com/a", "url:https://example.org/b"}
	if got := summaries(result.Entities); !reflect.DeepEqual(got, want) {
		t.Fatalf("adjacent Markdown links = %#v, want %#v", got, want)
	}
}

func TestDetectMapsBytesToDisplayColumns(t *testing.T) {
	result, _ := Detect("中 https://example.com", Options{})
	if len(result.Entities) != 1 {
		t.Fatalf("entities = %#v", result.Entities)
	}
	span := result.Entities[0].Span
	if span.StartCol != 3 || span.EndCol != 22 {
		t.Fatalf("URL span = %#v, want [3,22)", span)
	}
}

func TestDetectFilenameLocationMetadata(t *testing.T) {
	result, _ := Detect("main.go:12:4", Options{})
	entity := result.Entities[0]
	if entity.Meta["line"] != "12" || entity.Meta["col"] != "4" {
		t.Fatalf("location metadata = %#v", entity.Meta)
	}
	if entity.Location == nil || entity.Location.Path != "main.go" || entity.Location.Line != 12 || entity.Location.Column != 4 {
		t.Fatalf("structured location = %#v", entity.Location)
	}
}

func TestDetectTracebackFileLocation(t *testing.T) {
	result, _ := Detect(`File "/tmp/app.py", line 20`, Options{})
	if len(result.Entities) != 1 || result.Entities[0].Location == nil {
		t.Fatalf("traceback entities = %#v", result.Entities)
	}
	location := result.Entities[0].Location
	if location.Path != "/tmp/app.py" || location.Line != 20 || location.Column != 0 {
		t.Fatalf("traceback location = %#v", location)
	}
}

func TestDetectCustomCaptureRulesAndInvalidPattern(t *testing.T) {
	result, err := Detect("CUSTOM-52463 ISSUE-123-ABC", Options{CustomPatterns: []string{`(?P<match>CUSTOM)-([0-9]+)`, `ISSUE-([0-9]+)-([A-Z]+)`, `[`}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"custom:CUSTOM", "custom:123", "custom:ABC"}
	if got := summaries(result.Entities); !reflect.DeepEqual(got, want) {
		t.Fatalf("custom entities = %#v, want %#v", got, want)
	}
}

func TestDetectCustomFallsBackWhenNamedCaptureIsEmpty(t *testing.T) {
	result, _ := Detect("ABC PLAIN-123", Options{CustomPatterns: []string{`^(?P<match>)(ABC)`, `PLAIN-[0-9]+`}})
	want := []string{"custom:ABC", "custom:PLAIN-123"}
	if got := summaries(result.Entities); !reflect.DeepEqual(got, want) {
		t.Fatalf("custom fallback = %#v, want %#v", got, want)
	}
}

func TestDetectCustomUsesSpanTextForGrapheme(t *testing.T) {
	result, _ := Detect("éx", Options{CustomPatterns: []string{"e"}})
	if len(result.Entities) != 1 || result.Entities[0].Text != "é" || result.Entities[0].Span.StartCol != 0 || result.Entities[0].Span.EndCol != 1 {
		t.Fatalf("grapheme candidate = %#v", result.Entities)
	}
}

func TestDetectExclusions(t *testing.T) {
	result, _ := Detect("https://example.com https://example.org", Options{Exclusions: []Exclusion{{Type: "text", Pattern: "example.com"}}})
	want := []string{"url:https://example.org"}
	if got := summaries(result.Entities); !reflect.DeepEqual(got, want) {
		t.Fatalf("excluded entities = %#v", got)
	}
}

func TestDetectStyleAndHyperlink(t *testing.T) {
	result, _ := Detect("\x1b[31;1merror warning\x1b[0m", Options{Style: true})
	want := []string{"styled:error", "styled:warning"}
	if got := summaries(result.Entities); !reflect.DeepEqual(got, want) {
		t.Fatalf("styled entities = %#v", got)
	}

	result, _ = Detect("\x1b]8;;https://example.com\adocs\x1b]8;;\a", Options{Style: true})
	if len(result.Entities) != 1 || result.Entities[0].Kind != entity.KindURL || result.Entities[0].Text != "https://example.com" || result.Entities[0].Span.EndCol != 4 {
		t.Fatalf("hyperlink entities = %#v", result.Entities)
	}
}

func TestDetectTableCells(t *testing.T) {
	data, err := os.ReadFile("testdata/docker_ps.txt")
	if err != nil {
		t.Fatal(err)
	}
	result, _ := Detect(string(data), Options{Table: &table.Options{}})
	found := false
	for _, item := range result.Entities {
		if item.Kind == entity.KindCell && item.Text == "13 months ago" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("compound table cell missing: %#v", summaries(result.Entities))
	}
}

func TestDetectWords(t *testing.T) {
	result, _ := Detect("hello foo-bar", Options{Words: true})
	want := []string{"word:hello", "word:foo-bar"}
	if got := summaries(result.Entities); !reflect.DeepEqual(got, want) {
		t.Fatalf("words = %#v", got)
	}
}

func TestDetectKubectlProfile(t *testing.T) {
	data, err := os.ReadFile("testdata/k8s_pod_list.txt")
	if err != nil {
		t.Fatal(err)
	}
	result, _ := Detect("$ kubectl get pods\n"+string(data), Options{Table: &table.Options{}, Profiles: &profile.Options{ExcludePrompt: true}, CustomPatterns: []string{`\$`}})
	if len(result.Prompts) != 1 || len(result.Tables) != 1 {
		t.Fatalf("prompts/tables = %#v/%#v", result.Prompts, result.Tables)
	}
	columns := result.Tables[0].Columns
	if columns[0].Semantic != "k8s.pod" || columns[1].Weight != 0.4 {
		t.Fatalf("kubectl columns = %#v", columns)
	}
	for _, item := range result.Entities {
		if item.Text == "$" {
			t.Fatalf("prompt prefix was not excluded: %#v", item)
		}
	}
}

func TestDetectDockerProfiles(t *testing.T) {
	data, err := os.ReadFile("testdata/docker_ps.txt")
	if err != nil {
		t.Fatal(err)
	}
	result, _ := Detect("$ docker ps\n"+string(data), Options{Table: &table.Options{}, Profiles: &profile.Options{ExcludePrompt: true}})
	if len(result.Tables) != 1 || result.Tables[0].Columns[0].Semantic != "docker.container" {
		t.Fatalf("docker tables = %#v", result.Tables)
	}
	if got := result.Tables[0].Columns[3].Weight; got != 0.4 {
		t.Fatalf("CREATED weight = %v", got)
	}

	input := "$ docker images\nREPOSITORY  TAG    IMAGE ID  CREATED      SIZE\nnginx       latest abc123      2 days ago   100MB\nredis       7      def456      3 days ago   120MB"
	result, _ = Detect(input, Options{Table: &table.Options{}, Profiles: &profile.Options{ExcludePrompt: true}})
	if len(result.Tables) != 1 || result.Tables[0].Columns[0].Semantic != "docker.repository" || result.Tables[0].Columns[2].Semantic != "docker.image_id" {
		t.Fatalf("docker images columns = %#v", result.Tables)
	}
}

func TestDetectGitProfiles(t *testing.T) {
	long := "$ git status\nOn branch main\nChanges to be committed:\n\tmodified:   internal/state.go\nUntracked files:\n\tnew_file.txt"
	result, _ := Detect(long, Options{Profiles: &profile.Options{ExcludePrompt: true}})
	for _, path := range []string{"internal/state.go", "new_file.txt"} {
		found := false
		for _, item := range result.Entities {
			if item.Kind == entity.KindPath && item.Text == path && item.Meta["source"] == "git" {
				found = true
			}
		}
		if !found {
			t.Errorf("git path %q missing: %#v", path, summaries(result.Entities))
		}
	}

	short := "$ git status -s\n M internal/state.go\n?? new_file.txt"
	result, _ = Detect(short, Options{Profiles: &profile.Options{ExcludePrompt: true}})
	if got := summaries(result.Entities); !containsSummary(got, "path:internal/state.go") || !containsSummary(got, "path:new_file.txt") {
		t.Fatalf("short status = %#v", got)
	}

	result, _ = Detect("$ git branch\n* main\n  dev", Options{Profiles: &profile.Options{ExcludePrompt: true}})
	if got := summaries(result.Entities); !reflect.DeepEqual(got, []string{"branch:main", "branch:dev"}) {
		t.Fatalf("branches = %#v", got)
	}

	for _, command := range []string{"git status -sb", "git -C repo status -sb"} {
		result, _ = Detect("$ "+command+"\n?? newdir/", Options{Profiles: &profile.Options{ExcludePrompt: true}})
		if got := summaries(result.Entities); !containsSummary(got, "path:newdir/") {
			t.Errorf("%s untracked directory = %#v", command, got)
		}
	}
}

func containsSummary(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func TestDetectLSProfiles(t *testing.T) {
	long := "$ ls -l\n-rw-r--r-- 1 user group 12 Jan 1 10:00 file one.txt\n-rw-r--r-- 1 user group 13 Jan 2 10:00 other.go"
	result, _ := Detect(long, Options{Profiles: &profile.Options{ExcludePrompt: true}})
	for _, path := range []string{"file one.txt", "other.go"} {
		if !containsSummary(summaries(result.Entities), "path:"+path) {
			t.Errorf("long ls path %q missing: %#v", path, summaries(result.Entities))
		}
	}

	result, _ = Detect("$ ls\nfoo.txt bar.go", Options{Profiles: &profile.Options{ExcludePrompt: true}})
	if got := summaries(result.Entities); !containsSummary(got, "filename:foo.txt") || !containsSummary(got, "filename:bar.go") {
		t.Fatalf("short ls = %#v", got)
	}

	result, _ = Detect("$ ls -l\nlrwxr-xr-x 1 user group 4 Jan 1 10:00 link -> target.txt", Options{Profiles: &profile.Options{ExcludePrompt: true}})
	if got := summaries(result.Entities); !containsSummary(got, "path:link") || containsSummary(got, "path:link -> target.txt") {
		t.Fatalf("symlink selection = %#v", got)
	}
}

func TestResultJSONControlsTimings(t *testing.T) {
	result, _ := Detect("https://example.com", Options{})
	plain, err := result.JSON(false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain), "timings") {
		t.Fatalf("timings leaked into stable JSON: %s", plain)
	}
	var decoded struct {
		Entities []entity.Entity `json:"entities"`
	}
	if err := json.Unmarshal(plain, &decoded); err != nil || len(decoded.Entities) != 1 {
		t.Fatalf("JSON = %s, err=%v", plain, err)
	}
	withTimings, _ := result.JSON(true)
	if !strings.Contains(string(withTimings), "timings") {
		t.Fatalf("timings missing: %s", withTimings)
	}
}
