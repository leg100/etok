package crdinfo

import (
	"github.com/iancoleman/strcase"
	"github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type CRDInfo struct {
	// CLI command
	Name string

	// Kind (templates take care of CamelCasing)
	Kind        string
	APISingular string
	APIPlural   string
	Entrypoint  []string
	ArgsHandler string
	StateOnly   bool

	//Override cobra command description
	Description string
}

func (crd *CRDInfo) GroupVersionResource() schema.GroupVersionResource {
	return v1alpha1.SchemeGroupVersion.WithResource(crd.APIPlural)
}

func (crd *CRDInfo) GroupVersionKind() schema.GroupVersionKind {
	return v1alpha1.SchemeGroupVersion.WithKind(strcase.ToCamel(crd.Kind))
}

func (crd *CRDInfo) GroupVersionKindList() schema.GroupVersionKind {
	return v1alpha1.SchemeGroupVersion.WithKind(strcase.ToCamel(crd.Kind) + "List")
}

var Inventory = map[string]CRDInfo{
	"apply": {
		Name:        "apply",
		Kind:        "apply",
		APISingular: "apply",
		APIPlural:   "applies",
		Entrypoint:  []string{"terraform", "apply"},
		ArgsHandler: "DoubleDashArgsHandler",
	},
	"destroy": {
		Name:        "destroy",
		Kind:        "destroy",
		APISingular: "destroy",
		APIPlural:   "destroys",
		Entrypoint:  []string{"terraform", "destroy"},
		ArgsHandler: "DoubleDashArgsHandler",
	},
	"force-unlock": {
		Name:        "force-unlock",
		Kind:        "force-unlock",
		APISingular: "forceunlock",
		APIPlural:   "forceunlocks",
		Entrypoint:  []string{"terraform", "force-unlock"},
		ArgsHandler: "DoubleDashArgsHandler",
	},
	"get": {
		Name:        "get",
		Kind:        "get",
		APISingular: "get",
		APIPlural:   "gets",
		Entrypoint:  []string{"terraform", "get"},
		ArgsHandler: "DoubleDashArgsHandler",
	},
	"import": {
		Name:        "import",
		Kind:        "imp",
		APISingular: "imp",
		APIPlural:   "imps",
		Entrypoint:  []string{"terraform", "import"},
		ArgsHandler: "DoubleDashArgsHandler",
		StateOnly:   true,
	},
	"init": {
		Name:        "init",
		Kind:        "init",
		APISingular: "init",
		APIPlural:   "inits",
		Entrypoint:  []string{"terraform", "init"},
		ArgsHandler: "DoubleDashArgsHandler",
	},
	"output": {
		Name:        "output",
		Kind:        "output",
		APISingular: "output",
		APIPlural:   "outputs",
		Entrypoint:  []string{"terraform", "output"},
		ArgsHandler: "DoubleDashArgsHandler",
		StateOnly:   true,
	},
	"plan": {
		Name:        "plan",
		Kind:        "plan",
		APISingular: "plan",
		APIPlural:   "plans",
		Entrypoint:  []string{"terraform", "plan"},
		ArgsHandler: "DoubleDashArgsHandler",
	},
	"refresh": {
		Name:        "refresh",
		Kind:        "refresh",
		APISingular: "refresh",
		APIPlural:   "refreshs",
		Entrypoint:  []string{"terraform", "refresh"},
		ArgsHandler: "DoubleDashArgsHandler",
	},
	"shell": {
		Name:        "shell",
		Kind:        "shell",
		APISingular: "shell",
		APIPlural:   "shells",
		Entrypoint:  []string{"sh"},
		ArgsHandler: "ShellWrapDoubleDashArgsHandler",
		Description: "Run interactive shell on workspace pod",
	},
	"show": {
		Name:        "show",
		Kind:        "show",
		APISingular: "show",
		APIPlural:   "shows",
		Entrypoint:  []string{"terraform", "show"},
		ArgsHandler: "DoubleDashArgsHandler",
		StateOnly:   true,
	},
	"state": {
		Name:        "state",
		Kind:        "state",
		APISingular: "state",
		APIPlural:   "states",
		Entrypoint:  []string{"terraform", "state"},
		ArgsHandler: "DoubleDashArgsHandler",
		StateOnly:   true,
	},
	"taint": {
		Name:        "taint",
		Kind:        "taint",
		APISingular: "taint",
		APIPlural:   "taints",
		Entrypoint:  []string{"terraform", "taint"},
		ArgsHandler: "DoubleDashArgsHandler",
		StateOnly:   true,
	},
	"untaint": {
		Name:        "untaint",
		Kind:        "untaint",
		APISingular: "untaint",
		APIPlural:   "untaints",
		Entrypoint:  []string{"terraform", "untaint"},
		ArgsHandler: "DoubleDashArgsHandler",
		StateOnly:   true,
	},
	"validate": {
		Name:        "validate",
		Kind:        "validate",
		APISingular: "validate",
		APIPlural:   "validates",
		Entrypoint:  []string{"terraform", "validate"},
		ArgsHandler: "DoubleDashArgsHandler",
	},
}
