package table

import "github.com/Hanaasagi/magonote/pkg/textdetection/screen"

type Options struct {
	MinRows             int
	MinColumns          int
	ConfidenceThreshold float64
}

type Table struct {
	StartLine  int      `json:"start_line"`
	EndLine    int      `json:"end_line"`
	HeaderLine int      `json:"header_line"`
	Columns    []Column `json:"columns"`
	Rows       []Row    `json:"rows"`
	Confidence float64  `json:"confidence"`
}

type Column struct {
	Index    int     `json:"index"`
	Start    int     `json:"start"`
	End      int     `json:"end"`
	Name     string  `json:"name,omitempty"`
	Align    string  `json:"align"`
	Semantic string  `json:"semantic,omitempty"`
	Weight   float64 `json:"weight"`
}

type Row struct {
	Line  int    `json:"line"`
	Cells []Cell `json:"cells"`
}

type Cell struct {
	Text  string      `json:"text"`
	Span  screen.Span `json:"span"`
	Col   int         `json:"col"`
	Empty bool        `json:"empty"`
}
