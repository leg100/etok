package e2e

import (
	"fmt"
	"testing"
	"time"

	goctx "context"

	"github.com/leg100/stok/operator/pkg/apis"
	terraformv1alpha1 "github.com/leg100/stok/operator/pkg/apis/terraform/v1alpha1"
	"github.com/leg100/stok/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/operator-framework/operator-sdk/pkg/status"
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
	workspaceList := &terraformv1alpha1.WorkspaceList{}
	err := framework.AddToFrameworkScheme(apis.AddToScheme, workspaceList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}

	commandList := &terraformv1alpha1.CommandList{}
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
	err = e2eutil.WaitForOperatorDeployment(t, f.KubeClient, namespace, "stok-operator", 1, time.Second*5, time.Second*30)
	if err != nil {
		t.Fatal(err)
	}

	// create workspace custom resource
	workspace := &terraformv1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workspace-1",
			Namespace: namespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "Workspace",
		},
	}
	err = f.Client.Create(goctx.TODO(), workspace, &framework.CleanupOptions{TestContext: ctx, Timeout: time.Second * 5, RetryInterval: time.Second * 1})
	if err != nil {
		t.Fatal(err)
	}

	err = wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		err = f.Client.Get(goctx.TODO(), types.NamespacedName{Namespace: namespace, Name: "workspace-1"}, &corev1.PersistentVolumeClaim{})

		if err != nil {
			return false, err
		}

		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	configMaps := []struct {
		name  string
		path  string
		files []string
	}{
		{
			name:  "tarball1",
			path:  "./test/e2e/tarball1",
			files: []string{"test1.tf", "test2.tf"},
		},
		{
			name:  "tarball2",
			path:  "./test/e2e/tarball2",
			files: []string{"test1.tf", "test2.tf", "test3.tf"},
		},
		{
			name:  "tarball3",
			path:  "./test/e2e/tarball3",
			files: []string{},
		},
	}
	for _, cm := range configMaps {
		// create tar
		tar, err := utils.Create(cm.path, cm.files)
		if err != nil {
			t.Fatal(err)
		}

		configMap := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cm.name,
				Namespace: "operator-test",
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			BinaryData: map[string][]byte{
				"tarball.tar.gz": tar.Bytes(),
			},
		}
		err = f.Client.Create(goctx.TODO(), &configMap, &framework.CleanupOptions{TestContext: ctx, Timeout: time.Second * 5, RetryInterval: time.Second * 1})
		if err != nil {
			t.Fatal(err)
		}
	}

	commands := []struct {
		name            string
		args            []string
		completedReason string
		configMap       string
	}{
		{
			name:            "command-1",
			args:            []string{"-c", "test -f test1.tf; touch .terraform/persist_this"},
			completedReason: "PodSucceeded",
			configMap:       "tarball1",
		},
		{
			name:            "command-2",
			args:            []string{"-c", "test -f test3.tf; test -f .terraform/persist_this"},
			completedReason: "PodSucceeded",
			configMap:       "tarball2",
		},
		{
			name:            "command-3",
			args:            []string{"-c", "test -f test1.tf"},
			completedReason: "PodFailed",
			configMap:       "tarball3",
		},
	}

	// create command resources
	for _, c := range commands {
		instance := &terraformv1alpha1.Command{
			ObjectMeta: metav1.ObjectMeta{
				Name:      c.name,
				Namespace: namespace,
				Labels:    map[string]string{"workspace": "workspace-1"},
			},
			TypeMeta: metav1.TypeMeta{
				Kind: "Command",
			},
			Spec: terraformv1alpha1.CommandSpec{
				Command:      []string{"sh"},
				Args:         c.args,
				ConfigMap:    c.configMap,
				ConfigMapKey: "tarball.tar.gz",
			},
		}
		err = f.Client.Create(goctx.TODO(), instance, &framework.CleanupOptions{TestContext: ctx, Timeout: time.Second * 5, RetryInterval: time.Second * 1})
		if err != nil {
			t.Fatal(err)
		}
	}

	// wait for commands' completed condition to have status true and expected reason
	for _, c := range commands {
		err = wait.Poll(retryInterval, timeout, func() (done bool, err error) {
			instance := &terraformv1alpha1.Command{}
			err = f.Client.Get(goctx.TODO(), types.NamespacedName{Namespace: namespace, Name: c.name}, instance)

			if err != nil {
				return false, err
			}

			if instance.Status.Conditions.IsTrueFor(status.ConditionType("Completed")) {
				fmt.Printf("Command %s completed\n", c.name)
				reason := instance.Status.Conditions.GetCondition(status.ConditionType("Completed")).Reason
				if string(reason) == c.completedReason {
					return true, nil
				} else {
					return true, fmt.Errorf("expected %s, got %v", c.completedReason, reason)
				}
			}

			return false, nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
