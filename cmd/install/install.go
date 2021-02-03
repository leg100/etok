package install

import (
	"context"
	"errors"
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
	// Relative paths to the cluster roles to be installed. Paths relative to
	// the root of the repo.
	clusterRolePaths = []string{
		"config/rbac/role.yaml",
		"config/rbac/user.yaml",
		"config/rbac/admin.yaml",
	}

	// Interval between polling deployment status
	interval = time.Second

	errInvalidBackupConfig = errors.New("invalid backup config")
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
	// Annotations to add to the service account resource
	serviceAccountAnnotations map[string]string

	// Toggle only installing CRDs
	crdsOnly bool

	// Toggle reading resources from local files rather than a URL
	local bool

	// Toggle waiting for deployment to be ready
	wait bool
	// Time to wait for
	timeout time.Duration

	// Print out resources and don't install
	dryRun bool

	// Toggle state backups
	backupProviderName string

	// GCS backup bucket
	gcsBucket string
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
			if err := o.validateBackupOptions(); err != nil {
				return err
			}

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
	cmd.Flags().StringToStringVar(&o.serviceAccountAnnotations, "sa-annotations", map[string]string{}, "Annotations to add to the etok ServiceAccount. Add iam.gke.io/gcp-service-account=[GSA_NAME]@[PROJECT_NAME].iam.gserviceaccount.com for workload identity")
	cmd.Flags().BoolVar(&o.crdsOnly, "crds-only", o.crdsOnly, "Only generate CRD resources. Useful for updating CRDs for an existing Etok install.")

	cmd.Flags().StringVar(&o.backupProviderName, "backup-provider", "", "Enable backups specifying a provider (only 'gcs' supported currently)")

	cmd.Flags().StringVar(&o.gcsBucket, "gcs-bucket", "", "Specify GCS bucket for terraform state backups")

	return cmd, o
}

func (o *installOptions) validateBackupOptions() error {
	if o.backupProviderName != "" {
		if o.backupProviderName != "gcs" {
			return fmt.Errorf("%w: %s is invalid value for --backup-provider, valid options are: gcs", errInvalidBackupConfig, o.backupProviderName)
		}
	}

	if (o.backupProviderName == "" && o.gcsBucket != "") || (o.backupProviderName != "" && o.gcsBucket == "") {
		return fmt.Errorf("%w: you must specify both --backup-provider and --gcs-bucket", errInvalidBackupConfig)
	}

	return nil
}

func (o *installOptions) install(ctx context.Context) error {
	var deploy *appsv1.Deployment
	var resources []runtimeclient.Object

	for _, path := range crdPaths {
		res, err := o.crd(path)
		if err != nil {
			return err
		}
		resources = append(resources, res)
	}

	if !o.crdsOnly {
		for _, path := range clusterRolePaths {
			role, err := o.clusterRole(path)
			if err != nil {
				return err
			}
			resources = append(resources, role)
		}

		resources = append(resources, operatorClusterRoleBinding(o.namespace))
		resources = append(resources, userClusterRoleBinding())
		resources = append(resources, adminClusterRoleBinding())
		resources = append(resources, namespace(o.namespace))
		resources = append(resources, serviceAccount(o.namespace, o.serviceAccountAnnotations))

		secretPresent := o.secretFile != ""

		// Deploy options
		dopts := []podTemplateOption{}
		dopts = append(dopts, WithSecret(secretPresent))
		dopts = append(dopts, WithImage(o.image))
		if o.backupProviderName == "gcs" {
			dopts = append(dopts, WithGCSProvider(o.gcsBucket))
		}

		deploy = deployment(o.namespace, dopts...)
		resources = append(resources, deploy)

		if o.secretFile != "" {
			key, err := ioutil.ReadFile(o.secretFile)
			if err != nil {
				return err
			}

			resources = append(resources, secret(o.namespace, key))
		}
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

	if o.wait && !o.crdsOnly {
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

		kind := res.GetObjectKind().GroupVersionKind().Kind

		err := o.RuntimeClient.Get(ctx, runtimeclient.ObjectKeyFromObject(res), existing)
		switch {
		case kerrors.IsNotFound(err):
			fmt.Fprintf(o.Out, "Creating resource %s %s\n", kind, klog.KObj(res))
			if err := o.RuntimeClient.Create(ctx, res); err != nil {
				return fmt.Errorf("unable to create resource: %w", err)
			}
		case err != nil:
			return err
		default:
			if kind == "ClusterRoleBinding" && (res.GetName() == "etok-users" || res.GetName() == "etok-admins") {
				// Preserve any out-of-band changes to subjects
				existingBinding := existing.(*rbacv1.ClusterRoleBinding)
				updatedBinding := res.(*rbacv1.ClusterRoleBinding)
				updatedBinding.Subjects = existingBinding.Subjects
			}

			res.SetResourceVersion(existing.GetResourceVersion())

			fmt.Fprintf(o.Out, "Updating resource %s %s\n", kind, klog.KObj(res))
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

// Unmarshal cluster role. Unlike most other resources this is read from a YAML
// file from the repo, which in turn is installed with `make manifests`.  Can
// also be read from a URL.
func (o *installOptions) clusterRole(path string) (*rbacv1.ClusterRole, error) {
	data, err := getLocalOrRemoteDoc(o.local, path, repoURL)
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
