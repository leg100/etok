package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/creasty/defaults"
	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/clientcreator"
	"github.com/leg100/stok/pkg/env"
	"github.com/leg100/stok/util"
	"github.com/leg100/stok/version"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

// Options pertaining to stok apps
type Options struct {
	// Selected app
	app App

	// CLI non-flag args
	Args []string

	// Name
	Name string

	// Run name
	RunName string `default:"-"`

	// Kubernetes namespace
	Namespace string `default:"default"`

	// Terraform workspace
	Workspace string `default:"default"`

	// Stok Workspace's WorkspaceSpec
	WorkspaceSpec v1alpha1.WorkspaceSpec

	// Terraform config path
	Path string `default:"."`

	// Timeout for workspace to be healthy
	TimeoutWorkspace time.Duration `default:"10s"`

	// Timeout for workspace pod to be running and ready
	TimeoutWorkspacePod time.Duration `default:"10s"`

	// Timeout for pod to be ready and running
	TimeoutPod time.Duration `default:"10s"`

	// Timeout for client to signal readiness
	TimeoutClient time.Duration `default:"10s"`

	// timeout waiting in workspace queue
	// flag name: timeout-queue
	TimeoutQueue time.Duration `default:"1h"`

	// TODO: rename to timeout-pending (enqueue is too similar sounding to queue)
	// timeout waiting to be queued
	// flag name: timeout-enqueue
	TimeoutEnqueue time.Duration `default:"10s"`

	// Stdout, Stderr writers
	// TODO: ErrOut might well now be redundant
	Out, ErrOut io.Writer

	// Toggle debug-level logging
	Debug bool

	// Path to local concatenated CRD schema
	LocalCRDPath string

	// Toggle reading CRDs from local file
	LocalCRDToggle bool

	// URL to concatenated CRD schema
	RemoteCRDURL string

	// Docker image used for both the operator and the runner
	Image string `default:"-"`

	// Kubernetes resource kind
	Kind string

	// Runner: toggle waiting for resource
	NoWait bool

	// Runner: Tarball filename
	Tarball string `default:"tarball.tar.gz"`

	// Stok run command
	Command string

	// Operator metrics bind endpoint
	MetricsAddress string

	// Toggle operator leader election
	EnableLeaderElection bool

	// Workspace name with optional namespace i.e. namespace/workspace
	StokEnv string

	// Create service account if it does not exist
	CreateServiceAccount bool

	// Create secret if it does not exist
	CreateSecret bool

	// Kubernetes context
	KubeContext string

	// Kubernetes config
	KubeConfig *rest.Config

	// Deferred creation of clients
	clientcreator.Interface
}

func (opts *Options) SetDefaults() {
	if defaults.CanUpdate(opts.Image) {
		opts.Image = version.Image
	}
	if defaults.CanUpdate(opts.RunName) {
		opts.RunName = fmt.Sprintf("run-%s", util.GenerateRandomString(5))
	}
}

func NewOpts() (*Options, error) {
	opts := &Options{Interface: clientcreator.NewClientCreator()}
	if err := defaults.Set(opts); err != nil {
		return nil, err
	}
	return opts, nil
}

func NewFakeOpts(out io.Writer, objs... runtime.Object) (*Options, error) {
	opts := &Options{
		Interface: clientcreator.NewFakeClientCreator(objs...),
		Out: out,
	}
	if err := defaults.Set(opts); err != nil {
		return nil, err
	}
	return opts, nil
}

func NewFakeOptsWithClients(out io.Writer, objs... runtime.Object) (*Options, error) {
	opts, err := NewFakeOpts(out, objs...)
	if err != nil {
		return nil, err
	}

	opts.CreateClients(opts.KubeContext)
	return opts, nil
}

// Construct app with its constructor newApp and save it for use in RunApp
func (opts *Options) SelectApp(newApp NewApp) {
	opts.app = newApp(opts)
}

// Run app if previously selected
func (opts *Options) RunApp(ctx context.Context) error {
	if opts.app != nil {
		return opts.app.Run(ctx)
	}
	return nil
}

// Set namespace and workspace from the environment file - if the file exists and the namespace
// and/or workspace have not been explicitly set via their respective flags
func (opts *Options) SetNamespaceAndWorkspaceFromEnv(fs *pflag.FlagSet) error {
	stokenv, err := env.ReadStokEnv(opts.Path)
	if err != nil {
		if os.IsNotExist(err) {
			// Fallback to values set from flags, whether user set or their defaults
			return nil
		}
		return err
	}

	if !isFlagPassed(fs, "namespace") {
		opts.Namespace = stokenv.Namespace()
	}

	if !isFlagPassed(fs, "workspace") {
		opts.Workspace = stokenv.Workspace()
	}

	return nil
}

// Check if user has passed a flag
func isFlagPassed(fs *pflag.FlagSet, name string) (found bool) {
	fs.Visit(func(f *pflag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}
