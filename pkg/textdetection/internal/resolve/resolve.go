package resolve

import (
	"sort"

	"github.com/Hanaasagi/magonote/pkg/textdetection/entity"
	"github.com/Hanaasagi/magonote/pkg/textdetection/screen"
)

// Candidates applies exclusions and greedy interval resolution. Stable sorting
// preserves detector registration order when all explicit rank keys tie.
func Candidates(input []entity.Candidate, exclusions []screen.Span) []entity.Entity {
	candidates := make([]entity.Candidate, 0, len(input))
	for _, candidate := range input {
		if candidate.Span.EndCol <= candidate.Span.StartCol || intersectsAny(candidate.Span, exclusions) {
			continue
		}
		candidates = append(candidates, candidate)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.Class != b.Class {
			return a.Class > b.Class
		}
		aLen, bLen := a.Span.EndCol-a.Span.StartCol, b.Span.EndCol-b.Span.StartCol
		if aLen != bLen {
			return aLen > bLen
		}
		return a.Confidence > b.Confidence
	})

	accepted := make([]entity.Entity, 0, len(candidates))
	for _, candidate := range candidates {
		overlaps := false
		for i, existing := range accepted {
			if spansIntersect(candidate.Span, existing.Span) {
				if candidate.Span == existing.Span {
					accepted[i].Meta = mergeCellMetadata(existing, candidate)
				}
				overlaps = true
				break
			}
		}
		if !overlaps {
			accepted = append(accepted, candidate)
		}
	}
	sort.SliceStable(accepted, func(i, j int) bool {
		if accepted[i].Span.Line != accepted[j].Span.Line {
			return accepted[i].Span.Line < accepted[j].Span.Line
		}
		if accepted[i].Span.StartCol != accepted[j].Span.StartCol {
			return accepted[i].Span.StartCol < accepted[j].Span.StartCol
		}
		return accepted[i].Span.EndCol < accepted[j].Span.EndCol
	})
	return accepted
}

func mergeCellMetadata(winner, loser entity.Candidate) map[string]string {
	if winner.Kind == entity.KindCell || loser.Kind != entity.KindCell {
		return winner.Meta
	}
	merged := make(map[string]string, len(winner.Meta)+len(loser.Meta))
	for key, value := range winner.Meta {
		merged[key] = value
	}
	for key, value := range loser.Meta {
		if _, exists := merged[key]; !exists && value != "" {
			merged[key] = value
		}
	}
	return merged
}

func intersectsAny(span screen.Span, exclusions []screen.Span) bool {
	for _, exclusion := range exclusions {
		if spansIntersect(span, exclusion) {
			return true
		}
	}
	return false
}

func spansIntersect(a, b screen.Span) bool {
	return a.Line == b.Line && a.StartCol < b.EndCol && b.StartCol < a.EndCol
}
