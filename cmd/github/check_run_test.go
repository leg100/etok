package github

import (
	"testing"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCheckRun(t *testing.T) {
	//
	// Zero events
	//
	cr := checkRun{&v1alpha1.CheckRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "dev",
			Name:      "12345-networks",
		},
		Spec: v1alpha1.CheckRunSpec{},
		Status: v1alpha1.CheckRunStatus{
			Events: []*v1alpha1.CheckRunEvent{},
		},
	}}

	assert.Nil(t, cr.id())
	assert.Equal(t, "12345-networks-0", cr.etokRunName())
	assert.Equal(t, planCmd, cr.command())
	assert.Equal(t,
		"terraform init -no-color -input=false && terraform plan -no-color -input=false /plans/12345-networks-0",
		cr.script())
	assert.Equal(t, 0, cr.currentIteration())
	assert.False(t, cr.isCompleted())

	//
	// Event #1 (always a 'created' event)
	//
	cr.Status.Events = append(cr.Status.Events, &v1alpha1.CheckRunEvent{
		Created: &v1alpha1.CheckRunCreatedEvent{ID: 987},
	})

	assert.Equal(t, int64(987), *cr.id())
	assert.Equal(t, 0, cr.currentIteration())
	assert.False(t, cr.isCompleted())

	//
	// Event #2: completed
	//
	cr.Status.Events = append(cr.Status.Events, &v1alpha1.CheckRunEvent{
		Completed: &v1alpha1.CheckRunCompletedEvent{},
	})

	assert.Equal(t, int64(987), *cr.id())
	assert.Equal(t, 0, cr.currentIteration())
	assert.True(t, cr.isCompleted())

	//
	// Event #3: requested_action=apply
	//
	cr.Status.Events = append(cr.Status.Events, &v1alpha1.CheckRunEvent{
		RequestedAction: &v1alpha1.CheckRunRequestedActionEvent{Action: "apply"},
	})

	assert.Equal(t, int64(987), *cr.id())
	assert.Equal(t, 1, cr.currentIteration())
	assert.Equal(t, "12345-networks-1", cr.etokRunName())
	assert.Equal(t, applyCmd, cr.command())
	assert.Equal(t,
		"terraform init -no-color -input=false && terraform apply -no-color -input=false -out=/plans/12345-networks-0",
		cr.script())
	assert.False(t, cr.isCompleted())
}
