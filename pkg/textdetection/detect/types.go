package detect

import (
	"github.com/Hanaasagi/magonote/pkg/textdetection/entity"
	"github.com/Hanaasagi/magonote/pkg/textdetection/profile"
	"github.com/Hanaasagi/magonote/pkg/textdetection/screen"
	"github.com/Hanaasagi/magonote/pkg/textdetection/table"
	"github.com/Hanaasagi/magonote/pkg/textdetection/token"
)

type Options struct {
	Style bool
	Words bool
}

type Context struct {
	Screen  *screen.Screen
	Chunks  [][]token.Chunk
	Tables  []table.Table
	Prompts []profile.Prompt
	Blocks  []profile.Block
	Options Options
}

type Detector interface {
	Name() string
	Detect(ctx *Context) []entity.Candidate
}

func candidateFromBytes(ctx *Context, kind entity.Kind, line, start, end int, class int, source string, confidence float64) entity.Candidate {
	span := ctx.Screen.SpanOf(line, start, end)
	byteStart, byteEnd := ctx.Screen.ByteOf(line, span.StartCol), ctx.Screen.ByteOf(line, span.EndCol)
	return entity.Candidate{
		Kind: kind, Text: ctx.Screen.Lines[line].Plain[byteStart:byteEnd], Span: span,
		Class: class, Source: source, Confidence: confidence,
	}
}
