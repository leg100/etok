package k8s

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/leg100/etok/pkg/testutil"
	"gotest.tools/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDeploymentIsReady(t *testing.T) {
	tests := []struct {
		name   string
		deploy *appsv1.Deployment
		err    error
	}{
		{
			name:   "successful",
			deploy: successfulDeploy(),
		},
		{
			name:   "failure",
			deploy: deploy(),
			err:    wait.ErrWaitTimeout,
		},
	}
	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			client := fake.NewClientBuilder().WithObjects(runtimeclient.Object(tt.deploy)).Build()

			assert.Equal(t, tt.err, DeploymentIsReady(context.Background(), client, "etok", "etok", 100*time.Millisecond, 10*time.Millisecond))
		})
	}
}

func deploy() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "etok",
			Namespace: "etok",
		},
	}
}

func successfulDeploy() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "etok",
			Namespace: "etok",
		},
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:               appsv1.DeploymentAvailable,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: metav1.Time{Time: time.Now().Add(-11 * time.Second)},
				},
			},
		},
	}
}
