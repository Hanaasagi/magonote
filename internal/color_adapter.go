package internal

import (
	"regexp"
	"strconv"

	"github.com/fatih/color"
	"github.com/gdamore/tcell/v2"

	tdscreen "github.com/Hanaasagi/magonote/pkg/textdetection/screen"
)

// This file holds the adapters between the backend-agnostic color/style models
// (pkg/textdetection/screen and internal Color) and tcell's rendering types.
// Keeping them separate isolates the rendering backend from presentation logic.

func screenStyleToTcell(style tdscreen.Style) tcell.Style {
	result := tcell.StyleDefault.
		Foreground(screenColorToTcell(style.Fg)).
		Background(screenColorToTcell(style.Bg)).
		Bold(style.Attr&tdscreen.AttrBold != 0).
		Dim(style.Attr&tdscreen.AttrDim != 0).
		Italic(style.Attr&tdscreen.AttrItalic != 0).
		Underline(style.Attr&tdscreen.AttrUnderline != 0).
		Blink(style.Attr&tdscreen.AttrBlink != 0).
		Reverse(style.Attr&tdscreen.AttrReverse != 0).
		StrikeThrough(style.Attr&tdscreen.AttrStrike != 0)
	// tcell has no hidden attribute; retain the other style information.
	if style.Hyperlink != "" {
		result = result.Url(style.Hyperlink)
	}
	return result
}

func screenColorToTcell(color tdscreen.Color) tcell.Color {
	switch color.Mode {
	case tdscreen.ColorANSI16, tdscreen.ColorANSI256:
		return tcell.PaletteColor(int(color.Index))
	case tdscreen.ColorRGB:
		return tcell.NewRGBColor(int32(color.R), int32(color.G), int32(color.B))
	default:
		return tcell.ColorDefault
	}
}

// Pre-compiled pattern for RGB color matching.
var rgbColorPattern = regexp.MustCompile(`\x1b\[38;2;(\d+);(\d+);(\d+)m`)

// colorToTcell converts a Color to a tcell.Color. A nil Color maps to the
// terminal default so zero-value themes are safe.
func colorToTcell(c Color) tcell.Color {
	if c == nil {
		return tcell.ColorDefault
	}
	if cw, ok := c.(ColorWrapper); ok {
		if cw.isRGB {
			return tcell.NewRGBColor(int32(cw.r), int32(cw.g), int32(cw.b))
		}

		colorMap := map[color.Attribute]tcell.Color{
			color.FgBlack:   tcell.ColorBlack,
			color.FgRed:     tcell.ColorRed,
			color.FgGreen:   tcell.ColorGreen,
			color.FgYellow:  tcell.ColorYellow,
			color.FgBlue:    tcell.ColorBlue,
			color.FgMagenta: tcell.ColorFuchsia,
			color.FgCyan:    tcell.ColorAqua,
			color.FgWhite:   tcell.ColorWhite,
			color.Reset:     tcell.ColorDefault,
		}

		if tcellColor, exists := colorMap[cw.GetFgColor()]; exists {
			return tcellColor
		}
	}

	// Fallback: parse RGB from colored text.
	coloredText := c.FgString("test")
	if matches := rgbColorPattern.FindStringSubmatch(coloredText); len(matches) == 4 {
		r, _ := strconv.Atoi(matches[1])
		g, _ := strconv.Atoi(matches[2])
		b, _ := strconv.Atoi(matches[3])
		return tcell.NewRGBColor(int32(r), int32(g), int32(b))
	}

	return tcell.ColorDefault
}
