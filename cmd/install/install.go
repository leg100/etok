package install

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/leg100/etok/cmd/flags"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/labels"
	"github.com/leg100/etok/pkg/version"
	"github.com/spf13/cobra"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	defaultNamespace = "etok"
)

var (
	// The URL of the repo from which certain resources will be retrieved (CRDs,
	// cluster role).
	repoURL = "https://raw.githubusercontent.com/leg100/etok/v" + version.Version

	// Relative paths to CRDs to be installed. Paths relative to the root of the
	// repo.
	crdPaths = []string{
		"config/crd/bases/etok.dev_workspaces.yaml",
		"config/crd/bases/etok.dev_runs.yaml",
	}
	// Relative paths to the cluster role resource to be installed. Path
	// relative to the root of the repo.
	clusterRolePath = "config/rbac/role.yaml"

	// Interval between polling deployment status
	interval = time.Second
)

type installOptions struct {
	*cmdutil.Factory

	*client.Client

	name        string
	namespace   string
	image       string
	kubeContext string

	// Path on local fs containing GCP service account key
	secretFile string

	// Toggle reading resources from local files rather than a URL
	local bool

	// Toggle waiting for deployment to be ready
	wait bool
	// Time to wait for
	timeout time.Duration

	// Print out resources and don't install
	dryRun bool
}

func InstallCmd(f *cmdutil.Factory) (*cobra.Command, *installOptions) {
	o := &installOptions{
		Factory:   f,
		namespace: defaultNamespace,
	}

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install etok operator",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			o.Client, err = o.CreateRuntimeClient(o.kubeContext)
			if err != nil {
				return err
			}

			return o.install(cmd.Context())
		},
	}

	flags.AddNamespaceFlag(cmd, &o.namespace)
	flags.AddKubeContextFlag(cmd, &o.kubeContext)

	cmd.Flags().StringVar(&o.name, "name", "etok-operator", "Name for kubernetes resources")
	cmd.Flags().StringVar(&o.image, "image", version.Image, "Docker image used for both the operator and the runner")

	cmd.Flags().BoolVar(&o.local, "local", false, "Read resources from local files (default false)")
	cmd.Flags().BoolVar(&o.dryRun, "dry-run", false, "Don't install resources just print out them in YAML format")
	cmd.Flags().BoolVar(&o.wait, "wait", true, "Toggle waiting for deployment to be ready")
	cmd.Flags().DurationVar(&o.timeout, "timeout", 60*time.Second, "Timeout for waiting for deployment to be ready")

	cmd.Flags().StringVar(&o.secretFile, "secret-file", "", "Path on local filesystem to key file")

	return cmd, o
}

func (o *installOptions) install(ctx context.Context) error {
	var resources []runtimeclient.Object

	for _, path := range crdPaths {
		res, err := o.crd(path)
		if err != nil {
			return err
		}
		resources = append(resources, res)
	}

	clusterRoleResource, err := o.clusterRole()
	if err != nil {
		return err
	}
	resources = append(resources, clusterRoleResource)

	resources = append(resources, clusterRoleBinding(o.namespace))
	resources = append(resources, namespace(o.namespace))
	resources = append(resources, serviceAccount(o.namespace, nil))

	secretPresent := o.secretFile != ""
	deploy := deployment(o.namespace, WithSecret(secretPresent))
	resources = append(resources, deploy)

	if o.secretFile != "" {
		key, err := ioutil.ReadFile(o.secretFile)
		if err != nil {
			return err
		}

		resources = append(resources, secret(o.namespace, key))
	}

	// Set labels
	for _, r := range resources {
		labels.SetCommonLabels(r)
		labels.SetLabel(r, labels.OperatorComponent)
	}

	if o.dryRun {
		// Print out YAML representation
		var docs []string
		for _, r := range resources {
			data, err := yaml.Marshal(r)
			if err != nil {
				return err
			}
			docs = append(docs, string(data))
		}
		fmt.Fprintf(o.Out, strings.Join(docs, "---\n"))

		// Don't install resources
		return nil
	}

	if err := o.createOrUpdate(ctx, resources); err != nil {
		return err
	}

	if o.wait {
		fmt.Fprintf(o.Out, "Waiting for Deployment to be ready\n")
		if err := o.deploymentIsReady(ctx, deploy); err != nil {
			return fmt.Errorf("failure while waiting for deployment to be ready: %w", err)
		}
	}

	return nil
}

