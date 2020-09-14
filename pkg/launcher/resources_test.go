package launcher

import (
	"testing"
	"time"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
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

func TestCreateRun(t *testing.T) {
	tc := &Launcher{
		Namespace:     "default",
		Workspace:     "default",
		Args:          []string{},
		TimeoutClient: time.Minute,
		TimeoutQueue:  time.Minute,
	}

	client := fake.NewFakeClientWithScheme(scheme.Scheme, runtime.Object(&workspaceEmptyQueue))

	plan, err := tc.createRun(client, "run-12345", "run-12345")
	if err != nil {
		t.Fatal(err)
	}

	require.Equal(t, 0, len(plan.GetArgs()))
}

func TestCreateConfigMap(t *testing.T) {
	tc := &Launcher{
		Namespace: "default",
		Workspace: "default",
	}

	client := fake.NewFakeClientWithScheme(scheme.Scheme, runtime.Object(&workspaceEmptyQueue))

	// TODO: create real tarball
	tarball := make([]byte, 1024)

	configMap, err := tc.createConfigMap(client, tarball, "run-12345", "config.tar.gz")
	if err != nil {
		t.Error(err)
	}
	if configMap.Name != "run-12345" {
		t.Errorf("want run-12345, got %s\n", configMap.Name)
	}
}
