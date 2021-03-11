package github

import (
	"context"
	"fmt"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"

	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	// or "gopkg.in/unrolled/render.v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"github.com/leg100/etok/cmd/flags"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/k8s"
	"github.com/leg100/etok/pkg/labels"
	"github.com/leg100/etok/pkg/version"
	"github.com/spf13/cobra"
)

var (
	defaultTimeout = 10 * time.Minute

	// Interval between polling deployment status
	deploymentInterval = time.Second

	// Timeout for deployment readiness
	deploymentTimeout = 10 * time.Second
)

type deployOptions struct {
	*cmdutil.Factory

	*client.Client

	namespace   string
	kubeContext string
	image       string

	appName           string
	appCreatorOptions createAppOptions

	// Annotations to add to the service account resource
	serviceAccountAnnotations map[string]string

	// Type of service for webhook
	serviceType string

	// Toggle waiting for deployment to be ready
	wait bool

	// Webhook listening port
	port int32

	// Github's hostname
	githubHostname string
}

func deployCmd(f *cmdutil.Factory) (*cobra.Command, *deployOptions) {
	var webhookUrl flags.Url

	o := &deployOptions{
		Factory:   f,
		namespace: defaultNamespace,
	}
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy github app",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			// Create (controller-runtime) k8s client
			o.Client, err = o.CreateRuntimeClient(o.kubeContext)
			if err != nil {
				return err
			}

			// Ensure namespace exists
			ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: o.namespace}}
			_, err = controllerutil.CreateOrUpdate(cmd.Context(), o.RuntimeClient, &ns, func() error { return nil })
			if err != nil {
				return err
			}

			creds := credentials{
				client:    o.RuntimeClient,
				namespace: o.namespace,
				timeout:   defaultTimeout,
			}

			// Skip app creation if credentials already exist
			exists, err := creds.exists(cmd.Context())
			if err != nil {
				return fmt.Errorf("unable to check if credentials exist: %w", err)
			}
			if exists {
				fmt.Println("Github app already created")
				return nil
			}

			if err := createApp(cmd.Context(), o.appName, webhookUrl.String(), o.githubHostname, &creds, o.appCreatorOptions); err != nil {
				return fmt.Errorf("unable to complete app creation: %w", err)
			}

			fmt.Println("Github app created successfully")

			// Deploy webhook k8s resources
			if err := o.deploy(cmd.Context()); err != nil {
				return err
			}

			return nil
		},
	}

	flags.AddNamespaceFlag(cmd, &o.namespace)
	flags.AddKubeContextFlag(cmd, &o.kubeContext)

	cmd.Flags().IntVar(&o.appCreatorOptions.port, "manifest-port", 0, "Manifest server port")
	cmd.Flags().MarkHidden("manifest-port")

	cmd.Flags().BoolVar(&o.appCreatorOptions.disableBrowser, "manifest-disable-browser", false, "Disable automatic opening of browser for manifest server")
	cmd.Flags().MarkHidden("manifest-disable-browser")

	cmd.Flags().BoolVar(&o.appCreatorOptions.devMode, "manifest-dev", false, "Enable development mode for manifest server")
	cmd.Flags().MarkHidden("manifest-dev")

	cmd.Flags().StringVar(&o.appCreatorOptions.githubOrg, "org", "", "Add github app to an organization account instead of your user account")

	cmd.Flags().StringVar(&o.appName, "name", "etok", "Name of github app")
	cmd.Flags().StringVar(&o.githubHostname, "hostname", "github.com", "Github hostname")

	cmd.Flags().Var(&webhookUrl, "url", "Webhook URL")
	cmd.MarkFlagRequired("url")

	cmd.Flags().StringVar(&o.image, "image", version.Image, "Container image for webhook server")

	cmd.Flags().StringToStringVar(&o.serviceAccountAnnotations, "sa-annotations", map[string]string{}, "Annotations to add to the webhook ServiceAccount. Add iam.gke.io/gcp-service-account=[GSA_NAME]@[PROJECT_NAME].iam.gserviceaccount.com for workload identity")

	cmd.Flags().BoolVar(&o.wait, "wait", true, "Toggle waiting for deployment to be ready")

	// Listening port of deployment? Service? Ingress?
	cmd.Flags().Int32Var(&o.port, "port", defaultWebhookPort, "Webhook server listening port")
	return cmd, o
}

func (o *deployOptions) deploy(ctx context.Context) error {
	var resources []runtimeclient.Object

	deploymentResource := deployment(o.namespace, o.image, o.port)

	resources = append(resources, service(o.namespace, o.port))
	resources = append(resources, deploymentResource)
	resources = append(resources, clusterRoleBinding(o.namespace))
	resources = append(resources, clusterRole(o.namespace))
	resources = append(resources, serviceAccount(o.namespace, o.serviceAccountAnnotations))

	for _, r := range resources {
		labels.SetCommonLabels(r)
		labels.SetLabel(r, labels.WebhookComponent)

		_, err := controllerutil.CreateOrUpdate(ctx, o.RuntimeClient, r, func() error { return nil })
		if err != nil {
			return err
		}
	}

	if o.wait {
		fmt.Fprintf(o.Out, "Waiting for Deployment to be ready\n")
		if err := k8s.DeploymentIsReady(ctx, o.RuntimeClient, deploymentResource, deploymentInterval, deploymentTimeout); err != nil {
			return fmt.Errorf("failure while waiting for deployment to be ready: %w", err)
		}
	}

	return nil
}

// createOrUpdate idempotently installs resources, creating the resource if it
// doesn't already exist, otherwise updating it.
func (o *deployOptions) createOrUpdate(ctx context.Context, res runtimeclient.Object) (err error) {
	existing := res.DeepCopyObject().(runtimeclient.Object)

	kind := res.GetObjectKind().GroupVersionKind().Kind

	err = o.RuntimeClient.Get(ctx, runtimeclient.ObjectKeyFromObject(res), existing)
	if kerrors.IsNotFound(err) {
		fmt.Fprintf(o.Out, "Creating resource %s %s\n", kind, klog.KObj(res))
		if err := o.RuntimeClient.Create(ctx, res); err != nil {
			return fmt.Errorf("unable to create resource: %w", err)
		}
	} else if err != nil {
		return err
	}

	res.SetResourceVersion(existing.GetResourceVersion())

	fmt.Fprintf(o.Out, "Updating resource %s %s\n", kind, klog.KObj(res))
	if err := o.RuntimeClient.Update(ctx, res); err != nil {
		return fmt.Errorf("unable to update existing resource: %w", err)
	}

	return nil
}
