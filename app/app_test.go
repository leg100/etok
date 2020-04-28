package app

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/leg100/stok/pkg/apis"
	"github.com/leg100/stok/pkg/apis/terraform/v1alpha1"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeFake "k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var workspaceEmptyQueue = v1alpha1.Workspace{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "default",
		Namespace: "default",
	},
	Spec: v1alpha1.WorkspaceSpec{
		SecretName: "secret-1",
	},
}

// generateNameReactor implements the logic required for the GenerateName field to work when using
// the fake client. Add it with client.PrependReactor to your fake client.
func generateNameReactor(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
	s := action.(ktesting.CreateAction).GetObject().(*v1.ConfigMap)
	if s.Name == "" && s.GenerateName != "" {
		s.Name = fmt.Sprintf("%sxxxx", s.GenerateName)
	}
	return false, nil, nil
}

var pod = v1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "stok-xxxx",
		Namespace: "default",
	},
	Spec: v1.PodSpec{
		Containers: []v1.Container{
			{
				Name:  "terraform",
				Image: "hashicorp/terraform:0.12.21",
			},
		},
	},
	Status: v1.PodStatus{
		Phase: v1.PodRunning,
		Conditions: []v1.PodCondition{
			{
				Type:   v1.PodReady,
				Status: v1.ConditionTrue,
			},
		},
	},
}

func TestCheckWorkspaceExistsTrue(t *testing.T) {
	// adds core GVKs
	s := scheme.Scheme
	// adds CRD GVKs
	apis.AddToScheme(s)

	app := &App{
		Namespace: "default",
		Workspace: "default",
		Client:    fake.NewFakeClientWithScheme(s, runtime.Object(&workspaceEmptyQueue)),
	}

	found, err := app.CheckWorkspaceExists()
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Error("want workspace, got no workspace")
	}

}

func TestCheckWorkspaceExistsFalse(t *testing.T) {
	// adds core GVKs
	s := scheme.Scheme
	// adds CRD GVKs
	apis.AddToScheme(s)

	app := &App{
		Namespace: "default",
		Workspace: "default",
		Client:    fake.NewFakeClientWithScheme(s),
	}

	found, err := app.CheckWorkspaceExists()
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("don't want workspace, got workspace")
	}
}

func TestCreateConfigMap(t *testing.T) {
	// adds core GVKs
	s := scheme.Scheme
	// adds CRD GVKs
	apis.AddToScheme(s)

	app := &App{
		Namespace:  "default",
		Workspace:  "default",
		Args:       []string{"plan"},
		Resources:  []runtime.Object{},
		Client:     fake.NewFakeClientWithScheme(s, runtime.Object(&workspaceEmptyQueue)),
		KubeClient: kubeFake.NewSimpleClientset(),
		Logger:     zap.NewExample().Sugar(),
	}

	app.KubeClient.(*kubeFake.Clientset).PrependReactor("create", "configmaps", generateNameReactor)

	// TODO: create real tarball
	tarball := bytes.NewBufferString("foo")
	name, err := app.CreateConfigMap(tarball)
	if err != nil {
		t.Error(err)
	}
	if name != "stok-xxxx" {
		t.Errorf("want stok-xxxx, got %s\n", name)
	}
}

func TestWaitForPodReadyAndRunning(t *testing.T) {
	app := &App{
		Namespace:  "default",
		Workspace:  "default",
		KubeClient: kubeFake.NewSimpleClientset(),
	}

	_, err := app.KubeClient.CoreV1().Pods(app.Namespace).Create(&pod)
	if err != nil {
		t.Error(err)
	}

	_, err = app.WaitForPod("stok-xxxx", podRunningAndReady, 1*time.Second)
	if err != nil {
		t.Error(err)
	}
}

func TestWaitForPodFailed(t *testing.T) {
	app := &App{
		Namespace:  "default",
		Workspace:  "default",
		KubeClient: kubeFake.NewSimpleClientset(),
	}

	pod.Status = v1.PodStatus{
		Phase: v1.PodFailed,
		ContainerStatuses: []v1.ContainerStatus{
			{
				State: v1.ContainerState{
					Terminated: &v1.ContainerStateTerminated{
						Message: "Message regarding last termination of container",
					},
				},
			},
		},
	}

	_, err := app.KubeClient.CoreV1().Pods(app.Namespace).Create(&pod)
	if err != nil {
		t.Error(err)
	}

	_, err = app.WaitForPod("stok-xxxx", podRunningAndReady, 1*time.Second)
	if err.Error() != "Message regarding last termination of container" {
		t.Error(err)
	}
}
