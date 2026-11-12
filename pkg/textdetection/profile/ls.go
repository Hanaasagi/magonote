package profile

import (
	"regexp"
	"strings"

	"github.com/Hanaasagi/magonote/pkg/textdetection/entity"
	"github.com/Hanaasagi/magonote/pkg/textdetection/screen"
)

type lsProfile struct{}

func (lsProfile) Name() string { return "ls" }
func (lsProfile) Match(command Command) bool {
	switch commandBase(command) {
	case "ls", "ll", "la", "eza", "exa", "lsd":
		return true
	}
	return false
}

var lsLongPattern = regexp.MustCompile(`^[-dlcbps][-rwxsStT]{9}[@+.]?(?:\s+\S+){7}\s+(.+)$`)
var lsPermissionsPattern = regexp.MustCompile(`^[-dlcbps][-rwxsStT]{9}[@+.]?$`)

func (lsProfile) Detect(block Block, ctx *Context) []entity.Candidate {
	var candidates []entity.Candidate
	longOutput := false
	for _, tableIndex := range tablesInBlock(block, ctx.Tables) {
		table := &ctx.Tables[tableIndex]
		isLong := len(table.Columns) > 0 && (table.Columns[0].Name == "Permissions" || len(table.Rows) > 0 && len(table.Rows[0].Cells) > 0 && lsPermissionsPattern.MatchString(table.Rows[0].Cells[0].Text))
		if !isLong {
			continue
		}
		longOutput = true
		last := len(table.Columns) - 1
		for rowIndex := range table.Rows {
			cell := table.Rows[rowIndex].Cells[last]
			if cell.Empty {
				continue
			}
			text, span := withoutSymlinkTarget(ctx, cell.Text, cell.Span)
			candidates = append(candidates, candidateFromSpan(entity.KindPath, text, span, 0.95, "ls"))
		}
		for col := range table.Columns[:last] {
			table.Columns[col].Weight = 0.3
		}
	}

	for line := max(0, block.StartLine); line <= min(block.EndLine, len(ctx.Screen.Lines)-1); line++ {
		plain := ctx.Screen.Lines[line].Plain
		if indices := lsLongPattern.FindStringSubmatchIndex(plain); indices != nil {
			longOutput = true
			span := ctx.Screen.SpanOf(line, indices[2], indices[3])
			text, span := withoutSymlinkTarget(ctx, plain[indices[2]:indices[3]], span)
			candidates = append(candidates, candidateFromSpan(entity.KindPath, text, span, 0.95, "ls"))
		}
	}
	if longOutput {
		return candidates
	}

	for line := max(0, block.StartLine); line <= min(block.EndLine, len(ctx.Screen.Lines)-1); line++ {
		if line >= len(ctx.Chunks) {
			continue
		}
		for _, chunk := range ctx.Chunks[line] {
			candidates = append(candidates, candidateFromBytes(ctx, entity.KindPath, line, chunk.ByteStart, chunk.ByteEnd, 0.8, "ls"))
		}
	}
	return candidates
}

func withoutSymlinkTarget(ctx *Context, text string, span screen.Span) (string, screen.Span) {
	if index := strings.Index(text, " -> "); index >= 0 {
		byteStart := ctx.Screen.ByteOf(span.Line, span.StartCol)
		return text[:index], ctx.Screen.SpanOf(span.Line, byteStart, byteStart+index)
	}
	return text, span
}
