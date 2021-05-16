package github

import (
	"fmt"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
)

// checkRun is a wrapper around v1alpha1.CheckRun
type checkRun struct {
	*v1alpha1.CheckRun
}

// A check run only has an ID upon receiving a 'created' event
func (cr *checkRun) id() *int64 {
	if len(cr.Status.Events) == 0 {
		return nil
	}

	if cr.Status.Events[0].Created == nil {
		panic("expected first event to be a 'created' event")
	}

	return &cr.Status.Events[0].Created.ID
}

func (cr *checkRun) isCompleted() bool {
	if cr.currentEvent() != nil && cr.currentEvent().Completed != nil {
		return true
	}
	return false
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

// Determine the current command to run according to the most recently received
// event: plan is the default unless user has requested an apply
func (cr *checkRun) command() checkRunCommand {
	if cr.currentEvent().RequestedAction != nil && cr.currentEvent().RequestedAction.Action == "apply" {
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
		return fmt.Sprintf("%s && terraform plan -no-color -input=false %s", initCmd, cr.targetPlan())
	case applyCmd:
		return fmt.Sprintf("%s && terraform apply -no-color -input=false -out=%s", initCmd, cr.targetPlan())
	default:
		panic(fmt.Sprintf("unsupported check run command: %s", c))
	}
}
