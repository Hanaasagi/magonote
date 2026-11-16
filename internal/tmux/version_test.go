package tmux

import "testing"

func TestParseVersion(t *testing.T) {
	cases := []struct {
		in      string
		major   int
		minor   int
		suffix  string
		wantErr bool
	}{
		{"tmux 3.8", 3, 8, "", false},
		{"tmux 3.2a", 3, 2, "a", false},
		{"tmux 3.3a", 3, 3, "a", false},
		{"tmux next-3.4", 3, 4, "", false},
		{"tmux 2.9", 2, 9, "", false},
		{"tmux 3.0", 3, 0, "", false},
		{"  tmux 3.8  ", 3, 8, "", false},
		{"tmux master", 0, 0, "", true},
		{"garbage", 0, 0, "", true},
		{"", 0, 0, "", true},
	}
	for _, tc := range cases {
		v, err := ParseVersion(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseVersion(%q) expected error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseVersion(%q) unexpected error: %v", tc.in, err)
			continue
		}
		if v.Major != tc.major || v.Minor != tc.minor || v.Suffix != tc.suffix {
			t.Errorf("ParseVersion(%q) = %+v, want %d.%d%q", tc.in, v, tc.major, tc.minor, tc.suffix)
		}
	}
}

func TestVersionAtLeast(t *testing.T) {
	cases := []struct {
		v     Version
		major int
		minor int
		want  bool
	}{
		{Version{Major: 3, Minor: 8}, 3, 2, true},
		{Version{Major: 3, Minor: 2}, 3, 2, true},
		{Version{Major: 3, Minor: 2, Suffix: "a"}, 3, 2, true},
		{Version{Major: 3, Minor: 1}, 3, 2, false},
		{Version{Major: 2, Minor: 9}, 3, 2, false},
		{Version{Major: 4, Minor: 0}, 3, 2, true},
		{Version{Major: 3, Minor: 0}, 3, 2, false},
	}
	for _, tc := range cases {
		if got := tc.v.AtLeast(tc.major, tc.minor); got != tc.want {
			t.Errorf("%+v.AtLeast(%d,%d) = %v, want %v", tc.v, tc.major, tc.minor, got, tc.want)
		}
	}
}
