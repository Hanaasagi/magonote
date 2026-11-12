package table

import (
	"unicode"
	"unicode/utf8"

	"github.com/Hanaasagi/magonote/pkg/textdetection/screen"
	"github.com/Hanaasagi/magonote/pkg/textdetection/token"
)

func detectExplicit(s *screen.Screen, skip map[int]bool, opts Options) []Table {
	var tables []Table
	var lines []int
	var delimiters [][]int
	flush := func() {
		if len(lines) >= opts.MinRows {
			if table := explicitTable(s, lines, delimiters, opts); table != nil {
				tables = append(tables, *table)
			}
		}
		lines, delimiters = nil, nil
	}

	for line := range s.Lines {
		if skip[line] {
			flush()
			continue
		}
		plain := s.Lines[line].Plain
		if plain == "" {
			flush()
			continue
		}
		if token.IsSeparatorLine(plain) {
			continue
		}
		positions := delimiterPositions(s, line)
		if len(positions) < 2 {
			flush()
			continue
		}
		if len(delimiters) > 0 && len(delimiters[0]) != len(positions) {
			flush()
		}
		lines = append(lines, line)
		delimiters = append(delimiters, positions)
	}
	flush()
	return tables
}

func delimiterPositions(s *screen.Screen, line int) []int {
	var positions []int
	x := 0
	for _, cell := range s.Lines[line].Cells {
		if cell.Grapheme == "|" || cell.Grapheme == "│" {
			positions = append(positions, x)
		}
		x += int(cell.Width)
	}
	return positions
}

type explicitSlot struct{ start, end int }

func explicitTable(s *screen.Screen, lines []int, delimiters [][]int, opts Options) *Table {
	left, right := false, false
	for i, line := range lines {
		positions := delimiters[i]
		left = left || hasTextBetween(s, []int{line}, 0, positions[0])
		right = right || hasTextBetween(s, []int{line}, positions[len(positions)-1]+1, s.Lines[line].Width)
	}
	slots := makeExplicitSlots(delimiters[0], s.Lines[lines[0]].Width, left, right)
	if len(slots) < opts.MinColumns {
		return nil
	}

	table := &Table{StartLine: lines[0], EndLine: lines[len(lines)-1], HeaderLine: -1}
	table.Columns = make([]Column, len(slots))
	for i, slot := range slots {
		table.Columns[i] = Column{Index: i, Start: slot.start, End: slot.end, Align: "left", Weight: 1}
	}
	table.Rows = make([]Row, len(lines))
	for rowIndex, line := range lines {
		rowSlots := makeExplicitSlots(delimiters[rowIndex], s.Lines[line].Width, left, right)
		row := Row{Line: line, Cells: make([]Cell, len(slots))}
		for col, slot := range rowSlots {
			row.Cells[col] = extractCell(s, line, slot.start, slot.end, col)
		}
		table.Rows[rowIndex] = row
	}

	headerChunks := make([]token.Chunk, len(table.Columns))
	for i, cell := range table.Rows[0].Cells {
		headerChunks[i] = token.Chunk{Text: cell.Text, Span: cell.Span}
	}
	if isHeader(headerChunks) {
		table.HeaderLine = lines[0]
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

func makeExplicitSlots(delimiters []int, lineWidth int, left, right bool) []explicitSlot {
	var slots []explicitSlot
	if left {
		slots = append(slots, explicitSlot{start: 0, end: delimiters[0]})
	}
	for i := 1; i < len(delimiters); i++ {
		slots = append(slots, explicitSlot{start: delimiters[i-1] + 1, end: delimiters[i]})
	}
	if right {
		slots = append(slots, explicitSlot{start: delimiters[len(delimiters)-1] + 1, end: lineWidth})
	}
	return slots
}

func hasTextBetween(s *screen.Screen, lines []int, start, end int) bool {
	for _, line := range lines {
		x := 0
		for _, cell := range s.Lines[line].Cells {
			next := x + int(cell.Width)
			r, _ := utf8.DecodeRuneInString(cell.Grapheme)
			if next > start && x < end && !unicode.IsSpace(r) {
				return true
			}
			x = next
		}
	}
	return false
}
