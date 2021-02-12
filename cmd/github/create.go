package github

import (
	"context"
	"fmt"
	"time"

	// or "gopkg.in/unrolled/render.v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/leg100/etok/cmd/flags"
	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/client"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
)

var (
	defaultTimeout = 10 * time.Minute
)

type createOptions struct {
	*cmdutil.Factory

	*client.Client

	namespace   string
	kubeContext string

	flow *flowServerOptions
}

func createCmd(f *cmdutil.Factory) (*cobra.Command, *createOptions) {
	o := &createOptions{
		Factory:   f,
		namespace: defaultNamespace,
		flow: &flowServerOptions{
			creds: &credentials{
				name:    secretName,
				timeout: defaultTimeout,
			},
		},
	}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a github app",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			// Create k8s client
			o.Client, err = f.Create(o.kubeContext)
			if err != nil {
				return err
			}

			// Create dedicated namespace for github app
			if err := o.createNamespace(cmd.Context()); err != nil {
				return err
			}

			// Setup credentials namespace and client
			o.flow.creds.namespace = o.namespace
			o.flow.creds.client = o.Client.KubeClient

			flowServer, err := newFlowServer(o.flow)
			if err != nil {
				return err
			}

			// Skip app creation if credentials already exist
			exists, err := flowServer.creds.exists(cmd.Context())
			if err != nil {
				return fmt.Errorf("unable to check if credentials exist: %w", err)
			}
			if exists {
				fmt.Println("Github app already created")
				return nil
			}

			if err := flowServer.run(cmd.Context()); err != nil {
				return fmt.Errorf("unable to complete app creation: %w", err)
			}

			fmt.Println("Github app created successfully")

			// Deploy webhook here...

			return nil
		},
	}

	flags.AddNamespaceFlag(cmd, &o.namespace)
	flags.AddKubeContextFlag(cmd, &o.kubeContext)

	cmd.Flags().StringVar(&o.flow.webhook, "webhook", "", "Webhook URL")
	cmd.Flags().StringVar(&o.flow.name, "name", "etok", "Name of github app")
	cmd.Flags().StringVar(&o.flow.githubOrg, "org", "", "Create github app in github organization")
	cmd.Flags().StringVar(&o.flow.githubHostname, "hostname", "github.com", "Github hostname")

	cmd.Flags().BoolVar(&o.flow.disableBrowser, "disable-browser", false, "Disable automatic opening of browser")

	cmd.Flags().IntVar(&o.flow.port, "port", 0, "Flow server port")
	cmd.Flags().MarkHidden("port")

	cmd.Flags().BoolVar(&o.flow.devMode, "dev", false, "Disable development mode")
	cmd.Flags().MarkHidden("dev")

	return cmd, o
}

func (o *createOptions) createNamespace(ctx context.Context) error {
	_, err := o.KubeClient.CoreV1().Namespaces().Get(ctx, o.namespace, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		// Create namespace
		namespace := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: o.namespace}}
		if _, err := o.KubeClient.CoreV1().Namespaces().Create(ctx, &namespace, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("unable to create namespace %s: %w", o.namespace, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("unable to retrieve namespace %s: %w", o.namespace, err)
	}

	// Namespace exists already
	return nil
}
