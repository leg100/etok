package run

import (
	"fmt"
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
	"sh",
	"show",
	"state",
	"taint",
	"untaint",
	"validate",
}

// Generate name for cmd resource. The real program sets suffix to a random string, whereas the
// tests set it to something known ahead of time.
func GenerateName(suffix string) string {
	return fmt.Sprintf("%s-%s", "run", suffix)
}
