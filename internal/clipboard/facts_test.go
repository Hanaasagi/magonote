package clipboard

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Hanaasagi/magonote/internal/tmux"
)

type fakeQueryer struct {
	responses map[string]string
	errs      map[string]bool
	version   tmux.Version
}

func (f fakeQueryer) Query(_ context.Context, args ...string) (string, error) {
	key := strings.Join(args, " ")
	if f.errs[key] {
		return "", errors.New("unknown")
	}
	return f.responses[key], nil
}

func (f fakeQueryer) Version(_ context.Context) (tmux.Version, error) {
	return f.version, nil
}

func TestProbeAttachedAndName(t *testing.T) {
	q := fakeQueryer{
		responses: map[string]string{
			"display-message -p #{client_name}":     "client7",
			"display-message -p #{client_termname}": "xterm-256color",
		},
		errs:    map[string]bool{"show-environment -g TMUX": true},
		version: tmux.Version{Major: 3, Minor: 8},
	}
	f := Probe(context.Background(), q)
	if f.ClientName != "client7" || !f.ClientAttached {
		t.Errorf("client = %q attached=%v", f.ClientName, f.ClientAttached)
	}
	if f.Nested {
		t.Error("xterm client with no outer TMUX should not be nested")
	}
	if f.Version.Minor != 8 {
		t.Errorf("version = %+v", f.Version)
	}
}

func TestDetectNested(t *testing.T) {
	cases := []struct {
		name string
		q    fakeQueryer
		want bool
	}{
		{
			name: "outer TMUX in global env",
			q: fakeQueryer{responses: map[string]string{
				"show-environment -g TMUX": "TMUX=/tmp/outer.sock,1,0",
			}},
			want: true,
		},
		{
			name: "tmux termname (ssh nesting heuristic)",
			q: fakeQueryer{
				responses: map[string]string{"display-message -p #{client_termname}": "tmux-256color"},
				errs:      map[string]bool{"show-environment -g TMUX": true},
			},
			want: true,
		},
		{
			name: "screen termname",
			q: fakeQueryer{
				responses: map[string]string{"display-message -p #{client_termname}": "screen-256color"},
				errs:      map[string]bool{"show-environment -g TMUX": true},
			},
			want: true,
		},
		{
			name: "plain terminal",
			q: fakeQueryer{
				responses: map[string]string{"display-message -p #{client_termname}": "xterm-256color"},
				errs:      map[string]bool{"show-environment -g TMUX": true},
			},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := detectNested(context.Background(), tc.q); got != tc.want {
				t.Errorf("detectNested = %v, want %v", got, tc.want)
			}
		})
	}
}
