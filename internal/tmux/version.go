package tmux

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Minimum supported tmux version. magonote relies on `load-buffer -w` (OSC 52
// forwarding as tmux's own output), introduced in tmux 3.2.
const (
	MinMajor = 3
	MinMinor = 2
)

// Version is a parsed tmux version.
type Version struct {
	Major  int
	Minor  int
	Suffix string // release letter, e.g. "a" in "3.2a"
	Raw    string // original version string
}

// versionRe matches "tmux 3.8", "tmux 3.2a", and development "tmux next-3.4".
var versionRe = regexp.MustCompile(`^tmux\s+(?:next-)?(\d+)\.(\d+)([a-zA-Z]*)`)

// ParseVersion parses the output of `tmux -V`.
func ParseVersion(s string) (Version, error) {
	s = strings.TrimSpace(s)
	m := versionRe.FindStringSubmatch(s)
	if m == nil {
		return Version{Raw: s}, fmt.Errorf("unrecognized tmux version %q", s)
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	return Version{Major: major, Minor: minor, Suffix: m[3], Raw: s}, nil
}

// AtLeast reports whether v is >= major.minor. Release-letter suffixes are
// ignored for ordering (3.2a >= 3.2).
func (v Version) AtLeast(major, minor int) bool {
	if v.Major != major {
		return v.Major > major
	}
	return v.Minor >= minor
}

// String returns the original version string.
func (v Version) String() string { return v.Raw }

// Version queries and parses the server's tmux version.
func (c *Client) Version(ctx context.Context) (Version, error) {
	out, err := c.Query(ctx, "-V")
	if err != nil {
		return Version{}, err
	}
	return ParseVersion(out)
}

// CheckSupported returns an error when the tmux version is below the supported
// floor. A parse failure is also returned as an error; callers may choose to
// warn and proceed rather than hard-fail on an unusual version string.
func (c *Client) CheckSupported(ctx context.Context) error {
	v, err := c.Version(ctx)
	if err != nil {
		return err
	}
	if !v.AtLeast(MinMajor, MinMinor) {
		return fmt.Errorf("magonote requires tmux >= %d.%d, found %s; please upgrade tmux",
			MinMajor, MinMinor, v.Raw)
	}
	return nil
}
