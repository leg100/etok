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

var podFailed = v1.Pod{
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
	},
}

func TestCreateCommand(t *testing.T) {
	s := scheme.Scheme
	// adds CRD GVKs
	apis.AddToScheme(s)

	tc := &terraformCmd{
		Command:       "plan",
		Namespace:     "default",
		Workspace:     "default",
		Args:          []string{},
		TimeoutClient: time.Minute,
		TimeoutQueue:  time.Minute,
		scheme:        s,
	}

	crd, ok := crdinfo.Inventory[tc.Command]
	if !ok {
		t.Fatalf("Could not find Custom Resource Definition '%s'", tc.Command)
	}

	client := fake.NewFakeClientWithScheme(s, runtime.Object(&workspaceEmptyQueue))

	plan, err := tc.createCommand(client, crd)
	if err != nil {
		t.Fatal(err)
	}

	if len(plan.GetArgs()) != 0 {
		t.Fatalf("want one arg, got %d\n", len(plan.GetArgs()))
	}
}

func TestCreateConfigMap(t *testing.T) {
	// adds core GVKs
	s := scheme.Scheme
	// adds CRD GVKs
	apis.AddToScheme(s)

	tc := &terraformCmd{
		Command:   "plan",
		Namespace: "default",
		Workspace: "default",
	}

	client := kubeFake.NewSimpleClientset()

	// TODO: create real tarball
	tarball := bytes.NewBufferString("foo")
	configMap, err := tc.createConfigMap(client, &plan, tarball)
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
