package detect

import (
	"testing"

	"github.com/Hanaasagi/magonote/pkg/textdetection/entity"
	"github.com/Hanaasagi/magonote/pkg/textdetection/screen"
)

func testContext(plain string) *Context {
	s := screen.Parse(plain)
	return &Context{Screen: s}
}

func TestValidEmailRejectsLeadingSpecialChars(t *testing.T) {
	ctx := testContext(".user@example.com")
	c := entity.Candidate{Text: ".user@example.com"}
	if _, ok := validEmail(ctx, 0, 0, 17, c); ok {
		t.Error("leading dot should be rejected")
	}
	c.Text = "+user@example.com"
	if _, ok := validEmail(ctx, 0, 0, 17, c); ok {
		t.Error("leading plus should be rejected")
	}
	c.Text = "-user@example.com"
	if _, ok := validEmail(ctx, 0, 0, 17, c); ok {
		t.Error("leading hyphen should be rejected")
	}
}

func TestValidEmailRejectsBadTLD(t *testing.T) {
	ctx := testContext("user@host.x")
	c := entity.Candidate{Text: "user@host.x"}
	if _, ok := validEmail(ctx, 0, 0, 11, c); ok {
		t.Error("1-char TLD should be rejected")
	}
	ctx = testContext("user@host.thisiswaytoolongtobearealtld")
	c.Text = "user@host.thisiswaytoolongtobearealtld"
	if _, ok := validEmail(ctx, 0, 0, len(c.Text), c); ok {
		t.Error("TLD > 12 chars should be rejected")
	}
}

func TestValidEmailAcceptsNormal(t *testing.T) {
	ctx := testContext("user@example.com")
	c := entity.Candidate{Text: "user@example.com"}
	if _, ok := validEmail(ctx, 0, 0, 16, c); !ok {
		t.Error("normal email should be accepted")
	}
}

func TestValidUUIDAcceptsMixedCase(t *testing.T) {
	ctx := testContext("550e8400-E29B-41d4-a716-446655440000")
	c := entity.Candidate{Text: "550e8400-E29B-41d4-a716-446655440000"}
	if _, ok := validUUID(ctx, 0, 0, 36, c); !ok {
		t.Error("mixed-case UUID should be accepted")
	}
}

func TestFileLocationColumnLeniency(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		line   string
		col    string
		wantOK bool
		wantL  int
		wantC  int
	}{
		{"valid all", "f.go", "10", "4", true, 10, 4},
		{"invalid col zero", "f.go", "10", "0", true, 10, 0},
		{"invalid col text", "f.go", "10", "abc", true, 10, 0},
		{"invalid line zero", "f.go", "0", "", false, 0, 0},
		{"no line no col", "f.go", "", "", true, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loc := fileLocation(tt.path, tt.line, tt.col)
			if tt.wantOK && loc == nil {
				t.Fatal("expected non-nil Location")
			}
			if !tt.wantOK && loc != nil {
				t.Fatal("expected nil Location")
			}
			if loc != nil {
				if loc.Line != tt.wantL {
					t.Errorf("Line = %d, want %d", loc.Line, tt.wantL)
				}
				if loc.Column != tt.wantC {
					t.Errorf("Column = %d, want %d", loc.Column, tt.wantC)
				}
			}
		})
	}
}

func TestUniformHexCaseForSHA(t *testing.T) {
	if !uniformHexCase("abcdef1234567890") {
		t.Error("all lowercase should pass")
	}
	if !uniformHexCase("ABCDEF1234567890") {
		t.Error("all uppercase should pass")
	}
	if uniformHexCase("abcdEF1234567890") {
		t.Error("mixed case should fail for SHA")
	}
}
