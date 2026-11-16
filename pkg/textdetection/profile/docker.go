package profile

import (
	"strings"

	"github.com/Hanaasagi/magonote/pkg/textdetection/entity"
)

type dockerProfile struct{}

func (dockerProfile) Name() string { return "docker" }
func (dockerProfile) Match(command Command) bool {
	base := commandBase(command)
	if base != "docker" && base != "podman" {
		return false
	}
	argv := dockerCommandArgs(command.Argv)
	if len(argv) == 0 {
		return false
	}
	return argv[0] == "ps" || argv[0] == "images" || len(argv) > 1 && (argv[0] == "container" || argv[0] == "image") && argv[1] == "ls"
}

func dockerCommandArgs(argv []string) []string {
	for i := 1; i < len(argv); {
		arg := argv[i]
		switch arg {
		case "--context", "--host", "--config", "--tlscacert", "--tlscert", "--tlskey":
			if i+1 >= len(argv) {
				return nil
			}
			i += 2
			continue
		case "--debug", "--tls", "--tlsverify":
			i++
			continue
		}
		if strings.HasPrefix(arg, "--context=") || strings.HasPrefix(arg, "--host=") || strings.HasPrefix(arg, "--config=") {
			i++
			continue
		}
		return argv[i:]
	}
	return nil
}

func (dockerProfile) Detect(block Block, ctx *Context) []entity.Candidate {
	for _, tableIndex := range tablesInBlock(block, ctx.Tables) {
		for col := range ctx.Tables[tableIndex].Columns {
			column := &ctx.Tables[tableIndex].Columns[col]
			switch strings.ToUpper(column.Name) {
			case "CONTAINER ID":
				column.Semantic = "docker.container"
			case "IMAGE":
				column.Semantic = "docker.image"
			case "NAMES":
				column.Semantic = "docker.container_name"
			case "IMAGE ID":
				column.Semantic = "docker.image_id"
			case "REPOSITORY":
				column.Semantic = "docker.repository"
			case "TAG":
				column.Semantic = "docker.tag"
			case "COMMAND", "CREATED", "STATUS":
				column.Weight = 0.4
			}
		}
	}
	return nil
}
