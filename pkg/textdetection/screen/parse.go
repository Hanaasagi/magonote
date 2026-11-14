package screen

import (
	"strings"

	"github.com/rivo/uniseg"
)

type ColorMode uint8

const (
	ColorDefault ColorMode = iota
	ColorANSI16
	ColorANSI256
	ColorRGB
)

type Color struct {
	Mode  ColorMode `json:"mode"`
	Index uint8     `json:"index,omitempty"`
	R     uint8     `json:"r,omitempty"`
	G     uint8     `json:"g,omitempty"`
	B     uint8     `json:"b,omitempty"`
}

type AttrMask uint16

const (
	AttrBold AttrMask = 1 << iota
	AttrDim
	AttrItalic
	AttrUnderline
	AttrBlink
	AttrReverse
	AttrHidden
	AttrStrike
)

type Style struct {
	Fg        Color    `json:"fg"`
	Bg        Color    `json:"bg"`
	Attr      AttrMask `json:"attr"`
	Hyperlink string   `json:"hyperlink,omitempty"`
}

type styleRun struct {
	start, end int
	style      Style
}

type lineBuilder struct {
	plain strings.Builder
	runs  []styleRun
}

// Parse converts captured terminal text into a display-column grid. Invalid
// input and incomplete escape sequences are ignored rather than returned as
// errors. An optional tab width defaults to 8.
func Parse(input string, tabWidth ...int) *Screen {
	tab := 8
	if len(tabWidth) > 0 && tabWidth[0] > 0 {
		tab = tabWidth[0]
	}

	p := parser{tabWidth: tab}
	p.parse(input)
	p.finishLine()
	return &Screen{Lines: p.lines}
}

type parser struct {
	style    Style
	tabWidth int
	line     lineBuilder
	lines    []Line
	cursor   int // display column position for CR/tab handling
}

func (p *parser) parse(input string) {
	for i := 0; i < len(input); {
		b := input[i]
		switch {
		case b == '\n':
			p.finishLine()
			i++
		case b == '\r':
			if i+1 < len(input) && input[i+1] == '\n' {
				i++
				continue
			}
			p.resetLine()
			i++
		case b == '\t':
			tabStop := p.cursor + (p.tabWidth - p.cursor%p.tabWidth)
			lineWidth := uniseg.StringWidth(p.line.plain.String())
			if tabStop > lineWidth {
				padding := strings.Repeat(" ", tabStop-lineWidth)
				start := p.line.plain.Len()
				p.line.plain.WriteString(padding)
				end := p.line.plain.Len()
				p.line.runs = append(p.line.runs, styleRun{start: start, end: end, style: p.style})
			}
			p.cursor = tabStop
			i++
		case b == 0x1b:
			i = p.escape(input, i)
		case b < 0x20 || b == 0x7f:
			i++
		default:
			start := i
			for i < len(input) {
				b = input[i]
				if b == 0x1b || b == '\n' || b == '\r' || b == '\t' || b < 0x20 || b == 0x7f {
					break
				}
				i++
			}
			p.append(input[start:i])
		}
	}
}

func (p *parser) resetLine() {
	p.cursor = 0
}

// displayColToByte returns the byte offset in text corresponding to the given
// display column. If col falls in the middle of a wide character, it rounds up
// to the next grapheme boundary.
func displayColToByte(text string, col int) int {
	if col <= 0 {
		return 0
	}
	pos := 0
	graphemes := uniseg.NewGraphemes(text)
	for graphemes.Next() {
		_, end := graphemes.Positions()
		w := uniseg.StringWidth(graphemes.Str())
		pos += w
		if pos >= col {
			return end
		}
	}
	return len(text)
}

func (p *parser) append(text string) {
	if text == "" {
		return
	}
	textWidth := uniseg.StringWidth(text)
	lineWidth := uniseg.StringWidth(p.line.plain.String())

	if p.cursor < lineWidth {
		existing := p.line.plain.String()
		startByte := displayColToByte(existing, p.cursor)
		overEndCol := p.cursor + textWidth
		var endByte int

		if overEndCol >= lineWidth {
			endByte = len(existing)
			p.line.plain.Reset()
			p.line.plain.WriteString(existing[:startByte])
			p.line.plain.WriteString(text)
		} else {
			endByte = displayColToByte(existing, overEndCol)
			p.line.plain.Reset()
			p.line.plain.WriteString(existing[:startByte])
			p.line.plain.WriteString(text)
			p.line.plain.WriteString(existing[endByte:])
		}

		newStart := startByte
		newEnd := startByte + len(text)
		p.overwriteRuns(startByte, endByte, newStart, newEnd)
		p.cursor += textWidth
		return
	}

	start := p.line.plain.Len()
	p.line.plain.WriteString(text)
	end := p.line.plain.Len()
	if len(p.line.runs) > 0 {
		last := &p.line.runs[len(p.line.runs)-1]
		if last.style == p.style && last.end == start {
			last.end = end
		} else {
			p.line.runs = append(p.line.runs, styleRun{start: start, end: end, style: p.style})
		}
	} else {
		p.line.runs = append(p.line.runs, styleRun{start: start, end: end, style: p.style})
	}
	p.cursor += textWidth
}

