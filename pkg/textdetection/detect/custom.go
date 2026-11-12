package detect

import (
	"log/slog"
	"regexp"

	"github.com/Hanaasagi/magonote/pkg/textdetection/entity"
)

type customDetector struct{ patterns []string }

func NewCustomDetector(patterns []string) Detector { return &customDetector{patterns: patterns} }
func (d *customDetector) Name() string             { return "custom" }

func (d *customDetector) Detect(ctx *Context) []entity.Candidate {
	var candidates []entity.Candidate
	for _, pattern := range d.patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			slog.Warn("invalid custom pattern; skipping", "pattern", pattern, "error", err)
			continue
		}
		named := re.SubexpIndex("match")
		for line := range ctx.Screen.Lines {
			plain := ctx.Screen.Lines[line].Plain
			for _, indices := range re.FindAllStringSubmatchIndex(plain, -1) {
				matched := false
				if named > 0 {
					start, end := indices[2*named], indices[2*named+1]
					if start >= 0 && end > start {
						candidates = append(candidates, candidateFromBytes(ctx, entity.KindCustom, line, start, end, entity.ClassCustom, "custom", 1))
						matched = true
					}
				}
				if !matched {
					for group := 1; group < len(indices)/2; group++ {
						start, end := indices[2*group], indices[2*group+1]
						if start >= 0 && end > start {
							candidates = append(candidates, candidateFromBytes(ctx, entity.KindCustom, line, start, end, entity.ClassCustom, "custom", 1))
							matched = true
						}
					}
				}
				if !matched && indices[1] > indices[0] {
					candidates = append(candidates, candidateFromBytes(ctx, entity.KindCustom, line, indices[0], indices[1], entity.ClassCustom, "custom", 1))
				}
			}
		}
	}
	return candidates
}
