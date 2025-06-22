package internal

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/fatih/color"
)

// Color interface defines how to colorize text
type Color interface {
	FgString(text string) string
	GetFgColor() color.Attribute
}

// ColorWrapper wraps fatih/color functionality
type ColorWrapper struct {
	colorFunc func(...interface{}) string
	colorAttr color.Attribute
	isRGB     bool
	r, g, b   uint8
}

// FgString returns a string with the color applied
func (c ColorWrapper) FgString(text string) string {
	if c.isRGB {
		// Since fatih/color doesn't directly support RGB in the same way,
		// we'll fall back to the ANSI escape sequence for RGB
		return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s\x1b[0m", c.r, c.g, c.b, text)
	}
	return c.colorFunc(text)
}

// GetFgColor returns the color.Attribute for this color
func (c ColorWrapper) GetFgColor() color.Attribute {
	return c.colorAttr
}

var rgbRegex = regexp.MustCompile(`^#([a-fA-F0-9]{2})([a-fA-F0-9]{2})([a-fA-F0-9]{2})$`)

var (
	colorCache = make(map[string]Color, 32)
	colorMutex sync.RWMutex
)

var predefinedColors = map[string]ColorWrapper{
	"black": {
		colorFunc: color.New(color.FgBlack).SprintFunc(),
		colorAttr: color.FgBlack,
	},
	"red": {
		colorFunc: color.New(color.FgRed).SprintFunc(),
		colorAttr: color.FgRed,
	},
	"green": {
		colorFunc: color.New(color.FgGreen).SprintFunc(),
		colorAttr: color.FgGreen,
	},
	"yellow": {
		colorFunc: color.New(color.FgYellow).SprintFunc(),
		colorAttr: color.FgYellow,
	},
	"blue": {
		colorFunc: color.New(color.FgBlue).SprintFunc(),
		colorAttr: color.FgBlue,
	},
	"magenta": {
		colorFunc: color.New(color.FgMagenta).SprintFunc(),
		colorAttr: color.FgMagenta,
	},
	"cyan": {
		colorFunc: color.New(color.FgCyan).SprintFunc(),
		colorAttr: color.FgCyan,
	},
	"white": {
		colorFunc: color.New(color.FgWhite).SprintFunc(),
		colorAttr: color.FgWhite,
	},
	"default": {
		colorFunc: color.New(color.Reset).SprintFunc(),
		colorAttr: color.Reset,
	},
}

// GetColor parses a color string and returns a Color interface
func GetColor(name string) Color {
	// Check cache first
	colorMutex.RLock()
	if cached, exists := colorCache[name]; exists {
		colorMutex.RUnlock()
		return cached
	}
	colorMutex.RUnlock()

	var result Color

	// Check for RGB color
	if m := rgbRegex.FindStringSubmatch(name); m != nil {
		r, _ := strconv.ParseUint(m[1], 16, 8)
		g, _ := strconv.ParseUint(m[2], 16, 8)
		b, _ := strconv.ParseUint(m[3], 16, 8)
		result = ColorWrapper{
			colorFunc: color.New(color.FgWhite).SprintFunc(),
			colorAttr: color.FgWhite,
			isRGB:     true,
			r:         uint8(r),
			g:         uint8(g),
			b:         uint8(b),
		}
	} else {
		lowerName := strings.ToLower(name)
		if predefined, exists := predefinedColors[lowerName]; exists {
			result = predefined
		} else {
			panic(fmt.Sprintf("Unknown color: %s", name))
		}
	}

	colorMutex.Lock()
	colorCache[name] = result
	colorMutex.Unlock()

	return result
}
