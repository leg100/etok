package github

import (
	"fmt"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// checkRun is a wrapper around v1alpha1.CheckRun
type checkRun struct {
	*v1alpha1.CheckRun
}

// A check run only has an ID upon receiving a Created event
func (cr *checkRun) id() *int64 {
	for _, ev := range cr.Status.Events {
		if ev.Created != nil {
			return &ev.Created.ID
		}
	}
	return nil
}

func (cr *checkRun) isCreated() bool {
	return cr.id() != nil
}

func (cr *checkRun) isCreateRequested() bool {
	return meta.FindStatusCondition(cr.Status.Conditions, "CreateRequested") != nil
}

func (cr *checkRun) setCreateRequested() {
	meta.SetStatusCondition(&cr.CheckRun.Status.Conditions, metav1.Condition{
		Type:    "CreateRequested",
		Status:  metav1.ConditionTrue,
		Reason:  "CreateRequestSent",
		Message: "Create request has been sent to the Github API",
	})
}

// Has current iteration completed?
func (cr *checkRun) isCompleted() bool {
	if len(cr.Status.Iterations) != cr.currentIteration()+1 {
		return false
	}
	return cr.Status.Iterations[cr.currentIteration()].Completed
}

// Set status
func (cr *checkRun) setStatus(status string) {
	cr.CheckRun.Status.Status = status
}

// Set conclusion
func (cr *checkRun) setConclusion(conclusion *string) {
	cr.CheckRun.Status.Conclusion = conclusion
}

func (cr *checkRun) currentEvent() *v1alpha1.CheckRunEvent {
	if len(cr.Status.Events) > 0 {
		return cr.Status.Events[len(cr.Status.Events)-1]
	}
	return nil
}

func (cr *checkRun) etokRunName() string {
	return cr.etokRunNameByIteration(cr.currentIteration())
}

func (cr *checkRun) etokRunNameByIteration(i int) string {
	return fmt.Sprintf("%s-%d", cr.Name, i)
}

func (cr *checkRun) currentIteration() (i int) {
	for _, ev := range cr.Status.Events {
		if ev.Rerequested != nil || ev.RequestedAction != nil {
			i++
		}
	}
	return
}

// Set status of current iteration
func (cr *checkRun) setIterationStatus(completed bool) {
	// Ensure iterations status is populated first
	for i := len(cr.Status.Iterations); i < cr.currentIteration()+1; i++ {
		cr.Status.Iterations = append(cr.Status.Iterations, &v1alpha1.CheckRunIteration{
			Run: cr.etokRunNameByIteration(i),
		})
	}
	cr.Status.Iterations[cr.currentIteration()].Completed = completed
}

// Determine the current command to run according to the most recently received
// event: plan is the default unless user has requested an apply
func (cr *checkRun) command() checkRunCommand {
	if cr.currentEvent() != nil && cr.currentEvent().RequestedAction != nil && cr.currentEvent().RequestedAction.Action == "apply" {
		return applyCmd
	}
	return planCmd
}

// Get path to plan file for use with the check run comand
func (cr *checkRun) targetPlan() planPath {
	switch c := cr.command(); c {
	case planCmd:
		return newPlanPath(cr.etokRunName())
	case applyCmd:
		// Apply references plan produced by the previous iteration
		return newPlanPath(cr.etokRunNameByIteration(cr.currentIteration() - 1))
	default:
		panic(fmt.Sprintf("unsupported check run command: %s", c))
	}
}

// Generate script to be executed for this check run
func (cr *checkRun) script() string {
	initCmd := "terraform init -no-color -input=false"

	switch c := cr.command(); c {
	case planCmd:
		return fmt.Sprintf("%s && terraform plan -no-color -input=false -out=%s", initCmd, cr.targetPlan())
	case applyCmd:
		return fmt.Sprintf("%s && terraform apply -no-color -input=false %s", initCmd, cr.targetPlan())
	default:
		panic(fmt.Sprintf("unsupported check run command: %s", c))
	}
}
