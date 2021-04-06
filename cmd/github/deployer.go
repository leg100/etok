package github

import (
	"context"
	"fmt"
	"time"

	"github.com/leg100/etok/pkg/k8s"
	"github.com/leg100/etok/pkg/labels"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// Interval between polling deployment status
	deploymentInterval = time.Second

	// Timeout for deployment readiness
	deploymentTimeout = 10 * time.Second
)

type deployer struct {
	namespace string

	image string

	// Webhook listening port
	port int32

	// Annotations to add to the service account resource
	serviceAccountAnnotations map[string]string

	// Toggle waiting for deployment to be ready
	wait bool
}

func (o *deployer) deploy(ctx context.Context, client runtimeclient.Client) error {
	var resources []runtimeclient.Object

	deploymentResource := deployment(o.namespace, o.image, o.port)

	resources = append(resources, service(o.namespace, o.port))
	resources = append(resources, deploymentResource)
	resources = append(resources, clusterRoleBinding(o.namespace))
	resources = append(resources, clusterRole())
	resources = append(resources, serviceAccount(o.namespace, o.serviceAccountAnnotations))

	for _, r := range resources {
		labels.SetCommonLabels(r)
		labels.SetLabel(r, labels.WebhookComponent)

		existing := r.DeepCopyObject().(runtimeclient.Object)

		err := client.Get(ctx, runtimeclient.ObjectKeyFromObject(r), existing)
		switch {
		case kerrors.IsNotFound(err):
			fmt.Printf("Creating resource %T %s\n", r, klog.KObj(r))
			if err := client.Create(ctx, r); err != nil {
				return fmt.Errorf("unable to create resource: %w", err)
			}
		case err != nil:
			return err
		default:
			r.SetResourceVersion(existing.GetResourceVersion())

			kind := r.GetObjectKind().GroupVersionKind().Kind

			fmt.Printf("Updating resource %s %s\n", kind, klog.KObj(r))
			if err := client.Update(ctx, r); err != nil {
				return fmt.Errorf("unable to update existing resource: %w", err)
			}
			fmt.Printf("%T %s has been %s\n", r, klog.KObj(r), "updated")
		}
	}

	if o.wait {
		fmt.Printf("Waiting for Deployment to be ready\n")
		if err := k8s.DeploymentIsReady(ctx, client, deploymentResource, deploymentInterval, deploymentTimeout); err != nil {
			return fmt.Errorf("failure while waiting for deployment to be ready: %w", err)
		}
	}

	return nil
}
