package backup

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/leg100/etok/pkg/backup"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
)

// flags represents the backup provider flags
type flags interface {
	addToFlagSet(*pflag.FlagSet)
	createProvider(context.Context) (backup.Provider, error)
	validate() error
}

// flagMaker is a constructor for a flags obj
type flagMaker func() flags

// providerMap maps name of backup provider to a flags constructor
type providerMap struct {
	name  string
	maker flagMaker
}

// providers is the collection of provider mappings
type providerMaps []providerMap

var (
	// mappings is a singleton containing collection of provider mappings
	mappings = providerMaps{}

	// ErrInvalidConfig is wrapped within all errors from this pkg, and can be
	// used by downstream to identify errors
	ErrInvalidConfig = errors.New("invalid backup config")

	ErrInvalidProvider = fmt.Errorf("%w: invalid provider", ErrInvalidConfig)
)

func addProvider(name string, f flagMaker) {
	mappings = append(mappings, providerMap{name: name, maker: f})
}

// Config holds the flag configuration for all providers
type Config struct {
	providers       []string
	providerToFlags map[string]flags

	flagSet *pflag.FlagSet

	// Name of Selected provider
	Selected string
}

func NewConfig(optionalMappings ...providerMap) *Config {
	cfg := &Config{
		flagSet:         pflag.NewFlagSet("backup", pflag.ContinueOnError),
		providerToFlags: make(map[string]flags),
	}

	cfgMappings := mappings
	if len(optionalMappings) > 0 {
		cfgMappings = optionalMappings
	}

	for _, m := range cfgMappings {
		cfg.providers = append(cfg.providers, m.name)
		cfg.providerToFlags[m.name] = m.maker()
		cfg.providerToFlags[m.name].addToFlagSet(cfg.flagSet)
	}

	cfg.flagSet.StringVar(&cfg.Selected, "backup-provider", "", fmt.Sprintf("Enable backups specifying a provider (%v)", strings.Join(cfg.providers, ",")))

	return cfg
}

// AddToFlagSet adds config's (and its providers') flagsets to fs
func (c *Config) AddToFlagSet(fs *pflag.FlagSet) {
	fs.AddFlagSet(c.flagSet)
}

// GetEnvVars converts the complete flagset into a list of k8s environment
// variable objects. Assumes config's flagset is present within fs, and assumes
// fs has been parsed.
func (c *Config) GetEnvVars(fs *pflag.FlagSet) (envvars []corev1.EnvVar) {
	c.flagSet.VisitAll(func(flag *pflag.Flag) {
		// Only fs has been parsed so we need to get populated value from there
		if f := fs.Lookup(flag.Name); f != nil {
			envvars = append(envvars, corev1.EnvVar{Name: flagToEnvVarName(f), Value: f.Value.String()})
		}
	})
	return
}

// flagToEnvVarName converts flag f to an etok environment variable name
func flagToEnvVarName(f *pflag.Flag) string {
	return fmt.Sprintf("ETOK_%s", strings.Replace(strings.ToUpper(f.Name), "-", "_", -1))
}

func (c *Config) CreateSelectedProvider(ctx context.Context) (backup.Provider, error) {
	flags, ok := c.providerToFlags[c.Selected]
	if !ok {
		return nil, nil
	}
	return flags.createProvider(ctx)
}

// Validate all user-specified flags
func (c *Config) Validate(fs *pflag.FlagSet) error {
	if c.Selected == "" {
		return nil
	}
	flags, ok := c.providerToFlags[c.Selected]
	if !ok {
		return fmt.Errorf("%w: %s (valid providers: %s)", ErrInvalidProvider, c.Selected, strings.Join(c.providers, ","))
	}

	// Validate selected provider's flags
	return flags.validate()
}
