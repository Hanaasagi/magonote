package table

import (
	"os"
	"strings"
	"testing"

	"github.com/Hanaasagi/magonote/pkg/textdetection/screen"
	"github.com/Hanaasagi/magonote/pkg/textdetection/token"
)

func detectText(input string) []Table {
	s := screen.Parse(input)
	chunks := make([][]token.Chunk, len(s.Lines))
	for i := range s.Lines {
		chunks[i] = token.Chunks(s, i)
	}
	return Detect(s, chunks, nil, Options{})
}

func fixtureText(t *testing.T, name string, start, end int) string {
	t.Helper()
	data, err := os.ReadFile("../testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	return strings.Join(lines[start:end], "\n")
}

func TestDockerPSProjection(t *testing.T) {
	tables := detectText(fixtureText(t, "docker_ps.txt", 0, 13))
	if len(tables) != 1 {
		t.Fatalf("got %d tables", len(tables))
	}
	table := tables[0]
	if table.HeaderLine != 0 || len(table.Rows) != 13 || len(table.Columns) != 7 {
		t.Fatalf("table shape: header=%d rows=%d columns=%d", table.HeaderLine, len(table.Rows), len(table.Columns))
	}
	wantHeaders := []string{"CONTAINER ID", "IMAGE", "COMMAND", "CREATED", "STATUS", "PORTS", "NAMES"}
	for i, want := range wantHeaders {
		if table.Columns[i].Name != want {
			t.Errorf("column %d name = %q, want %q", i, table.Columns[i].Name, want)
		}
	}
	if got := table.Rows[1].Cells[3].Text; got != "13 months ago" {
		t.Errorf("CREATED cell = %q", got)
	}
	if got := table.Rows[1].Cells[4].Text; got != "Exited (255) 13 months ago" {
		t.Errorf("STATUS cell = %q", got)
	}
}

func TestHeaderlessDockerKeepsCompoundCells(t *testing.T) {
	tables := detectText(fixtureText(t, "commands.txt", 1, 3))
	if len(tables) != 1 || len(tables[0].Columns) != 7 || len(tables[0].Rows) != 2 {
		t.Fatalf("unexpected tables: %#v", tables)
	}
	if tables[0].HeaderLine != -1 {
		t.Errorf("HeaderLine = %d, want -1", tables[0].HeaderLine)
	}
	if got := tables[0].Rows[0].Cells[3].Text; got != "13 months ago" {
		t.Errorf("CREATED = %q", got)
	}
	if got := tables[0].Rows[0].Cells[4].Text; got != "Up 2 days" {
		t.Errorf("STATUS = %q", got)
	}
}

func TestEzaHeaderSupportsSingleColumnGrooves(t *testing.T) {
	tables := detectText(fixtureText(t, "commands.txt", 5, 22))
	if len(tables) != 1 || len(tables[0].Columns) != 5 || len(tables[0].Rows) != 17 {
		t.Fatalf("unexpected eza shape: %#v", tables)
	}
	table := tables[0]
	wantHeaders := []string{"Permissions", "Size", "User", "Date Modified", "Name"}
	for i, want := range wantHeaders {
		if table.Columns[i].Name != want {
			t.Errorf("column %d = %q, want %q", i, table.Columns[i].Name, want)
		}
	}
	if got := table.Rows[1].Cells[3].Text; got != "2025-06-17 22:24" {
		t.Errorf("date cell = %q", got)
	}
	if table.Columns[1].Align != "right" {
		t.Errorf("Size alignment = %q, want right", table.Columns[1].Align)
	}
}

func TestTableFixtureSkipsSeparatorAndKeepsCompoundName(t *testing.T) {
	tables := detectText(fixtureText(t, "table.txt", 0, 12))
	if len(tables) != 1 || len(tables[0].Columns) != 4 || len(tables[0].Rows) != 11 {
		t.Fatalf("unexpected table shape: %#v", tables)
	}
	row := tables[0].Rows[1]
	if row.Line != 2 || row.Cells[0].Text != "Sample User 1" {
		t.Fatalf("first data row = %#v", row)
	}
}

func TestIPFixtureSplitsAtBlankLine(t *testing.T) {
	tables := detectText(fixtureText(t, "ip.txt", 0, 41))
	if len(tables) != 2 {
		t.Fatalf("got %d tables, want 2", len(tables))
	}
	for i, table := range tables {
		if len(table.Rows) != 20 || len(table.Columns) != 4 || table.HeaderLine != -1 {
			t.Errorf("table %d shape: rows=%d columns=%d header=%d", i, len(table.Rows), len(table.Columns), table.HeaderLine)
		}
	}
	if tables[1].StartLine != 21 {
		t.Errorf("second table starts at %d, want 21", tables[1].StartLine)
	}
}

func TestKubernetesCellsRemainIntact(t *testing.T) {
	tables := detectText(fixtureText(t, "k8s_pod_list.txt", 0, 58))
	if len(tables) != 1 || len(tables[0].Columns) != 5 || len(tables[0].Rows) != 58 {
		t.Fatalf("unexpected k8s shape: %#v", tables)
	}
	foundReady, foundAge := false, false
	for _, row := range tables[0].Rows {
		for _, cell := range row.Cells {
			foundReady = foundReady || cell.Text == "1/1"
			foundAge = foundAge || cell.Text == "8d"
		}
	}
	if !foundReady || !foundAge {
		t.Errorf("missing short cells: READY=%v AGE=%v", foundReady, foundAge)
	}
}

func TestExplicitMarkdownTable(t *testing.T) {
	input := "| Name | Age |\n| :--- | ---: |\n| Alice | 8 |\n| Bob | 22 |"
	tables := detectText(input)
	if len(tables) != 1 || len(tables[0].Columns) != 2 || len(tables[0].Rows) != 3 {
		t.Fatalf("unexpected markdown table: %#v", tables)
	}
	table := tables[0]
	if table.HeaderLine != 0 || table.Columns[0].Name != "Name" || table.Columns[1].Name != "Age" {
		t.Errorf("markdown header = %#v", table)
	}
	if got := table.Rows[1].Cells[0].Text; got != "Alice" {
		t.Errorf("first cell = %q", got)
	}
	if got := table.Rows[1].Cells[1].Text; got != "8" {
		t.Errorf("age cell = %q", got)
	}
}

func TestExplicitBoxTable(t *testing.T) {
	input := "┌───┬───┐\n│ A │ B │\n├───┼───┤\n│ x │ y │\n└───┴───┘"
	tables := detectText(input)
	if len(tables) != 1 || len(tables[0].Columns) != 2 || len(tables[0].Rows) != 2 {
		t.Fatalf("unexpected box table: %#v", tables)
	}
	if tables[0].HeaderLine != 1 || tables[0].StartLine != 1 || tables[0].EndLine != 3 {
		t.Errorf("box table boundaries = %#v", tables[0])
	}
}

func TestExplicitTableWithoutSpaces(t *testing.T) {
	tables := detectText("|foo|bar|\n|one|two|")
	if len(tables) != 1 || len(tables[0].Columns) != 2 || len(tables[0].Rows) != 2 {
		t.Fatalf("unexpected compact table: %#v", tables)
	}
	if got := tables[0].Rows[0].Cells[0].Text; got != "foo" {
		t.Errorf("cell = %q", got)
	}
}

func TestKeepsValidTableBeforeLayoutBreak(t *testing.T) {
	input := "A  B  C\nx  y  z\naaaa bbbb\ncccc dddd"
	tables := detectText(input)
	if len(tables) != 1 || len(tables[0].Rows) != 2 || len(tables[0].Columns) != 3 {
		t.Fatalf("layout break discarded valid prefix: %#v", tables)
	}
	if tables[0].StartLine != 0 || tables[0].EndLine != 1 {
		t.Errorf("table boundaries = %d..%d", tables[0].StartLine, tables[0].EndLine)
	}
}

func TestFindsValidTableAfterNoisePrefix(t *testing.T) {
	input := "noise  xx\nA  B  C\nx  y  z\nu  v  w"
	tables := detectText(input)
	if len(tables) != 1 || tables[0].StartLine != 1 || len(tables[0].Rows) != 3 || len(tables[0].Columns) != 3 {
		t.Fatalf("valid suffix was discarded: %#v", tables)
	}
}

func TestFindsTablesOnBothSidesOfLayoutBreak(t *testing.T) {
	input := "A  B  C\nx  y  z\naaaa bbbb\nD  E  F\nq  r  s\nu  v  w"
	tables := detectText(input)
	if len(tables) != 2 || tables[0].StartLine != 0 || tables[1].StartLine != 3 {
		t.Fatalf("tables around layout break = %#v", tables)
	}
}

func TestRejectsHeaderlessBlockWithUnstableChunkCounts(t *testing.T) {
	input := " * branch            master     -> FETCH_HEAD\n   4a229ab..f7f0762  master     -> origin/master\nUpdating 4a229ab..f7f0762"
	if got := detectText(input); len(got) != 0 {
		t.Fatalf("git log prose became a table: %#v", got)
	}
}

func TestConfidenceRoundsFloatingPointNoise(t *testing.T) {
	if got := normalizeConfidence(0.8800000000000001); got != 0.88 {
		t.Fatalf("normalized confidence = %.17g, want 0.88", got)
	}
}

func TestRejectsNonTables(t *testing.T) {
	if got := detectText(fixtureText(t, "commands.txt", 24, 27)); len(got) != 0 {
		t.Errorf("go.sum detected as table: %#v", got)
	}
	if got := detectText("a b\nc d"); len(got) != 0 {
		t.Errorf("single-space prose detected as table: %#v", got)
	}
}
