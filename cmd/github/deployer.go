package github

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	yamlserializer "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/kubernetes"

	"github.com/leg100/etok/config"
	"github.com/leg100/etok/pkg/k8s"
	"github.com/leg100/etok/pkg/labels"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type deployer struct {
	namespace string

	image string

	// Webhook listening port
	port int32

	// Annotations to add to the service account resource
	serviceAccountAnnotations map[string]string

	// Timeout and interval for deployment readiness
	timeout, interval time.Duration

	patch runtimeclient.Patch
}

func (d *deployer) deploy(ctx context.Context, client runtimeclient.Client) error {
	var decUnstructured = yamlserializer.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

	resources, err := config.GetWebhookResources()
	if err != nil {
		panic(err.Error())
	}

	for _, res := range resources {
		// Decode YAML manifest into unstructured.Unstructured
		obj := &unstructured.Unstructured{}
		_, _, err := decUnstructured.Decode(res, nil, obj)
		if err != nil {
			return err
		}

		switch obj.GetKind() {
		case "ClusterRoleBinding", "ClusterRole":
			// Skip setting namespace for non-namespaced resources
		default:
			obj.SetNamespace(d.namespace)
		}

		if obj.GetKind() == "Deployment" {
			// Override container settings

			// Get deploy containers
			containers, found, err := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
			if err != nil || !found || len(containers) != 1 {
				panic("deployment resource is corrupt")
			}

			// Set image
			if err := unstructured.SetNestedField(containers[0].(map[string]interface{}), d.image, "image"); err != nil {
				panic(err.Error())
			}

			// Update deployment with updated container
			if err := unstructured.SetNestedSlice(obj.Object, containers, "spec", "template", "spec", "containers"); err != nil {
				panic(err.Error())
			}
		}

		if obj.GetKind() == "ServiceAccount" {
			obj.SetAnnotations(d.serviceAccountAnnotations)
		}

		// Set labels
		labels.SetCommonLabels(obj)
		labels.SetLabel(obj, labels.WebhookComponent)

		// Check resource exists and create or patch accordingly
		err = client.Get(ctx, runtimeclient.ObjectKeyFromObject(obj), obj.DeepCopy())
		switch {
		case kerrors.IsNotFound(err):
			fmt.Printf("Creating resource %s %s\n", obj.GetKind(), klog.KObj(obj))
			err = client.Create(ctx, obj, &runtimeclient.CreateOptions{FieldManager: "etok-cli"})
			if err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			// Update the object with SSA
			fmt.Printf("Updating resource %s %s\n", obj.GetKind(), klog.KObj(obj))
			force := true
			err = client.Patch(ctx, obj, d.patch, &runtimeclient.PatchOptions{
				FieldManager: "etok-cli",
				Force:        &force,
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *deployer) wait(ctx context.Context, client kubernetes.Interface) error {
	fmt.Printf("Waiting for Deployment to be ready\n")
	if err := k8s.DeploymentIsReady(ctx, client, d.namespace, "webhook", d.timeout, d.interval); err != nil {
		return fmt.Errorf("failure while waiting for deployment to be ready: %w", err)
	}

	return nil
}
