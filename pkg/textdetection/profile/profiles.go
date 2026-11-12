package profile

import (
	"path/filepath"

	"github.com/Hanaasagi/magonote/pkg/textdetection/entity"
	"github.com/Hanaasagi/magonote/pkg/textdetection/screen"
	"github.com/Hanaasagi/magonote/pkg/textdetection/table"
)

type Profile interface {
	Name() string
	Match(command Command) bool
	Detect(block Block, ctx *Context) []entity.Candidate
}

func DetectBlocks(blocks []Block, ctx *Context) []entity.Candidate {
	profiles := []Profile{kubectlProfile{}, dockerProfile{}, gitProfile{}, lsProfile{}}
	var candidates []entity.Candidate
	for _, block := range blocks {
		if block.Command == nil {
			continue
		}
		for _, profile := range profiles {
			if profile.Match(*block.Command) {
				candidates = append(candidates, profile.Detect(block, ctx)...)
				break
			}
		}
	}
	return candidates
}

func commandBase(command Command) string {
	if len(command.Argv) == 0 {
		return ""
	}
	return filepath.Base(command.Argv[0])
}

func candidateFromBytes(ctx *Context, kind entity.Kind, line, start, end int, confidence float64, source string) entity.Candidate {
	span := ctx.Screen.SpanOf(line, start, end)
	byteStart, byteEnd := ctx.Screen.ByteOf(line, span.StartCol), ctx.Screen.ByteOf(line, span.EndCol)
	return entity.Candidate{
		Kind: kind, Text: ctx.Screen.Lines[line].Plain[byteStart:byteEnd], Span: span,
		Class: entity.ClassSemantic, Source: "profile", Confidence: confidence,
		Meta: map[string]string{"source": source},
	}
}

func candidateFromSpan(kind entity.Kind, text string, span screen.Span, confidence float64, source string) entity.Candidate {
	return entity.Candidate{Kind: kind, Text: text, Span: span, Class: entity.ClassSemantic, Source: "profile", Confidence: confidence, Meta: map[string]string{"source": source}}
}

func tablesInBlock(block Block, tables []table.Table) []int {
	var indices []int
	for i := range tables {
		if tables[i].StartLine <= block.EndLine && tables[i].EndLine >= block.StartLine {
			indices = append(indices, i)
		}
	}
	return indices
}
