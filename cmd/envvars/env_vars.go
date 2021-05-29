package envvars

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Each flag can also be set with an env variable whose name starts with `ETOK_`.
func SetFlagsFromEnvVariables(cmd *cobra.Command) {
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		envVar := flagToEnvVarName(f)
		if val, present := os.LookupEnv(envVar); present {
			cmd.LocalFlags().Set(f.Name, val)
		}
	})
	for _, child := range cmd.Commands() {
		SetFlagsFromEnvVariables(child)
	}
}

// Unset env vars prefixed with `ETOK_`
func UnsetEtokVars() {
	for _, kv := range os.Environ() {
		parts := strings.Split(kv, "=")
		if strings.HasPrefix(parts[0], "ETOK_") {
			if err := os.Unsetenv(parts[0]); err != nil {
				panic(err.Error())
			}
		}
	}
}

func flagToEnvVarName(f *pflag.Flag) string {
	return fmt.Sprintf("ETOK_%s", strings.Replace(strings.ToUpper(f.Name), "-", "_", -1))
}
