package textdetection

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Hanaasagi/magonote/pkg/textdetection/profile"
	"github.com/Hanaasagi/magonote/pkg/textdetection/table"
	"github.com/mattn/go-runewidth"
)

var updateGolden = flag.Bool("update", false, "update detection golden files")

func TestGolden(t *testing.T) {
	previousWidth := runewidth.DefaultCondition.EastAsianWidth
	runewidth.DefaultCondition.EastAsianWidth = false
	t.Cleanup(func() { runewidth.DefaultCondition.EastAsianWidth = previousWidth })

	withTables := Options{Table: &table.Options{}}
	tests := []struct {
		name string
		opts Options
	}{
		{name: "false_positives", opts: Options{}},
		{name: "docker_ps", opts: withTables},
		{name: "table", opts: withTables},
		{name: "markdown_table", opts: withTables},
		{name: "cjk_table", opts: withTables},
		{name: "colored_ls", opts: Options{Style: true}},
		{name: "commands", opts: Options{Table: &table.Options{}, Profiles: &profile.Options{ExcludePrompt: true}}},
		{name: "prompts", opts: Options{Profiles: &profile.Options{ExcludePrompt: true}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input, err := os.ReadFile("testdata/" + test.name + ".txt")
			if err != nil {
				t.Fatal(err)
			}
			result, err := Detect(string(input), test.opts)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := result.JSON(false)
			if err != nil {
				t.Fatal(err)
			}
			var pretty bytes.Buffer
			if err := json.Indent(&pretty, raw, "", "  "); err != nil {
				t.Fatal(err)
			}
			pretty.WriteByte('\n')
			path := "testdata/" + test.name + ".golden.json"
			if *updateGolden {
				if err := os.WriteFile(path, pretty.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(pretty.Bytes(), want) {
				t.Fatalf("golden mismatch for %s (run go test ./pkg/textdetection -run TestGolden -update)\n%s", path, firstGoldenDifference(want, pretty.Bytes()))
			}
		})
	}
}

func firstGoldenDifference(want, got []byte) string {
	wantLines := strings.Split(string(want), "\n")
	gotLines := strings.Split(string(got), "\n")
	for i := 0; i < min(len(wantLines), len(gotLines)); i++ {
		if wantLines[i] != gotLines[i] {
			return fmt.Sprintf("first difference at line %d:\nexpected: %s\nactual:   %s", i+1, wantLines[i], gotLines[i])
		}
	}
	return fmt.Sprintf("only line counts differ: expected %d, actual %d", len(wantLines), len(gotLines))
}
