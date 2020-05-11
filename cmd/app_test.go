package cmd

import (
	"bytes"
	"testing"
	"time"

	"github.com/leg100/stok/crdinfo"
	"github.com/leg100/stok/pkg/apis"
	"github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeFake "k8s.io/client-go/kubernetes/fake"
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

var plan = v1alpha1.Plan{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "stok-xxxx",
		Namespace: "default",
		Labels: map[string]string{
			"app":       "stok",
			"workspace": "workspace-1",
		},
	},
	CommandSpec: v1alpha1.CommandSpec{
		Args: []string{"plan"},
	},
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

func TestCreateCommand(t *testing.T) {
	// adds core GVKs
	s := scheme.Scheme
	// adds CRD GVKs
	apis.AddToScheme(s)

	app := &App{
		Namespace:  "default",
		Workspace:  "default",
		crd:        crdinfo.CRDInfo{Name: "plan", APISingular: "plan", APIPlural: "plans", Entrypoint: []string{"terraform", "plan"}},
		Args:       []string{},
		Resources:  []runtime.Object{},
		Client:     fake.NewFakeClientWithScheme(s, runtime.Object(&workspaceEmptyQueue)),
		KubeClient: kubeFake.NewSimpleClientset(),
		Command:    &v1alpha1.Plan{},
	}

	// TODO: create real tarball
	err := app.CreateCommand()
	if err != nil {
		t.Error(err)
	}

	gotArgs := app.Command.GetArgs()
	if len(gotArgs) != 0 {
		t.Fatalf("want one arg, got %d\n", len(gotArgs))
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
		crd:        crdinfo.CRDInfo{Name: "plan", APISingular: "plan", APIPlural: "plans", Entrypoint: []string{"terraform", "plan"}},
		Args:       []string{"plan"},
		Resources:  []runtime.Object{},
		Client:     fake.NewFakeClientWithScheme(s, runtime.Object(&workspaceEmptyQueue), runtime.Object(&plan)),
		KubeClient: kubeFake.NewSimpleClientset(),
		Command:    &plan,
	}

	// TODO: create real tarball
	tarball := bytes.NewBufferString("foo")
	configMap, err := app.CreateConfigMap(tarball)
	if err != nil {
		t.Error(err)
	}
	if configMap.Name != "stok-xxxx" {
		t.Errorf("want stok-xxxx, got %s\n", configMap.Name)
	}

	ownerRefs := configMap.GetOwnerReferences()
	if len(ownerRefs) != 1 {
		t.Fatal("want one ownerref, got none")
	}
	if ownerRefs[0].Kind != "Plan" {
		t.Errorf("want ownerref controller kind Plan, got %s\n", ownerRefs[0].Kind)
	}
	if ownerRefs[0].Name != "stok-xxxx" {
		t.Errorf("want ownerref controller name stok-xxxx got %s\n", ownerRefs[0].Name)
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
