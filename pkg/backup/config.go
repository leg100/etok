package backup

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/spf13/pflag"
)

type flags interface {
	addToFlagSet(*pflag.FlagSet)
	createProvider(context.Context) (Provider, error)
	validate() error
}

var (
	providerToFlagsMaker = make(map[string]func() flags)

	ErrInvalidConfig = errors.New("invalid backup config")
)

// Config holds the configuration for all providers
type Config struct {
	providerToFlags map[string]flags

	flagSet *pflag.FlagSet

	// Name of selected provider
	selected string
}

func NewConfig() *Config {
	cfg := &Config{
		flagSet:         pflag.NewFlagSet("backup", pflag.ContinueOnError),
		providerToFlags: make(map[string]flags),
	}

	for p, fm := range providerToFlagsMaker {
		cfg.providerToFlags[p] = fm()
	}

	cfg.flagSet.StringVar(&cfg.selected, "backup-provider", "", fmt.Sprintf("Enable backups specifying a provider (%v)", strings.Join(cfg.providers(), ",")))

	return cfg
}

func (c *Config) AddToFlagSet(fs *pflag.FlagSet) {
	fs.AddFlagSet(c.flagSet)

	for _, f := range c.providerToFlags {
		f.addToFlagSet(fs)
	}
}

func (c *Config) GetEnvVars() []corev1.EnvVar {
	var envvars = []corev1.EnvVar{}
	c.flagSet.VisitAll(func(flag *pflag.Flag) {
		envvars = append(envvars, corev1.EnvVar{Name: flagToEnvVarName(flag), Value: flag.Value.String()})
	})
	return envvars
}

func flagToEnvVarName(f *pflag.Flag) string {
	return fmt.Sprintf("ETOK_%s", strings.Replace(strings.ToUpper(f.Name), "-", "_", -1))
}

func (c *Config) CreateSelectedProvider(ctx context.Context) (Provider, error) {
	flags, ok := c.providerToFlags[c.selected]
	if !ok {
		return nil, nil
	}
	return flags.createProvider(ctx)
}

// Validate all user-specified flags
func (c *Config) Validate() error {
	if c.selected == "" {
		return nil
	}
	flags, ok := c.providerToFlags[c.selected]
	if !ok {
		return fmt.Errorf("%w: invalid provider selected: %s (valid providers: %s)", ErrInvalidConfig, c.selected, strings.Join(c.providers(), ","))
	}

	// Validate selected provider's flags
	return flags.validate()
}

func (c *Config) providers() (providers []string) {
	for p := range c.providerToFlags {
		providers = append(providers, p)
	}
	return
}
