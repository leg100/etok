package options

import (
	"flag"
	"io"
	"os"
	"time"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/env"
	"github.com/leg100/stok/pkg/k8s/stokclient"
	"k8s.io/client-go/kubernetes"
)

// Options populated by CLI args
type StokOptions struct {
	// CLI non-flag args
	Args []string

	// Name
	Name string

	// Kubernetes namespace
	Namespace string

	// Terraform workspace
	Workspace string

	// Stok Workspace's WorkspaceSpec
	WorkspaceSpec v1alpha1.WorkspaceSpec

	// Terraform config path
	Path string

	// Timeout for workspace to be healthy
	TimeoutWorkspace time.Duration

	// Timeout for workspace pod to be running and ready
	TimeoutWorkspacePod time.Duration

	// Timeout for pod to be ready and running
	TimeoutPod time.Duration

	// Timeout for client to signal readiness
	TimeoutClient time.Duration

	// timeout waiting in workspace queue
	// default: 1h
	// flag name: timeout-queue
	TimeoutQueue time.Duration

	// TODO: rename to timeout-pending (enqueue is too similar sounding to queue)
	// timeout waiting to be queued
	// default: 10s
	// flag name: timeout-enqueue
	TimeoutEnqueue time.Duration

	// Kubernetes context
	Context string

	// Kubernetes built-in client
	KubeClient kubernetes.Interface

	// Stok generated client
	StokClient stokclient.Interface

	// Stdout, Stderr writers
	Out, ErrOut io.Writer

	// Toggle printing version
	Version bool

	// Toggle printing usage
	Help bool

	// Toggle debug-level logging
	Debug bool

	// Path to local concatenated CRD schema
	LocalCRDPath string

	// Toggle reading CRDs from local file
	LocalCRDToggle bool

	// URL to concatenated CRD schema
	RemoteCRDURL string

	// Docker image used for both the operator and the runner
	Image string

	// Kubernetes resource kind
	Kind string

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
}

func (opts *StokOptions) SetWorkspace(fs *flag.FlagSet) error {
	if isFlagPassed(fs, "workspace") {
		ws, _, err := env.ValidateAndParse(opts.StokEnv)
		if err != nil {
			return err
		}
		opts.Workspace = ws
		return nil
	}

	stokenv, err := env.ReadStokEnv(opts.Path)
	if err != nil {
		if !os.IsNotExist(err) {
			// It's ok for an environment file to not exist, but not any other error
			return err
		}
	}
	// Env file found
	opts.Workspace = stokenv.Workspace()

	return nil
}

func (opts *StokOptions) SetNamespace(fs *flag.FlagSet) error {
	if isFlagPassed(fs, "namespace") {
		_, ns, err := env.ValidateAndParse(opts.Workspace)
		if err != nil {
			return err
		}

		if ns != "" {
			opts.Namespace = ns
			return nil
		}
	}

	stokenv, err := env.ReadStokEnv(opts.Path)
	if err != nil {
		if !os.IsNotExist(err) {
			// It's ok for an environment file to not exist, but not any other error
			return err
		}
	}
	// Env file found
	opts.Namespace = stokenv.Namespace()

	return nil
}

// Check if user has passed a flag
func isFlagPassed(fs *flag.FlagSet, name string) (found bool) {
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}
