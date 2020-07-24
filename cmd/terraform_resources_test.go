package cmd

import (
	"testing"
	"time"

	"github.com/leg100/stok/pkg/apis"
	"github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
		Name:      "stok-plan-12345",
		Namespace: "default",
		Labels: map[string]string{
			"app":       "stok",
			"workspace": "workspace-1",
		},
	},
}

var pod = v1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "stok-plan-12345",
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
		Kind:          "Plan",
		Namespace:     "default",
		Workspace:     "default",
		Args:          []string{},
		TimeoutClient: time.Minute,
		TimeoutQueue:  time.Minute,
		scheme:        s,
	}

	client := fake.NewFakeClientWithScheme(s, runtime.Object(&workspaceEmptyQueue))

	plan, err := tc.createCommand(client, "stok-plan-12345", "stok-plan-12345")
	if err != nil {
		t.Fatal(err)
	}

	require.Equal(t, 0, len(plan.GetArgs()))
}

func TestCreateConfigMap(t *testing.T) {
	// adds core GVKs
	s := scheme.Scheme
	// adds CRD GVKs
	apis.AddToScheme(s)

	tc := &terraformCmd{
		Kind:      "Plan",
		Namespace: "default",
		Workspace: "default",
	}

	client := fake.NewFakeClientWithScheme(s, runtime.Object(&workspaceEmptyQueue))

	// TODO: create real tarball
	tarball := make([]byte, 1024)

	configMap, err := tc.createConfigMap(client, &plan, tarball, "stok-plan-12345", "config.tar.gz")
	if err != nil {
		t.Error(err)
	}
	if configMap.Name != "stok-plan-12345" {
		t.Errorf("want stok-plan-12345, got %s\n", configMap.Name)
	}

	ownerRefs := configMap.GetOwnerReferences()
	if len(ownerRefs) != 1 {
		t.Fatal("want one ownerref, got none")
	}

	require.Equal(t, "Plan", ownerRefs[0].Kind)
	require.Equal(t, "stok-plan-12345", ownerRefs[0].Name)
}
