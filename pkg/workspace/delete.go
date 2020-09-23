package workspace

import (
	"context"

	"github.com/apex/log"
	"github.com/leg100/stok/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DeleteWorkspace struct {
	Name      string
	Namespace string
	Path      string
	Context   string
}

func (t *DeleteWorkspace) Run(ctx context.Context) error {
	sc, err := k8s.StokClient()
	if err != nil {
		return err
	}

	if err = sc.StokV1alpha1().Workspaces(t.Namespace).Delete(ctx, t.Name, metav1.DeleteOptions{}); err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"workspace": t.Name,
		"namespace": t.Namespace,
	}).Info("Deleted workspace")

	return nil
}
