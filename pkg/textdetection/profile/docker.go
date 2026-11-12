package profile

import (
	"strings"

	"github.com/Hanaasagi/magonote/pkg/textdetection/entity"
)

type dockerProfile struct{}

func (dockerProfile) Name() string { return "docker" }
func (dockerProfile) Match(command Command) bool {
	base := commandBase(command)
	if base != "docker" && base != "podman" || len(command.Argv) < 2 {
		return false
	}
	argv := command.Argv
	return argv[1] == "ps" || argv[1] == "images" || len(argv) > 2 && (argv[1] == "container" || argv[1] == "image") && argv[2] == "ls"
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
