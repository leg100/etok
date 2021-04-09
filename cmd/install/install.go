package install

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/leg100/etok/cmd/backup"
	"github.com/leg100/etok/cmd/flags"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/config"
	"github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/labels"
	"github.com/leg100/etok/pkg/version"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"sigs.k8s.io/yaml"

	"k8s.io/apimachinery/pkg/runtime/schema"
	yamlserializer "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
)

const (
	defaultNamespace = "etok"
)

type installOptions struct {
	*cmdutil.Factory

	*client.Client

	name        string
	namespace   string
	image       string
	kubeContext string

	// Annotations to add to the service account resource
	serviceAccountAnnotations map[string]string

	// Toggle only installing CRDs
	crdsOnly bool

	// Toggle waiting for deployment to be ready
	wait bool
	// Time to wait for
	timeout time.Duration

	// Print out resources and don't install
	dryRun bool

	// State backup configuration
	backupCfg *backup.Config

	// flags are the install command's parsed flags
	flags *pflag.FlagSet
}

func InstallCmd(f *cmdutil.Factory) (*cobra.Command, *installOptions) {
	o := &installOptions{
		backupCfg: backup.NewConfig(),
		Factory:   f,
		namespace: defaultNamespace,
	}

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install etok operator",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			// Validate backup flags
			if err := o.backupCfg.Validate(cmd.Flags()); err != nil {
				return err
			}

			o.flags = cmd.Flags()

			kclient, err := o.Create(o.kubeContext)
			if err != nil {
				return err
			}

			o.Client, err = o.CreateRuntimeClient(o.kubeContext)
			if err != nil {
				return err
			}

			if err := o.install(cmd.Context(), o.Client.RuntimeClient); err != nil {
				return err
			}

			if o.wait && !o.crdsOnly && !o.dryRun {
				fmt.Fprintf(o.Out, "Waiting for Deployment to be ready\n")
				if err := deploymentIsReady(cmd.Context(), o.namespace, "etok", kclient.KubeClient, o.timeout, time.Second); err != nil {
					return fmt.Errorf("failure while waiting for deployment to be ready: %w", err)
				}
			}

			return nil
		},
	}

	flags.AddNamespaceFlag(cmd, &o.namespace)
	flags.AddKubeContextFlag(cmd, &o.kubeContext)

	o.backupCfg.AddToFlagSet(cmd.Flags())

	cmd.Flags().StringVar(&o.name, "name", "etok-operator", "Name for kubernetes resources")
	cmd.Flags().StringVar(&o.image, "image", version.Image, "Docker image used for both the operator and the runner")

	cmd.Flags().BoolVar(&o.dryRun, "dry-run", false, "Don't install resources just print out them in YAML format")
	cmd.Flags().BoolVar(&o.wait, "wait", true, "Toggle waiting for deployment to be ready")
	cmd.Flags().DurationVar(&o.timeout, "timeout", 60*time.Second, "Timeout for waiting for deployment to be ready")

	cmd.Flags().StringToStringVar(&o.serviceAccountAnnotations, "sa-annotations", map[string]string{}, "Annotations to add to the etok ServiceAccount. Add iam.gke.io/gcp-service-account=[GSA_NAME]@[PROJECT_NAME].iam.gserviceaccount.com for workload identity")
	cmd.Flags().BoolVar(&o.crdsOnly, "crds-only", o.crdsOnly, "Only generate CRD resources. Useful for updating CRDs for an existing Etok install.")

	return cmd, o
}

