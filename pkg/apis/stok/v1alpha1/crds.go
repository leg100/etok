package v1alpha1

import (
	"fmt"
	"strings"

	"github.com/iancoleman/strcase"
	"k8s.io/api/node/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var Commands = []CmdCRD{
	CmdCRD("apply"),
	CmdCRD("destroy"),
	CmdCRD("force-unlock"),
	CmdCRD("get"),
	CmdCRD("import"),
	CmdCRD("init"),
	CmdCRD("output"),
	CmdCRD("plan"),
	CmdCRD("refresh"),
	CmdCRD("plan"),
	CmdCRD("shell"),
	CmdCRD("show"),
	CmdCRD("taint"),
	CmdCRD("untaint"),
	CmdCRD("validate"),
}

type CmdCRD string

func (crd CmdCRD) Plural() string { return strings.ReplaceAll(string(crd), "-", "") + "s" }

func (crd CmdCRD) Kind() string { return strcase.ToCamel(string(crd)) }

func (crd CmdCRD) GroupVersionResource() schema.GroupVersionResource {
	return v1alpha1.SchemeGroupVersion.WithResource(crd.Plural())
}

func (crd CmdCRD) GroupVersionKind() schema.GroupVersionKind {
	return v1alpha1.SchemeGroupVersion.WithKind(crd.Kind())
}

func (crd CmdCRD) GroupVersionKindList() schema.GroupVersionKind {
	return v1alpha1.SchemeGroupVersion.WithKind(crd.Kind() + "List")
}

func (crd CmdCRD) Wrapper(args []string) []string {
	switch crd {
	case "shell":
		if len(args) > 0 {
			cmdStr := fmt.Sprintf("\"%s\"", strings.Join(args, " "))
			return []string{"sh", "-c", cmdStr}
		} else {
			return []string{"sh"}
		}
	default:
		return append([]string{"terraform", string(crd)}, args...)
	}
}
