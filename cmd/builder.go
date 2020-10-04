/*
Copyright 2019 The Skaffold Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"context"
	"flag"
	"fmt"

	"github.com/leg100/stok/cmd/flags"
	"github.com/leg100/stok/pkg/apps"
	"github.com/leg100/stok/pkg/k8s/stokclient"
	"github.com/leg100/stok/pkg/options"
	ff "github.com/peterbourgon/ff/v3"
	"github.com/peterbourgon/ff/v3/ffcli"
	"k8s.io/client-go/kubernetes"
)

// Builder is used to build ffcli commands.
type Builder interface {
	WithName(string) Builder
	WithShortUsage(string) Builder
	WithShortHelp(string) Builder
	WithLongHelp(string) Builder
	WithEnvVars() Builder
	WithFlags(...func(*flag.FlagSet, *options.StokOptions)) Builder
	WithOneArg() Builder
	WantsKubeClients() Builder
	WithApp(apps.NewFunc) Builder
	WithPreExec(stokPreExec) Builder
	WithExec(stokExec) Builder
	Build(*options.StokOptions, clientCreatorFunc) *ffcli.Command
	AddChild(Builder)
}

type builder struct {
	cmd              *ffcli.Command
	children         []Builder
	ffuncs           []func(*flag.FlagSet, *options.StokOptions)
	validate         func([]string) error
	preExec          stokPreExec
	exec             stokExec
	wantsKubeClients bool

	newApp apps.NewFunc
}

// ffcli-style Exec
type ffcliExec func(context.Context, []string) error

// stok-style Exec
type stokExec func(context.Context, *options.StokOptions) error

// stok-style PreExec
type stokPreExec func(*flag.FlagSet, *options.StokOptions) error

// Kubernetes clients factory (for testing)
type clientCreatorFunc func(string) (stokclient.Interface, kubernetes.Interface, error)

// NewCmd creates a new command builder.
func NewCmd(name string) Builder {
	return &builder{
		cmd: &ffcli.Command{
			Name: name,
		},
	}
}

func (b *builder) WithName(name string) Builder {
	b.cmd.Name = name
	return b
}

func (b *builder) WithShortUsage(usage string) Builder {
	b.cmd.ShortUsage = usage
	return b
}

func (b *builder) WithShortHelp(help string) Builder {
	b.cmd.ShortHelp = help
	return b
}

func (b *builder) WithLongHelp(help string) Builder {
	b.cmd.LongHelp = help
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

func (b *builder) WithApp(newApp apps.NewFunc) Builder {
	b.newApp = newApp
	return b
}

func (b *builder) WithOneArg() Builder {
	b.validate = func(args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("expected one argument")
		}
		return nil
	}
	return b
}

func (b *builder) Build(opts *options.StokOptions, clientCreator clientCreatorFunc) *ffcli.Command {
	// Create new flagset for command and set dest for usage/error msgs
	b.cmd.FlagSet = flag.NewFlagSet(b.cmd.Name, flag.ContinueOnError)
	b.cmd.FlagSet.SetOutput(opts.ErrOut)

	// Add specified flags to flagset
	for _, ffunc := range b.ffuncs {
		ffunc(b.cmd.FlagSet, opts)
	}

	// Add common flags to flagset
	flags.Common(b.cmd.FlagSet, opts)

	if b.exec != nil {
		b.cmd.Exec = b.execBuilder(opts, b.exec, clientCreator)
	}

	if b.newApp != nil {
		b.cmd.Exec = b.execBuilder(opts, func(ctx context.Context, opts *options.StokOptions) error {
			var err error
			// Sets pkg-level variable 'app'
			app, err = b.newApp(ctx, opts)
			return err
		}, clientCreator)
	}

	// Recursively build child commands
	for _, child := range b.children {
		b.cmd.Subcommands = append(b.cmd.Subcommands, child.Build(opts, clientCreator))
	}

	return b.cmd
}

func (b *builder) execBuilder(opts *options.StokOptions, f stokExec, clients clientCreatorFunc) ffcliExec {
	return func(ctx context.Context, args []string) error {
		if opts.Help {
			fmt.Fprintf(opts.Out, ffcli.DefaultUsageFunc(b.cmd))
			return nil
		}

		if b.validate != nil {
			if err := b.validate(args); err != nil {
				return err
			}
		}

		// Make positional args available via StokOptions
		opts.Args = args

		if b.wantsKubeClients {
			sc, kc, err := clients(opts.Context)
			if err != nil {
				return err
			}
			opts.KubeClient = kc
			opts.StokClient = sc
		}

		return f(ctx, opts)
	}
}

func (b *builder) AddChild(child Builder) {
	b.children = append(b.children, child)
}

func (b *builder) WithEnvVars() Builder {
	b.cmd.Options = []ff.Option{ff.WithEnvVarPrefix("STOK")}
	return b
}

func (b *builder) WithFlags(ffunc ...func(*flag.FlagSet, *options.StokOptions)) Builder {
	b.ffuncs = append(b.ffuncs, ffunc...)
	return b
}