// overwriteRuns replaces style runs covering [oldStart, oldEnd) with a new run
// covering [newStart, newEnd) using the parser's current style. Runs after the
// overwritten region are shifted to account for byte-length changes.
func (p *parser) overwriteRuns(oldStart, oldEnd, newStart, newEnd int) {
	byteShift := (newEnd - newStart) - (oldEnd - oldStart)
	var result []styleRun
	inserted := false

	for _, run := range p.line.runs {
		if run.end <= oldStart {
			result = append(result, run)
			continue
		}
		if run.start >= oldEnd {
			if !inserted {
				result = append(result, styleRun{start: newStart, end: newEnd, style: p.style})
				inserted = true
			}
			result = append(result, styleRun{
				start: run.start + byteShift,
				end:   run.end + byteShift,
				style: run.style,
			})
			continue
		}
		if run.start < oldStart {
			result = append(result, styleRun{start: run.start, end: oldStart, style: run.style})
		}
		if !inserted {
			result = append(result, styleRun{start: newStart, end: newEnd, style: p.style})
			inserted = true
		}
		if run.end > oldEnd {
			result = append(result, styleRun{
				start: newEnd,
				end:   run.end + byteShift,
				style: run.style,
			})
		}
	}
	if !inserted {
		result = append(result, styleRun{start: newStart, end: newEnd, style: p.style})
	}
	p.line.runs = result
}

func (p *parser) finishLine() {
	plain := p.line.plain.String()
	type cellData struct {
		text       string
		style      Style
		width      int
		start, end int
	}
	var data []cellData
	runIndex := 0
	graphemes := uniseg.NewGraphemes(plain)
	for graphemes.Next() {
		start, end := graphemes.Positions()
		text := graphemes.Str()
		for runIndex+1 < len(p.line.runs) && start >= p.line.runs[runIndex].end {
			runIndex++
		}
		style := Style{}
		if runIndex < len(p.line.runs) {
			style = p.line.runs[runIndex].style
		}
		rawWidth := uniseg.StringWidth(text)
		if rawWidth == 0 && len(data) > 0 {
			data[len(data)-1].text += text
			data[len(data)-1].end = end
			continue
		}
		width := GraphemeWidth(text)
		data = append(data, cellData{text: text, style: style, width: width, start: start, end: end})
	}

	finalWidth := 0
	for _, item := range data {
		finalWidth += item.width
	}
	line := Line{
		Plain:        plain,
		Cells:        make([]Cell, 0, len(data)),
		colToByte:    make([]int, finalWidth+1),
		byteToCol:    make([]int, len(plain)+1),
		byteToEndCol: make([]int, len(plain)+1),
		Width:        finalWidth,
	}
	col := 0
	for _, item := range data {
		line.Cells = append(line.Cells, Cell{Grapheme: item.text, Width: int8(item.width), Style: item.style, ByteOff: item.start})
		endCol := col + item.width
		line.byteToCol[item.start] = col
		line.byteToEndCol[item.start] = col
		for offset := item.start + 1; offset < item.end; offset++ {
			line.byteToCol[offset] = col
			line.byteToEndCol[offset] = endCol
		}
		line.byteToCol[item.end] = endCol
		line.byteToEndCol[item.end] = endCol
		for c := col; c < endCol; c++ {
			line.colToByte[c] = item.start
		}
		line.colToByte[endCol] = item.end
		col = endCol
	}
	p.lines = append(p.lines, line)
	p.line = lineBuilder{}
	p.cursor = 0
}

func (p *parser) escape(input string, start int) int {
	if start+1 >= len(input) {
		return len(input)
	}
	switch input[start+1] {
	case '[':
		return p.csi(input, start)
	case ']':
		return p.osc(input, start)
	case 'P', 'X', '^', '_':
		return skipString(input, start+2)
	case '(', ')', '*', '+', '#', '%':
		return min(len(input), start+3)
	default:
		return min(len(input), start+2)
	}
}

func (p *parser) csi(input string, start int) int {
	paramStart := start + 2
	intermediate := false
	for i := paramStart; i < len(input); i++ {
		b := input[i]
		switch {
		case b >= 0x30 && b <= 0x3f && !intermediate:
			continue
		case b >= 0x20 && b <= 0x2f:
			intermediate = true
			continue
		case b >= 0x40 && b <= 0x7e:
			if b == 'm' && !intermediate {
				p.style = applySGR(p.style, input[paramStart:i])
			}
			return i + 1
		default:
			// Abandon malformed CSI and reprocess this byte in the ground state.
			return i
		}
	}
	return len(input)
}

func (p *parser) osc(input string, start int) int {
	contentStart := start + 2
	for i := contentStart; i < len(input); i++ {
		switch input[i] {
		case '\a':
			p.applyOSC(input[contentStart:i])
			return i + 1
		case '\n':
			return i
		case 0x1b:
			if i+1 < len(input) && input[i+1] == '\\' {
				p.applyOSC(input[contentStart:i])
				return i + 2
			}
			return i
		}
	}
	return len(input)
}

func (p *parser) applyOSC(content string) {
	parts := strings.SplitN(content, ";", 3)
	if len(parts) == 3 && parts[0] == "8" {
		p.style.Hyperlink = parts[2]
	}
}

func skipString(input string, index int) int {
	for i := index; i < len(input); i++ {
		if input[i] == 0x1b && i+1 < len(input) && input[i+1] == '\\' {
			return i + 2
		}
	}
	return len(input)
}
