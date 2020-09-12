package run

import (
	"fmt"
	"strings"

	"github.com/iancoleman/strcase"
)

var TerraformCommands = []string{
	"apply",
	"destroy",
	"force-unlock",
	"get",
	"import",
	"init",
	"output",
	"plan",
	"refresh",
	"shell",
	"show",
	"state",
	"taint",
	"untaint",
	"validate",
}

// Convert k8s kind to stok CLI arg (e.g. Plan -> plan)
func RunKindToCLI(kind string) string {
	return strcase.ToKebab(kind)
}

// Convert k8s kind to k8s API resource type (e.g. Plan -> plans)
func RunKindToType(kind string) string {
	return strings.ToLower(kind) + "s"
}

// Convert stok CLI arg to k8s kind (e.g. plan -> Plan)
func RunCLIToKind(cli string) string {
	return strcase.ToCamel(cli)
}

// Convert stok CLI arg to k8s resource type (e.g. plan -> plans)
func RunCLIToType(cli string) string {
	return RunKindToType(RunCLIToKind(cli))
}

func CollectionKind(kind string) string {
	return kind + "List"
}

// Generate name for cmd resource. The real program sets suffix to a random string, whereas the
// tests set it to something known ahead of time.
func GenerateName(kind, suffix string) string {
	return fmt.Sprintf("%s-%s-%s", "stok", RunKindToCLI(kind), suffix)
}

// For a given k8s kind and user supplied args, return the named program and args to be executed on
// the pod (e.g. plan -- -input=false -> terraform plan -input=false
func RunnerArgsForKind(kind string, args []string) []string {
	switch kind {
	case "Shell":
		// Wrap shell args into a single command string
		if len(args) > 0 {
			return []string{"/bin/sh", "-c", strings.Join(args, " ")}
		} else {
			return []string{"/bin/sh"}
		}
	case "Workspace":
		return append([]string{"terraform", "init"}, args...)
	default:
		// All other kinds are run as a terraform command, and the stok CLI name translates directly
		// to the terraform command name (e.g. stok plan -> terraform plan)
		return append([]string{"terraform", RunKindToCLI(kind)}, args...)
	}
}
