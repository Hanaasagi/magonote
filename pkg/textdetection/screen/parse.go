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
	cursor   int // byte offset into line.plain for CR overwrites
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
			count := p.tabWidth - plainDisplayWidth(p.line.plain.String())%p.tabWidth
			p.append(strings.Repeat(" ", count))
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

func (p *parser) append(text string) {
	if text == "" {
		return
	}
	lineLen := p.line.plain.Len()
	if p.cursor < lineLen {
		existing := p.line.plain.String()
		overEnd := p.cursor + len(text)
		if overEnd > lineLen {
			p.line.plain.Reset()
			p.line.plain.WriteString(existing[:p.cursor])
			p.line.plain.WriteString(text)
		} else {
			p.line.plain.Reset()
			p.line.plain.WriteString(existing[:p.cursor])
			p.line.plain.WriteString(text)
			p.line.plain.WriteString(existing[overEnd:])
		}
		p.line.runs = append(p.line.runs, styleRun{start: p.cursor, end: p.cursor + len(text), style: p.style})
		p.cursor += len(text)
		return
	}
	start := lineLen
	p.line.plain.WriteString(text)
	if len(p.line.runs) > 0 {
		last := &p.line.runs[len(p.line.runs)-1]
		if last.style == p.style && last.end == start {
			last.end = p.line.plain.Len()
		} else {
			p.line.runs = append(p.line.runs, styleRun{start: start, end: p.line.plain.Len(), style: p.style})
		}
	} else {
		p.line.runs = append(p.line.runs, styleRun{start: start, end: p.line.plain.Len(), style: p.style})
	}
	p.cursor = p.line.plain.Len()
}

func plainDisplayWidth(plain string) int {
	width := 0
	graphemes := uniseg.NewGraphemes(plain)
	for graphemes.Next() {
		width += GraphemeWidth(graphemes.Str())
	}
	return width
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
