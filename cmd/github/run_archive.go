package github

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/archive"
	"github.com/leg100/etok/pkg/builders"
	"github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/controllers"
	"github.com/leg100/etok/pkg/util"
	corev1 "k8s.io/api/core/v1"
)

// Combination of Run and ConfigMap resource
type runArchive struct {
	run     *v1alpha1.Run
	archive *corev1.ConfigMap
}

// runArchive constructor
func newRunArchive(ws *v1alpha1.Workspace, command, previous, repo string, status *v1alpha1.RunStatus) (*runArchive, error) {
	id := fmt.Sprintf("run-%s", util.GenerateRandomString(5))

	script := runScript(id, command, previous)

	bldr := builders.Run(ws.Namespace, id, ws.Name, "sh", "-c", script)
	if status != nil {
		// For testing purposes seed status
		bldr.SetStatus(*status)
	}

	configMap, err := archive.ConfigMap(ws.Namespace, id, filepath.Join(repo, ws.Spec.VCS.WorkingDir), repo)
	if err != nil {
		return nil, err
	}

	return &runArchive{
		run:     bldr.Build(),
		archive: configMap,
	}, nil
}

// Create Run and ConfigMap resources in k8s
func (ra *runArchive) create(client *client.Client) error {
	_, err := client.RunsClient(ra.run.Namespace).Create(context.Background(), ra.run, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	_, err = client.ConfigMapsClient(ra.archive.Namespace).Create(context.Background(), ra.archive, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func runScript(id, command, previous string) string {
	script := new(bytes.Buffer)

	// Default is to create a new plan file with a filename the same as the etok
	// run ID
	planPath := filepath.Join(controllers.PlansMountPath, id)
	if command == "apply" {
		// Apply uses the plan file from the previous run
		planPath = filepath.Join(controllers.PlansMountPath, previous)
	}

	if err := generateEtokRunScript(script, planPath, command); err != nil {
		panic("unable to generate check run script: " + err.Error())
	}

	return script.String()
}
