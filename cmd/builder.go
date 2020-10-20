package cmd

import (
	"context"

	"github.com/leg100/stok/cmd/flags"
	"github.com/leg100/stok/pkg/app"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Builder is used to build cobra commands.
type Builder interface {
	WithName(string) Builder
	WithShortHelp(string) Builder
	WithLongHelp(string) Builder
	WithFlags(...flags.DeferredFlag) Builder
	WithVersionFlag(func() string) Builder
	WithPersistentFlags(...flags.DeferredFlag) Builder
	WithOneArg() Builder
	WithHidden() Builder
	WantsKubeClients() Builder
	WithApp(app.NewApp) Builder
	WithPreExec(stokPreExec) Builder
	WithExec(stokExec) Builder
	Build(*app.Options, bool) *cobra.Command
	AddChild(Builder)
}

type builder struct {
	use, short, long                 string
	version                          func() string
	children                         []Builder
	ffuncs                           []flags.DeferredFlag
	pffuncs                          []flags.DeferredFlag
	validate                         cobra.PositionalArgs
	preExec                          stokPreExec
	exec                             stokExec
	wantsKubeClients, isRoot, hidden bool
	newApp                              app.NewApp
}

// cobra-style Exec
type cobraExec func(*cobra.Command, []string) error

// stok-style Exec
type stokExec func(context.Context, *app.Options) error

// stok-style PreExec
type stokPreExec func(*pflag.FlagSet, *app.Options) error

// NewCmd creates a new command builder.
func NewCmd(name string) Builder {
	b := &builder{}
	b.use = name
	return b
}

func (b *builder) WithName(name string) Builder {
	b.use = name
	return b
}

func (b *builder) WithShortHelp(help string) Builder {
	b.short = help
	return b
}

func (b *builder) WithLongHelp(help string) Builder {
	b.long = help
	return b
}

func (b *builder) WithHidden() Builder {
	b.hidden = true
	return b
}

func (b *builder) WithVersionFlag(f func() string) Builder {
	b.version = f
	return b
}

func (b *builder) WantsKubeClients() Builder {
	b.wantsKubeClients = true
	b.WithFlags(flags.KubeContext)
	return b
}

func (b *builder) WithPreExec(exec stokPreExec) Builder {
	b.preExec = exec
	return b
}

func (b *builder) WithExec(exec stokExec) Builder {
	b.exec = exec
	return b
}

func (b *builder) WithApp(newApp app.NewApp) Builder {
	b.newApp = newApp
	return b
}

func (b *builder) WithOneArg() Builder {
	b.validate = cobra.ExactArgs(1)
	return b
}

func (b *builder) Build(opts *app.Options, isRoot bool) *cobra.Command {
	b.isRoot = isRoot

	// Ensure we build a new command from afresh
	cmd := &cobra.Command{
		Use:   b.use,
		Short: b.short,
		Long:  b.long,
	}

	if b.version != nil {
		cmd.Version = b.version()
	}

	cmd.Hidden = b.hidden

	// Set dest for usage/error msgs
	cmd.SetOut(opts.Out)

	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	// Add persistent flags
	for _, pffunc := range b.pffuncs {
		pffunc(cmd.PersistentFlags(), opts)
	}

	// Add flags
	for _, ffunc := range b.ffuncs {
		ffunc(cmd.Flags(), opts)
	}

	// Add arg validation
	if b.validate != nil {
		cmd.Args = b.validate
	}

	// Add exec callback
	if b.exec != nil {
		cmd.RunE = b.execBuilder(opts, b.exec)
	}

	// Add stok app callback
	if b.newApp != nil {
		cmd.RunE = b.execBuilder(opts, func(ctx context.Context, opts *app.Options) error {
			opts.SelectApp(b.newApp)
			return nil
		})
	}

	// Recursively build child commands
	for _, child := range b.children {
		cmd.AddCommand(child.Build(opts, false))
	}

	// Set usage and help templates
	t := &templater{
		IsRoot:        isRoot,
		UsageTemplate: MainUsageTemplate(),
	}
	cmd.SetUsageFunc(t.UsageFunc())
	cmd.SetHelpTemplate(MainHelpTemplate())

	return cmd
}

func (b *builder) execBuilder(opts *app.Options, exec stokExec) cobraExec {
	return func(cmd *cobra.Command, args []string) error {
		// Make positional args available via App
		opts.Args = args

		// Invoke pre-exec callback
		if b.preExec != nil {
			b.preExec(cmd.Flags(), opts)
		}

		// Only create kube clients if app needs them
		if b.wantsKubeClients {
			if err := opts.CreateClients(opts.KubeContext); err != nil {
				return err
			}
		}

		return exec(cmd.Context(), opts)
	}
}

func (b *builder) AddChild(child Builder) {
	b.children = append(b.children, child)
}

func (b *builder) WithFlags(ffunc ...flags.DeferredFlag) Builder {
	b.ffuncs = append(b.ffuncs, ffunc...)
	return b
}

func (b *builder) WithPersistentFlags(pffuncs ...flags.DeferredFlag) Builder {
	b.pffuncs = append(b.pffuncs, pffuncs...)
	return b
}
