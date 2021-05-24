package github

import (
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	// or "gopkg.in/unrolled/render.v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/leg100/etok/cmd/flags"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/version"
	"github.com/spf13/cobra"
)

var (
	// Default timeout for waiting for app to be created
	defaultTimeout = 10 * time.Minute
)

type deployOptions struct {
	*cmdutil.Factory

	*client.Client

	namespace   string
	kubeContext string

	appName           string
	appCreatorOptions createAppOptions

	// Type of service for webhook
	serviceType string

	// Github's hostname
	githubHostname string

	// Toggle waiting for deployment to be ready
	wait bool

	// Only deploy k8s resources, don't create app
	deployOnly bool

	deployer
}

func deployCmd(f *cmdutil.Factory) (*cobra.Command, *deployOptions) {
	var webhookUrl flags.Url

	o := &deployOptions{
		namespace: defaultNamespace,
	}
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy github app",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			// Create (controller-runtime) k8s client
			o.Client, err = f.CreateRuntimeClient(o.kubeContext)
			if err != nil {
				return err
			}

			// Ensure namespace exists
			ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: o.namespace}}
			_, err = controllerutil.CreateOrUpdate(cmd.Context(), o.RuntimeClient, &ns, func() error { return nil })
			if err != nil {
				return err
			}

			if !o.deployOnly && !o.crdsOnly {
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

				if !exists {
					if err := createApp(cmd.Context(), o.appName, webhookUrl.String(), o.githubHostname, &creds, o.appCreatorOptions); err != nil {
						return fmt.Errorf("unable to complete app creation: %w", err)
					}
				} else {
					fmt.Println("Skipping creation of Github app; app credentials found")
				}
			}

			// Deploy webhook k8s resources
			o.deployer.namespace = o.namespace
			o.deployer.port = defaultWebhookPort
			o.deployer.timeout = 60 * time.Second
			o.deployer.interval = 1 * time.Second
			o.deployer.patch = runtimeclient.Apply
			if err := o.deployer.deploy(cmd.Context(), o.RuntimeClient); err != nil {
				return err
			}

			// Wait for deployment to be ready
			if !o.crdsOnly && o.wait {
				if err := o.deployer.wait(cmd.Context(), o.RuntimeClient); err != nil {
					return err
				}
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

	cmd.Flags().StringVar(&o.image, "image", version.Image, "Container image for webhook server")

	cmd.Flags().StringToStringVar(&o.serviceAccountAnnotations, "sa-annotations", map[string]string{}, "Annotations to add to the webhook ServiceAccount. Add iam.gke.io/gcp-service-account=[GSA_NAME]@[PROJECT_NAME].iam.gserviceaccount.com for workload identity")

	cmd.Flags().BoolVar(&o.wait, "wait", true, "Toggle waiting for deployment to be ready")

	cmd.Flags().BoolVar(&o.deployOnly, "deploy-only", o.deployOnly, "Only deploy kubernetes resources, do not create github app")

	cmd.Flags().BoolVar(&o.crdsOnly, "crds-only", o.crdsOnly, "Only generate CRD resources. Useful for updating CRDs for an existing Etok install.")

	return cmd, o
}
