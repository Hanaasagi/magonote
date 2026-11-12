package table

import (
	"math"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Hanaasagi/magonote/pkg/textdetection/screen"
	"github.com/Hanaasagi/magonote/pkg/textdetection/token"
)

// Detect finds whitespace-projection tables. Explicit delimiter tables are
// handled by the same entry point in delimiter.go.
func Detect(s *screen.Screen, chunks [][]token.Chunk, skip map[int]bool, opts Options) []Table {
	opts = normalizeOptions(opts)
	tables := detectExplicit(s, skip, opts)
	var block []int
	flush := func() {
		tables = append(tables, projectBlocks(s, chunks, block, opts)...)
		block = nil
	}

	for line := range s.Lines {
		if skip[line] {
			flush()
			continue
		}
		plain := s.Lines[line].Plain
		if strings.TrimSpace(plain) == "" {
			flush()
			continue
		}
		if token.IsSeparatorLine(plain) {
			continue
		}
		if line >= len(chunks) || len(chunks[line]) < 2 || hasExplicitDelimiter(s, line) {
			flush()
			continue
		}
		block = append(block, line)
	}
	flush()
	sort.Slice(tables, func(i, j int) bool { return tables[i].StartLine < tables[j].StartLine })
	return tables
}

func normalizeOptions(opts Options) Options {
	if opts.MinRows <= 0 {
		opts.MinRows = 2
	}
	if opts.MinColumns <= 0 {
		opts.MinColumns = 2
	}
	if opts.ConfidenceThreshold <= 0 {
		opts.ConfidenceThreshold = 0.6
	}
	return opts
}

type groove struct{ start, end int }

func projectBlocks(s *screen.Screen, chunks [][]token.Chunk, lines []int, opts Options) []Table {
	var tables []Table
	for len(lines) >= opts.MinRows {
		if table := projectBlock(s, chunks, lines, opts); table != nil {
			return append(tables, *table)
		}
		cut := -1
		var prefix *Table
		for end := opts.MinRows; end < len(lines); end++ {
			if table := projectBlock(s, chunks, lines[:end], opts); table != nil {
				cut, prefix = end, table
				break
			}
		}
		if cut < 0 {
			lines = lines[1:]
			continue
		}
		tables = append(tables, *prefix)
		lines = lines[cut:]
	}
	return tables
}

// ponytail: start/prefix splitting searches invalid blocks repeatedly; visible
// panes are small. Add incremental groove tracking if large scrollback needs it.
func projectBlock(s *screen.Screen, chunks [][]token.Chunk, lines []int, opts Options) *Table {
	header := isHeader(chunks[lines[0]])
	if !header && !stableChunkShape(chunks, lines) {
		return nil
	}
	grooves, left, right := commonGrooves(s, lines)
	accepted := make([]groove, 0, len(grooves))
	hasWide, hasHeaderSingle := false, false
	for _, g := range grooves {
		width := g.end - g.start
		switch {
		case width >= 2:
			accepted = append(accepted, g)
			hasWide = true
		case width == 1 && header && headerSupportsGroove(chunks[lines[0]], g):
			accepted = append(accepted, g)
			hasHeaderSingle = true
		}
	}
	if len(accepted) == 0 || (!hasWide && !hasHeaderSingle) {
		return nil
	}

	type slot struct{ start, end int }
	var slots []slot
	cursor := left
	for _, g := range accepted {
		if g.start > cursor {
			slots = append(slots, slot{start: cursor, end: g.start})
		}
		cursor = g.end
	}
	if right > cursor {
		slots = append(slots, slot{start: cursor, end: right})
	}
	if len(slots) < opts.MinColumns {
		return nil
	}

	table := &Table{StartLine: lines[0], EndLine: lines[len(lines)-1], HeaderLine: -1}
	if header {
		table.HeaderLine = lines[0]
	}
	table.Columns = make([]Column, len(slots))
	for i, slot := range slots {
		table.Columns[i] = Column{Index: i, Start: slot.start, End: slot.end, Align: "left", Weight: 1}
	}
	table.Rows = make([]Row, len(lines))
	for rowIndex, line := range lines {
		row := Row{Line: line, Cells: make([]Cell, len(slots))}
		for col, slot := range slots {
			row.Cells[col] = extractCell(s, line, slot.start, slot.end, col)
		}
		table.Rows[rowIndex] = row
	}
	if header {
		for col := range table.Columns {
			table.Columns[col].Name = table.Rows[0].Cells[col].Text
		}
	}
	for col := range table.Columns {
		table.Columns[col].Align = columnAlignment(table.Rows, col)
	}
	table.Confidence = confidence(table)
	if table.Confidence < opts.ConfidenceThreshold {
		return nil
	}
	return table
}

