package profile

import (
	"reflect"
	"testing"

	"github.com/Hanaasagi/magonote/pkg/textdetection/screen"
)

func TestDetectPromptsAndCommands(t *testing.T) {
	input := "$ kubectl get pods\nuser@host:~/x$ docker ps\nroot@host:/tmp# git status\n目录 ❯ ls -alh\n[user@host]$ echo ok\n~/project> echo ok"
	prompts := DetectPrompts(screen.Parse(input), nil)
	if len(prompts) != 6 {
		t.Fatalf("got %d prompts: %#v", len(prompts), prompts)
	}
	wantArgv := [][]string{{"kubectl", "get", "pods"}, {"docker", "ps"}, {"git", "status"}, {"ls", "-alh"}, {"echo", "ok"}, {"echo", "ok"}}
	for i, want := range wantArgv {
		if !reflect.DeepEqual(prompts[i].Command.Argv, want) {
			t.Errorf("prompt %d argv = %#v, want %#v", i, prompts[i].Command.Argv, want)
		}
	}
	if prompts[0].PrefixEnd != 2 || prompts[3].PrefixEnd != 7 {
		t.Errorf("prefix columns = %d, %d", prompts[0].PrefixEnd, prompts[3].PrefixEnd)
	}
}

func TestPromptCommandParsing(t *testing.T) {
	prompts := DetectPrompts(screen.Parse(`$ sudo -u root env FOO="a b" kubectl get pods -o wide | head`), nil)
	if len(prompts) != 1 {
		t.Fatalf("prompts = %#v", prompts)
	}
	want := []string{"kubectl", "get", "pods", "-o", "wide"}
	if !reflect.DeepEqual(prompts[0].Command.Argv, want) {
		t.Fatalf("argv = %#v, want %#v", prompts[0].Command.Argv, want)
	}
}

func TestCustomPromptPattern(t *testing.T) {
	prompts := DetectPrompts(screen.Parse(">> docker ps"), []string{`^>> (?P<cmd>.+)$`, `[`})
	if len(prompts) != 1 || prompts[0].PrefixEnd != 3 || prompts[0].Command.Argv[0] != "docker" {
		t.Fatalf("custom prompt = %#v", prompts)
	}
}

func TestRejectsPromptFalsePositives(t *testing.T) {
	input := "-> foo\n# comment\n>>> print(\"x\")\na > b\nnot a prompt"
	if got := DetectPrompts(screen.Parse(input), nil); len(got) != 0 {
		t.Fatalf("false prompts = %#v", got)
	}
}

func TestCandidateFromBytesUsesSpanText(t *testing.T) {
	ctx := &Context{Screen: screen.Parse("éx")}
	candidate := candidateFromBytes(ctx, "custom", 0, 0, 1, 1, "test")
	if candidate.Text != "é" || candidate.Span.EndCol != 1 {
		t.Fatalf("candidate = %#v", candidate)
	}
}

func TestBlocksCoverCommandOutput(t *testing.T) {
	prompts := []Prompt{{Line: 0, Command: Command{Argv: []string{"docker", "ps"}}}, {Line: 3, Command: Command{Argv: []string{"git", "status"}}}}
	blocks := Blocks(prompts, 5)
	if len(blocks) != 2 || blocks[0].StartLine != 1 || blocks[0].EndLine != 2 || blocks[1].StartLine != 4 || blocks[1].EndLine != 4 {
		t.Fatalf("blocks = %#v", blocks)
	}
}
