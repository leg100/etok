package flags

import "github.com/spf13/pflag"

// Check if user has passed a flag
func IsFlagPassed(fs *pflag.FlagSet, name string) (found bool) {
	fs.Visit(func(f *pflag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}
