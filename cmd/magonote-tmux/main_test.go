package main

import (
	"reflect"
	"testing"
)

func TestPaneInfo_HasScrollData(t *testing.T) {
	tests := []struct {
		name     string
		paneInfo *PaneInfo
		want     bool
	}{
		{
			name: "has scroll data when in mode with positive position",
			paneInfo: &PaneInfo{
				InMode:         true,
				ScrollPosition: 10,
			},
			want: true,
		},
		{
			name: "has scroll data when in mode with zero position",
			paneInfo: &PaneInfo{
				InMode:         true,
				ScrollPosition: 0,
			},
			want: true,
		},
		{
			name: "no scroll data when not in mode",
			paneInfo: &PaneInfo{
				InMode:         false,
				ScrollPosition: 10,
			},
			want: false,
		},
		{
			name: "no scroll data when in mode with negative position",
			paneInfo: &PaneInfo{
				InMode:         true,
				ScrollPosition: -1,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.paneInfo.HasScrollData(); got != tt.want {
				t.Errorf("PaneInfo.HasScrollData() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMagonote_parsePaneInfo(t *testing.T) {
	m := &Magonote{}

	tests := []struct {
		name    string
		parts   []string
		want    *PaneInfo
		wantErr bool
	}{
		{
			name:  "normal pane not in mode",
			parts: []string{"%1", "0", "24", "0", "0", "active"},
			want: &PaneInfo{
				ID:             "%1",
				Height:         24,
				ScrollPosition: 0,
				InMode:         false,
				Zoomed:         false,
			},
			wantErr: false,
		},
		{
			name:  "pane in scroll mode",
			parts: []string{"%2", "1", "30", "15", "1", "active"},
			want: &PaneInfo{
				ID:             "%2",
				Height:         30,
				ScrollPosition: 15,
				InMode:         true,
				Zoomed:         true,
			},
			wantErr: false,
		},
		{
			name:    "insufficient parts",
			parts:   []string{"%1", "0", "24"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid height",
			parts:   []string{"%1", "0", "invalid", "0", "0", "active"},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := m.parsePaneInfo(tt.parts)
			if (err != nil) != tt.wantErr {
				t.Errorf("Magonote.parsePaneInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Magonote.parsePaneInfo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMagonote_buildScrollParams(t *testing.T) {
	tests := []struct {
		name           string
		activePaneInfo *PaneInfo
		want           string
	}{
		{
			name: "no scroll data",
			activePaneInfo: &PaneInfo{
				InMode: false,
				Height: 24,
			},
			want: "",
		},
		{
			name:           "nil pane info",
			activePaneInfo: nil,
			want:           "",
		},
		{
			name: "with scroll data",
			activePaneInfo: &PaneInfo{
				InMode:         true,
				Height:         30,
				ScrollPosition: 10,
			},
			want: " -S -10 -E 19",
		},
		{
			name: "zero scroll position",
			activePaneInfo: &PaneInfo{
				InMode:         true,
				Height:         24,
				ScrollPosition: 0,
			},
			want: " -S 0 -E 23",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Magonote{
				activePaneInfo: tt.activePaneInfo,
			}
			if got := m.buildScrollParams(); got != tt.want {
				t.Errorf("Magonote.buildScrollParams() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMagonote_buildCaptureCommand(t *testing.T) {
	tests := []struct {
		name           string
		activePaneInfo *PaneInfo
		want           string
	}{
		{
			name: "normal pane without scroll",
			activePaneInfo: &PaneInfo{
				ID:     "%1",
				InMode: false,
				Height: 24,
			},
			want: "tmux capture-pane -J -t %1 -p -e",
		},
		{
			name: "pane with scroll data",
			activePaneInfo: &PaneInfo{
				ID:             "%2",
				InMode:         true,
				Height:         30,
				ScrollPosition: 10,
			},
			want: "tmux capture-pane -J -t %2 -p -e -S -10 -E 19 | tail -n 30",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Magonote{
				activePaneInfo: tt.activePaneInfo,
			}
			if got := m.buildCaptureCommand(); got != tt.want {
				t.Errorf("Magonote.buildCaptureCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}
