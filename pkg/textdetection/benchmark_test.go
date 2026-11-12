package textdetection

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Hanaasagi/magonote/pkg/textdetection/profile"
	"github.com/Hanaasagi/magonote/pkg/textdetection/table"
)

func BenchmarkDetect(b *testing.B) {
	var input strings.Builder
	for line := 0; line < 60; line++ {
		text := fmt.Sprintf("2025-06-17T10:30:%02dZ 192.168.1.%d https://example.com/%d NAME%d  READY%d  /tmp/file%d.go", line%60, line%255, line, line, line, line)
		fmt.Fprintf(&input, "%-200s\n", text)
	}
	opts := Options{Table: &table.Options{}, Style: true, Profiles: &profile.Options{ExcludePrompt: true}}
	b.SetBytes(int64(input.Len()))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Detect(input.String(), opts); err != nil {
			b.Fatal(err)
		}
	}
}