// createOrUpdate idempotently installs resources, creating the resource if it
// doesn't already exist, otherwise updating it.
func (o *installOptions) createOrUpdate(ctx context.Context, resources []runtimeclient.Object) (err error) {
	for _, res := range resources {
		existing := res.DeepCopyObject().(runtimeclient.Object)

		err := o.RuntimeClient.Get(ctx, runtimeclient.ObjectKeyFromObject(res), existing)
		switch {
		case kerrors.IsNotFound(err):
			fmt.Fprintf(o.Out, "Creating resource %s %s\n", res.GetObjectKind().GroupVersionKind().Kind, klog.KObj(res))
			if err := o.RuntimeClient.Create(ctx, res); err != nil {
				return fmt.Errorf("unable to create resource: %w", err)
			}
		case err != nil:
			return err
		default:
			res.SetResourceVersion(existing.GetResourceVersion())
			fmt.Fprintf(o.Out, "Updating resource %s %s\n", res.GetObjectKind().GroupVersionKind().Kind, klog.KObj(res))
			if err := o.RuntimeClient.Update(ctx, res); err != nil {
				return fmt.Errorf("unable to update existing resource: %w", err)
			}
		}

	}

	return nil
}

// DeploymentIsReady will poll the kubernetes API server to see if the velero
// deployment is ready to service user requests.
func (o *installOptions) deploymentIsReady(ctx context.Context, deploy *appsv1.Deployment) error {
	var readyObservations int32
	return wait.PollImmediate(interval, o.timeout, func() (bool, error) {
		if err := o.RuntimeClient.Get(ctx, runtimeclient.ObjectKeyFromObject(deploy), deploy); err != nil {
			return false, err
		}

		for _, cond := range deploy.Status.Conditions {
			if isAvailable(cond) {
				readyObservations++
			}
		}
		// Make sure we query the deployment enough times to see the state change, provided there is one.
		if readyObservations > 4 {
			return true, nil
		} else {
			return false, nil
		}
	})
}

// CRDs. Unlike most other resources this is read from a YAML file from the
// repo, which in turn is installed with `make manifests`. Can also be read from
// a URL.
func (o *installOptions) crd(path string) (*apiextv1.CustomResourceDefinition, error) {
	data, err := getLocalOrRemoteDoc(o.local, path, repoURL)
	if err != nil {
		return nil, err
	}

	var obj apiextv1.CustomResourceDefinition
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	obj.Status = apiextv1.CustomResourceDefinitionStatus{}
	return &obj, nil
}

// Operator's ClusterRole. Unlike most other resources this is read from a YAML
// file from the repo, which in turn is installed with `make manifests`. Can
// also be read from a URL.
func (o *installOptions) clusterRole() (*rbacv1.ClusterRole, error) {
	data, err := getLocalOrRemoteDoc(o.local, clusterRolePath, repoURL)
	if err != nil {
		return nil, err
	}

	var role rbacv1.ClusterRole
	if err := yaml.Unmarshal(data, &role); err != nil {
		return nil, err
	}

	return &role, nil
}

func getLocalOrRemoteDoc(local bool, path, repo string) (data []byte, err error) {
	if local {
		data, err = ioutil.ReadFile(path)
		if err != nil {
			return nil, err
		}
	} else {
		u, err := url.Parse(repo)
		if err != nil {
			return nil, err
		}
		u.Path = filepath.Join(u.Path, path)
		resp, err := http.Get(u.String())
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("failed to retrieve %s: status code: %d", u, resp.StatusCode)
		}

		data, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
	}

	return data, nil
}
