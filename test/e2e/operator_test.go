package e2e

import (
	"testing"
	"time"

	goctx "context"

	"github.com/leg100/terraform-operator/pkg/apis"
	cachev1alpha1 "github.com/leg100/terraform-operator/pkg/apis/terraform/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
)

var (
	retryInterval        = time.Second * 5
	timeout              = time.Second * 60
	cleanupRetryInterval = time.Second * 1
	cleanupTimeout       = time.Second * 5
)

func TestWorkspace(t *testing.T) {
	workspaceList := &cachev1alpha1.WorkspaceList{}
	err := framework.AddToFrameworkScheme(apis.AddToScheme, workspaceList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}

	commandList := &cachev1alpha1.CommandList{}
	err = framework.AddToFrameworkScheme(apis.AddToScheme, commandList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}

	t.Run("Workspace1", CreateWorkspace)
}

func CreateWorkspace(t *testing.T) {
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()

	err := ctx.InitializeClusterResources(&framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		t.Fatalf("failed to initialize cluster resources: %v", err)
	}

	// get namespace
	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatal(err)
	}
	// get global framework variables
	f := framework.Global
	// wait for workspace-operator to be ready
	err = e2eutil.WaitForOperatorDeployment(t, f.KubeClient, namespace, "terraform-operator", 1, time.Second*5, time.Second*30)
	if err != nil {
		t.Fatal(err)
	}

	// create workspace custom resource
	exampleWorkspace := &cachev1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-workspace",
			Namespace: namespace,
		},
	}
	err = f.Client.Create(goctx.TODO(), exampleWorkspace, &framework.CleanupOptions{TestContext: ctx, Timeout: time.Second * 5, RetryInterval: time.Second * 1})
	if err != nil {
		t.Fatal(err)
	}

	err = wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		err = f.Client.Get(goctx.TODO(), types.NamespacedName{Namespace: namespace, Name: "example-workspace-pvc"}, &corev1.PersistentVolumeClaim{})

		if err != nil {
			return false, err
		}

		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// create command custom resource
	exampleCommand := &cachev1alpha1.Command{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "example-command",
			Namespace: namespace,
		},
		Spec: cachev1alpha1.CommandSpec{
			Workspace: "example-workspace",
			Args:      []string{"version"},
		},
	}
	err = f.Client.Create(goctx.TODO(), exampleCommand, &framework.CleanupOptions{TestContext: ctx, Timeout: time.Second * 5, RetryInterval: time.Second * 1})
	if err != nil {
		t.Fatal(err)
	}

	found := &corev1.Pod{}
	// wait for command pod to be created
	err = wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		err = f.Client.Get(goctx.TODO(), types.NamespacedName{Namespace: namespace, Name: "example-command-pod"}, found)

		if err != nil {
			return false, err
		}

		if found.Status.Phase == corev1.PodSucceeded {
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
