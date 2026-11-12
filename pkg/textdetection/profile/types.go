package profile

import (
	"github.com/Hanaasagi/magonote/pkg/textdetection/screen"
	"github.com/Hanaasagi/magonote/pkg/textdetection/table"
	"github.com/Hanaasagi/magonote/pkg/textdetection/token"
)

type Options struct {
	ExcludePrompt  bool
	PromptPatterns []string
}

type Prompt struct {
	Line      int     `json:"line"`
	PrefixEnd int     `json:"prefix_end"`
	Command   Command `json:"command"`
}

type Command struct {
	Raw  string   `json:"raw"`
	Argv []string `json:"argv"`
}

type Block struct {
	Command   *Command `json:"command,omitempty"`
	StartLine int      `json:"start_line"`
	EndLine   int      `json:"end_line"`
}

// Context is the shared read-only view needed by command profiles. Tables are
// mutable so profiles can annotate columns before CellDetector runs.
type Context struct {
	Screen *screen.Screen
	Chunks [][]token.Chunk
	Tables []table.Table
}
