package crdinfo

type CRDInfo struct {
	Name        string
	APISingular string
	APIPlural   string
	Entrypoint  []string
	ArgsHandler string
}

var Inventory = map[string]CRDInfo{
	"version": {
		Name:        "version",
		APISingular: "version",
		APIPlural:   "version",
		Entrypoint:  []string{"terraform", "version"},
		ArgsHandler: "DoubleDashArgsHandler",
	},
	"init": {
		Name:        "init",
		APISingular: "init",
		APIPlural:   "inits",
		Entrypoint:  []string{"terraform", "init"},
		ArgsHandler: "DoubleDashArgsHandler",
	},
	"plan": {
		Name:        "plan",
		APISingular: "plan",
		APIPlural:   "plans",
		Entrypoint:  []string{"terraform", "plan"},
		ArgsHandler: "DoubleDashArgsHandler",
	},
	"apply": {
		Name:        "apply",
		APISingular: "apply",
		APIPlural:   "apply",
		Entrypoint:  []string{"terraform", "apply"},
		ArgsHandler: "DoubleDashArgsHandler",
	},
	"force-unlock": {
		Name:        "force-unlock",
		APISingular: "forceunlock",
		APIPlural:   "forceunlocks",
		Entrypoint:  []string{"terraform", "force-unlock"},
		ArgsHandler: "DoubleDashArgsHandler",
	},
	"shell": {
		Name:        "shell",
		APISingular: "shell",
		APIPlural:   "shells",
		Entrypoint:  []string{"sh"},
		ArgsHandler: "ShellWrapDoubleDashArgsHandler",
	},
}
