package detect

import (
	"strings"

	"github.com/Hanaasagi/magonote/pkg/textdetection/entity"
)

type cellDetector struct{}

func NewCellDetector() Detector   { return cellDetector{} }
func (cellDetector) Name() string { return "cell" }

func (cellDetector) Detect(ctx *Context) []entity.Candidate {
	var candidates []entity.Candidate
	for _, table := range ctx.Tables {
		for _, row := range table.Rows {
			for _, cell := range row.Cells {
				if cell.Empty || cell.Span.EndCol-cell.Span.StartCol < 2 || !hasLetterOrDigit(cell.Text) || len(strings.Fields(cell.Text)) > 6 {
					continue
				}
				column := table.Columns[cell.Col]
				confidence := 0.8 * column.Weight
				meta := map[string]string{"column": column.Name}
				if column.Semantic != "" {
					meta["semantic"] = column.Semantic
				}
				if row.Line == table.HeaderLine {
					confidence = 0.5 * column.Weight
					meta["header"] = "true"
				}
				candidates = append(candidates, entity.Candidate{
					Kind: entity.KindCell, Text: cell.Text, Span: cell.Span, Confidence: confidence,
					Class: entity.ClassStructure, Source: "cell", Meta: meta,
				})
			}
		}
	}
	return candidates
}
