package workspace

import (
	"context"

	"github.com/apex/log"
	"github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DeleteWorkspace struct {
	Name      string
	Namespace string
	Path      string
	Context   string

	Factory k8s.FactoryInterface
}

func (t *DeleteWorkspace) Run(ctx context.Context) error {
	config, err := t.Factory.NewConfig(t.Context)
	if err != nil {
		return err
	}

	// Controller-runtime client for constructing workspace resource
	rc, err := t.Factory.NewClient(config)
	if err != nil {
		return err
	}

	if err = t.deleteWorkspace(rc); err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"workspace": t.Name,
		"namespace": t.Namespace,
	}).Info("Deleted workspace")

	return nil
}

func (t *DeleteWorkspace) deleteWorkspace(rc client.Client) error {
	return rc.Delete(context.TODO(), &v1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      t.Name,
			Namespace: t.Namespace,
		},
	})
}