func (o *installOptions) install(ctx context.Context, client runtimeclient.Client) error {
	var decUnstructured = yamlserializer.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

	resources, err := config.GetOperatorResources()
	if err != nil {
		panic(err.Error())
	}

	// Ensure namespace exists first
	ns := &corev1.Namespace{}
	ns.SetName(o.namespace)
	controllerutil.CreateOrUpdate(ctx, client, ns, func() error { return nil })

	// Determine if secret is present
	var secretPresent bool
	secret := &unstructured.Unstructured{}
	secret.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "Secret"})
	err = client.Get(ctx, runtimeclient.ObjectKey{Namespace: "etok", Name: "etok"}, secret)
	if err != nil && !kerrors.IsNotFound(err) {
		return fmt.Errorf("unable to check for secret: %w", err)
	}
	if err == nil {
		secretPresent = true
	}

	var docs []string

	for _, res := range resources {
		// Decode YAML manifest into unstructured.Unstructured
		obj := &unstructured.Unstructured{}
		_, _, err := decUnstructured.Decode(res, nil, obj)
		if err != nil {
			return err
		}

		if o.crdsOnly && obj.GetKind() != "CustomResourceDefinition" {
			continue
		}

		switch obj.GetKind() {
		case "CustomResourceDefinition", "ClusterRoleBinding", "ClusterRole":
			// Skip setting namespace for non-namespaced resources
		default:
			obj.SetNamespace(o.namespace)
		}

		if obj.GetKind() == "Deployment" {
			// Override container settings

			// Get deploy containers
			containers, found, err := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
			if err != nil || !found || len(containers) != 1 {
				panic("deployment resource is corrupt")
			}

			// Set image
			if err := unstructured.SetNestedField(containers[0].(map[string]interface{}), o.image, "image"); err != nil {
				panic(err.Error())
			}

			// Get container[0] env
			env, _, err := unstructured.NestedSlice(containers[0].(map[string]interface{}), "env")
			if err != nil {
				panic("deployment resource is corrupt")
			}

			// Set image env var
			env = append(env, map[string]interface{}{
				"name":  "ETOK_IMAGE",
				"value": o.image,
			})

			// Set backup envs
			for _, ev := range o.backupCfg.GetEnvVars(o.flags) {
				env = append(env, map[string]interface{}{
					"name":  ev.Name,
					"value": ev.Value,
				})
			}

			// Update container[0] envs
			if err := unstructured.SetNestedSlice(containers[0].(map[string]interface{}), env, "env"); err != nil {
				panic(err.Error())
			}

			if secretPresent {
				// Get container[0] envFroms
				envfrom, _, err := unstructured.NestedSlice(containers[0].(map[string]interface{}), "envFrom")
				if err != nil {
					panic("deployment resource is corrupt")
				}

				// Set reference to secret, to load env vars from secret resource
				envfrom = append(envfrom, map[string]interface{}{
					"secretRef": map[string]interface{}{
						"name": "etok",
					},
				})

				// Update container[0] envFroms
				if err := unstructured.SetNestedSlice(containers[0].(map[string]interface{}), envfrom, "envFrom"); err != nil {
					panic(err.Error())
				}
			}

			// Update deployment with updated container
			if err := unstructured.SetNestedSlice(obj.Object, containers, "spec", "template", "spec", "containers"); err != nil {
				panic(err.Error())
			}
		}

		if obj.GetKind() == "ServiceAccount" {
			obj.SetAnnotations(o.serviceAccountAnnotations)
		}

		// Set labels
		labels.SetCommonLabels(obj)
		labels.SetLabel(obj, labels.OperatorComponent)

		if o.dryRun {
			// Print out YAML representation
			data, err := yaml.Marshal(obj)
			if err != nil {
				panic(err.Error())
			}
			docs = append(docs, string(data))

			// Don't install resource
			continue
		}

		// Check resource exists and create or patch accordingly
		err = client.Get(ctx, runtimeclient.ObjectKeyFromObject(obj), obj.DeepCopy())
		switch {
		case kerrors.IsNotFound(err):
			fmt.Fprintf(o.Out, "Creating resource %s %s\n", obj.GetKind(), klog.KObj(obj))
			err = client.Create(ctx, obj, &runtimeclient.CreateOptions{FieldManager: "etok-cli"})
			if err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			// Update the object with SSA
			fmt.Fprintf(o.Out, "Updating resource %s %s\n", obj.GetKind(), klog.KObj(obj))
			force := true
			err = client.Patch(context.Background(), obj, runtimeclient.Apply, &runtimeclient.PatchOptions{
				FieldManager: "etok-cli",
				Force:        &force,
			})
			if err != nil {
				return err
			}
		}
	}

	if o.dryRun {
		// Print out YAML representation
		fmt.Fprint(o.Out, strings.Join(docs, "\n---\n"))
	}

	return nil
}

// DeploymentIsReady will poll the kubernetes API server to see if the etok
// deployment is ready to service user requests.
func deploymentIsReady(ctx context.Context, namespace, name string, client kubernetes.Interface, timeout, interval time.Duration) error {
	var readyObservations int32
	return wait.PollImmediate(interval, timeout, func() (bool, error) {
		deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		for _, cond := range deployment.Status.Conditions {
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