func commonGrooves(s *screen.Screen, lines []int) ([]groove, int, int) {
	maxWidth := 0
	for _, line := range lines {
		maxWidth = max(maxWidth, s.Lines[line].Width)
	}
	occupied := make([]bool, maxWidth)
	left, right := -1, 0
	for _, line := range lines {
		x := 0
		for _, cell := range s.Lines[line].Cells {
			r, _ := utf8.DecodeRuneInString(cell.Grapheme)
			if !unicode.IsSpace(r) {
				for col := x; col < x+int(cell.Width); col++ {
					occupied[col] = true
				}
				if left < 0 || x < left {
					left = x
				}
				right = max(right, x+int(cell.Width))
			}
			x += int(cell.Width)
		}
	}
	if left < 0 {
		return nil, 0, 0
	}

	var grooves []groove
	for col := left; col < right; {
		if occupied[col] {
			col++
			continue
		}
		start := col
		for col < right && !occupied[col] {
			col++
		}
		grooves = append(grooves, groove{start: start, end: col})
	}
	return grooves, left, right
}

func stableChunkShape(chunks [][]token.Chunk, lines []int) bool {
	counts := make(map[int]int, len(lines))
	best := 0
	for _, line := range lines {
		count := len(chunks[line])
		counts[count]++
		best = max(best, counts[count])
	}
	return best*10 >= len(lines)*7
}

func isHeader(chunks []token.Chunk) bool {
	if len(chunks) == 0 {
		return false
	}
	titleCase := 0
	for _, chunk := range chunks {
		if isNumberLike(chunk.Text) {
			return false
		}
		r, _ := utf8.DecodeRuneInString(chunk.Text)
		if unicode.IsUpper(r) && !strings.ContainsFunc(chunk.Text, unicode.IsDigit) {
			titleCase++
		}
	}
	return float64(titleCase)/float64(len(chunks)) >= 0.7
}

func isNumberLike(text string) bool {
	if _, err := strconv.ParseFloat(text, 64); err == nil {
		return true
	}
	if _, err := netip.ParseAddr(text); err == nil {
		return true
	}
	for _, layout := range []string{"2006-01-02", "02/01/2006", time.RFC3339} {
		if _, err := time.Parse(layout, text); err == nil {
			return true
		}
	}
	return false
}

func headerSupportsGroove(chunks []token.Chunk, g groove) bool {
	for i := 1; i < len(chunks); i++ {
		if g.start >= chunks[i-1].Span.EndCol && g.end == chunks[i].Span.StartCol {
			return true
		}
	}
	return false
}

func extractCell(s *screen.Screen, line, start, end, col int) Cell {
	startCol, endCol := -1, -1
	x := 0
	for _, cell := range s.Lines[line].Cells {
		next := x + int(cell.Width)
		r, _ := utf8.DecodeRuneInString(cell.Grapheme)
		if next > start && x < end && !unicode.IsSpace(r) {
			if startCol < 0 {
				startCol = x
			}
			endCol = next
		}
		x = next
	}
	if startCol < 0 {
		emptyAt := min(start, s.Lines[line].Width)
		return Cell{Span: screen.Span{Line: line, StartCol: emptyAt, EndCol: emptyAt}, Col: col, Empty: true}
	}
	byteStart, byteEnd := s.ByteOf(line, startCol), s.ByteOf(line, endCol)
	return Cell{
		Text: s.Lines[line].Plain[byteStart:byteEnd],
		Span: screen.Span{Line: line, StartCol: startCol, EndCol: endCol}, Col: col,
	}
}

func columnAlignment(rows []Row, col int) string {
	leftCounts := map[int]int{}
	rightCounts := map[int]int{}
	total, leftMax, rightMax := 0, 0, 0
	for _, row := range rows {
		cell := row.Cells[col]
		if cell.Empty {
			continue
		}
		total++
		leftCounts[cell.Span.StartCol]++
		rightCounts[cell.Span.EndCol]++
		leftMax = max(leftMax, leftCounts[cell.Span.StartCol])
		rightMax = max(rightMax, rightCounts[cell.Span.EndCol])
	}
	if total > 0 && rightMax > leftMax {
		return "right"
	}
	return "left"
}

func confidence(table *Table) float64 {
	filled, aligned := 0, 0
	for _, row := range table.Rows {
		for _, cell := range row.Cells {
			if !cell.Empty {
				filled++
			}
		}
	}
	for _, col := range table.Columns {
		leftCounts := map[int]int{}
		rightCounts := map[int]int{}
		total, best := 0, 0
		for _, row := range table.Rows {
			cell := row.Cells[col.Index]
			if cell.Empty {
				continue
			}
			total++
			leftCounts[cell.Span.StartCol]++
			rightCounts[cell.Span.EndCol]++
			best = max(best, max(leftCounts[cell.Span.StartCol], rightCounts[cell.Span.EndCol]))
		}
		if total > 0 && float64(best)/float64(total) >= 0.8 {
			aligned++
		}
	}
	fillRatio := float64(filled) / float64(len(table.Rows)*len(table.Columns))
	alignScore := float64(aligned) / float64(len(table.Columns))
	rowScore := min(1.0, float64(len(table.Rows))/5)
	return normalizeConfidence(0.5*fillRatio + 0.3*alignScore + 0.2*rowScore)
}

func normalizeConfidence(score float64) float64 {
	return math.Round(score*1e6) / 1e6
}

func hasExplicitDelimiter(s *screen.Screen, line int) bool {
	count := 0
	for _, cell := range s.Lines[line].Cells {
		if cell.Grapheme == "|" || cell.Grapheme == "│" {
			count++
		}
	}
	return count >= 2
}
