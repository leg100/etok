package command

import (
	"fmt"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/leg100/stok/util"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var CommandKinds = []string{
	"Apply",
	"Destroy",
	"ForceUnlock",
	"Get",
	"Import",
	"Init",
	"Output",
	"Plan",
	"Refresh",
	"Shell",
	"Show",
	"State",
	"Taint",
	"Untaint",
	"Validate",
}

func NewCommandFromGVK(scheme *runtime.Scheme, gvk schema.GroupVersionKind) (Interface, error) {
	obj, err := scheme.New(gvk)
	if err != nil {
		return nil, err
	}
	return obj.(Interface), nil
}

// Convert k8s kind to stok CLI arg (e.g. Plan -> plan)
func CommandKindToCLI(kind string) string {
	return strcase.ToKebab(kind)
}

// Convert k8s kind to k8s API resource type (e.g. Plan -> plans)
func CommandKindToType(kind string) string {
	return strings.ToLower(kind) + "s"
}

// Convert stok CLI arg to k8s kind (e.g. plan -> Plan)
func CommandCLIToKind(cli string) string {
	return strcase.ToCamel(cli)
}

// Convert stok CLI arg to k8s resource type (e.g. plan -> plans)
func CommandCLIToType(cli string) string {
	return CommandKindToType(CommandCLIToKind(cli))
}

func CollectionKind(kind string) string {
	return kind + "List"
}

// A (naive) implementation of the algorithm that k8s uses to generate a unique name on the
// server side when `generateName` is specified. Allows us to generate a unique name client-side
// for our k8s resources.
func GenerateName(kind string) string {
	return fmt.Sprintf("%s-%s-%s", "stok", CommandKindToCLI(kind), util.GenerateRandomString(5))
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
		return append([]string{"terraform", CommandKindToCLI(kind)}, args...)
	}
}
