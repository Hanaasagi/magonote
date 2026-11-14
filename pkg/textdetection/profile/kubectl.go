package profile

import (
	"strings"

	"github.com/Hanaasagi/magonote/pkg/textdetection/entity"
)

type kubectlProfile struct{}

func (kubectlProfile) Name() string { return "kubectl" }
func (kubectlProfile) Match(command Command) bool {
	base := commandBase(command)
	return (base == "kubectl" || base == "k" || base == "oc") && len(command.Argv) > 1 && command.Argv[1] == "get"
}

func (kubectlProfile) Detect(block Block, ctx *Context) []entity.Candidate {
	resource := kubectlResource(block.Command.Argv)

	for _, tableIndex := range tablesInBlock(block, ctx.Tables) {
		for col := range ctx.Tables[tableIndex].Columns {
			column := &ctx.Tables[tableIndex].Columns[col]
			switch strings.ToUpper(column.Name) {
			case "NAME":
				column.Semantic = "k8s." + resource
			case "NAMESPACE":
				column.Semantic = "k8s.namespace"
			case "NODE":
				column.Semantic = "k8s.node"
			case "READY", "STATUS", "RESTARTS", "AGE":
				column.Weight = 0.4
			}
		}
	}
	return nil
}

func kubectlResource(argv []string) string {
	for i := 2; i < len(argv); i++ {
		arg := argv[i]
		if strings.HasPrefix(arg, "-") {
			switch arg {
			case "-o", "--output", "-l", "--selector", "--field-selector", "--sort-by", "-n", "--namespace", "--context", "--cluster", "--user", "--token", "--server", "--kubeconfig":
				if i+1 < len(argv) {
					i++
				}
			}
			continue
		}
		return normalizeResource(arg)
	}
	return "resource"
}

func normalizeResource(resource string) string {
	resources := map[string]string{
		"po": "pod", "pod": "pod", "pods": "pod",
		"deploy": "deployment", "deployment": "deployment", "deployments": "deployment",
		"svc": "service", "service": "service", "services": "service",
		"ns": "namespace", "namespace": "namespace", "namespaces": "namespace",
		"no": "node", "node": "node", "nodes": "node",
	}
	if normalized, ok := resources[strings.ToLower(resource)]; ok {
		return normalized
	}
	return strings.TrimSuffix(strings.ToLower(resource), "s")
}
