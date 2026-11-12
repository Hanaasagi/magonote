package screen

import (
	"strconv"
	"strings"
)

func applySGR(style Style, params string) Style {
	parts := strings.Split(params, ";")
	for i := 0; i < len(parts); i++ {
		sub := strings.Split(parts[i], ":")
		code := sgrNumber(sub[0])
		switch code {
		case 0:
			link := style.Hyperlink
			style = Style{Hyperlink: link}
		case 1:
			style.Attr |= AttrBold
		case 2:
			style.Attr |= AttrDim
		case 3:
			style.Attr |= AttrItalic
		case 4, 21:
			style.Attr |= AttrUnderline
		case 5:
			style.Attr |= AttrBlink
		case 7:
			style.Attr |= AttrReverse
		case 8:
			style.Attr |= AttrHidden
		case 9:
			style.Attr |= AttrStrike
		case 22:
			style.Attr &^= AttrBold | AttrDim
		case 23:
			style.Attr &^= AttrItalic
		case 24:
			style.Attr &^= AttrUnderline
		case 25:
			style.Attr &^= AttrBlink
		case 27:
			style.Attr &^= AttrReverse
		case 28:
			style.Attr &^= AttrHidden
		case 29:
			style.Attr &^= AttrStrike
		case 38, 48, 58:
			color, consumed, ok := extendedColor(parts, i, sub)
			i += consumed
			if ok && code != 58 {
				if code == 38 {
					style.Fg = color
				} else {
					style.Bg = color
				}
			}
		case 39:
			style.Fg = Color{}
		case 49:
			style.Bg = Color{}
		case 59:
			// Default underline color; underline color is not retained.
		default:
			switch {
			case code >= 30 && code <= 37:
				style.Fg = Color{Mode: ColorANSI16, Index: uint8(code - 30)}
			case code >= 90 && code <= 97:
				style.Fg = Color{Mode: ColorANSI16, Index: uint8(code - 90 + 8)}
			case code >= 40 && code <= 47:
				style.Bg = Color{Mode: ColorANSI16, Index: uint8(code - 40)}
			case code >= 100 && code <= 107:
				style.Bg = Color{Mode: ColorANSI16, Index: uint8(code - 100 + 8)}
			}
		}
	}
	return style
}

func extendedColor(parts []string, index int, sub []string) (Color, int, bool) {
	if len(sub) > 1 {
		if len(sub) >= 3 && sub[1] == "5" {
			n, ok := sgrByte(sub[2])
			return Color{Mode: ColorANSI256, Index: n}, 0, ok
		}
		if len(sub) >= 5 && sub[1] == "2" {
			r, rok := sgrByte(sub[len(sub)-3])
			g, gok := sgrByte(sub[len(sub)-2])
			b, bok := sgrByte(sub[len(sub)-1])
			return Color{Mode: ColorRGB, R: r, G: g, B: b}, 0, rok && gok && bok
		}
		return Color{}, 0, false
	}

	if index+1 >= len(parts) {
		return Color{}, 0, false
	}
	mode := sgrNumber(parts[index+1])
	switch mode {
	case 5:
		if index+2 >= len(parts) {
			return Color{}, 1, false
		}
		n, ok := sgrByte(parts[index+2])
		return Color{Mode: ColorANSI256, Index: n}, 2, ok
	case 2:
		if index+4 >= len(parts) {
			return Color{}, len(parts) - index - 1, false
		}
		r, rok := sgrByte(parts[index+2])
		g, gok := sgrByte(parts[index+3])
		b, bok := sgrByte(parts[index+4])
		return Color{Mode: ColorRGB, R: r, G: g, B: b}, 4, rok && gok && bok
	default:
		return Color{}, 0, false
	}
}

func sgrNumber(s string) int {
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return n
}

func sgrByte(s string) (uint8, bool) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 255 {
		return 0, false
	}
	return uint8(n), true
}
