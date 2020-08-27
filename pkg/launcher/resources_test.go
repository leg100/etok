package launcher

import (
	"testing"
	"time"

	"github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/scheme"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

func TestCreateCommand(t *testing.T) {
	tc := &Launcher{
		Kind:          "Plan",
		Namespace:     "default",
		Workspace:     "default",
		Args:          []string{},
		TimeoutClient: time.Minute,
		TimeoutQueue:  time.Minute,
	}

	client := fake.NewFakeClientWithScheme(scheme.Scheme, runtime.Object(&workspaceEmptyQueue))

	plan, err := tc.createCommand(client, "stok-plan-12345", "stok-plan-12345")
	if err != nil {
		t.Fatal(err)
	}

	require.Equal(t, 0, len(plan.GetArgs()))
}

func TestCreateConfigMap(t *testing.T) {
	tc := &Launcher{
		Kind:      "Plan",
		Namespace: "default",
		Workspace: "default",
	}

	client := fake.NewFakeClientWithScheme(scheme.Scheme, runtime.Object(&workspaceEmptyQueue))

	// TODO: create real tarball
	tarball := make([]byte, 1024)

	configMap, err := tc.createConfigMap(client, tarball, "stok-plan-12345", "config.tar.gz")
	if err != nil {
		t.Error(err)
	}
	if configMap.Name != "stok-plan-12345" {
		t.Errorf("want stok-plan-12345, got %s\n", configMap.Name)
	}
}
