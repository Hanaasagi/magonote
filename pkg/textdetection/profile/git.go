package profile

import (
	"regexp"
	"strings"

	"github.com/Hanaasagi/magonote/pkg/textdetection/entity"
)

type gitProfile struct{}

func (gitProfile) Name() string { return "git" }
func (gitProfile) Match(command Command) bool {
	subcommand, _ := gitSubcommand(command.Argv)
	return commandBase(command) == "git" && (subcommand == "status" || subcommand == "gst" || subcommand == "branch")
}

var gitLongStatusPattern = regexp.MustCompile(`^\s+(?:modified|new file|deleted|renamed|copied|typechange|both modified|added by (?:us|them)|deleted by (?:us|them)):\s+(.+?)(?: -> (.+))?$`)
var gitShortStatusPattern = regexp.MustCompile(`^[ MADRCU?!]{2} (.+?)(?: -> (.+))?$`)
var gitBranchPattern = regexp.MustCompile(`^[* ] (\S+)$`)

func (gitProfile) Detect(block Block, ctx *Context) []entity.Candidate {
	subcommand, args := gitSubcommand(block.Command.Argv)
	if subcommand == "branch" {
		return gitBranches(block, ctx)
	}
	short := false
	for _, arg := range args {
		short = short || arg == "-s" || arg == "--short" || strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && strings.Contains(arg[1:], "s")
	}
	var candidates []entity.Candidate
	untracked := false
	for line := max(0, block.StartLine); line <= min(block.EndLine, len(ctx.Screen.Lines)-1); line++ {
		plain := ctx.Screen.Lines[line].Plain
		trimmed := strings.TrimSpace(plain)
		if trimmed == "Untracked files:" {
			untracked = true
			continue
		}
		pattern := gitLongStatusPattern
		if short {
			pattern = gitShortStatusPattern
		}
		indices := pattern.FindStringSubmatchIndex(plain)
		if indices != nil {
			candidates = append(candidates, gitPathCandidates(ctx, line, indices)...)
			continue
		}
		if untracked && len(plain) > 0 && (plain[0] == ' ' || plain[0] == '\t') && trimmed != "" && !strings.HasPrefix(trimmed, "(use ") {
			start := strings.Index(plain, trimmed)
			candidates = append(candidates, candidateFromBytes(ctx, entity.KindPath, line, start, start+len(trimmed), 0.95, "git"))
		} else if trimmed != "" && !strings.HasPrefix(trimmed, "(use ") {
			untracked = false
		}
	}
	return candidates
}

func gitSubcommand(argv []string) (string, []string) {
	for i := 1; i < len(argv); {
		arg := argv[i]
		switch arg {
		case "-C", "-c", "--git-dir", "--work-tree", "--namespace", "--exec-path", "--config-env":
			i += 2
			continue
		case "--no-pager", "-P", "--no-replace-objects", "--bare", "--no-optional-locks":
			i++
			continue
		}
		if strings.HasPrefix(arg, "--git-dir=") || strings.HasPrefix(arg, "--work-tree=") || strings.HasPrefix(arg, "--namespace=") || strings.HasPrefix(arg, "--config-env=") {
			i++
			continue
		}
		return arg, argv[i+1:]
	}
	return "", nil
}

func gitPathCandidates(ctx *Context, line int, indices []int) []entity.Candidate {
	var candidates []entity.Candidate
	for group := 1; group <= 2; group++ {
		start, end := indices[2*group], indices[2*group+1]
		if start >= 0 && end > start {
			candidates = append(candidates, candidateFromBytes(ctx, entity.KindPath, line, start, end, 0.95, "git"))
		}
	}
	return candidates
}

func gitBranches(block Block, ctx *Context) []entity.Candidate {
	var candidates []entity.Candidate
	for line := max(0, block.StartLine); line <= min(block.EndLine, len(ctx.Screen.Lines)-1); line++ {
		indices := gitBranchPattern.FindStringSubmatchIndex(ctx.Screen.Lines[line].Plain)
		if indices != nil {
			candidates = append(candidates, candidateFromBytes(ctx, entity.KindBranch, line, indices[2], indices[3], 0.95, "git"))
		}
	}
	return candidates
}
