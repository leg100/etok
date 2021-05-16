package github

import (
	"context"
	"fmt"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/scheme"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// checkSuite runs accordingly.
type checkSuiteReconciler struct {
	Scheme *runtime.Scheme
	runtimeclient.Client
	*repoManager
}

// Constructor for run reconciler
func newCheckSuiteReconciler(client runtimeclient.Client, refresher tokenRefresher, cloneDir string) *checkSuiteReconciler {
	return &checkSuiteReconciler{
		Scheme:      scheme.Scheme,
		Client:      client,
		repoManager: newRepoManager(cloneDir, refresher),
	}
}

// +kubebuilder:rbac:groups=etok.dev,resources=runs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=etok.dev,resources=runs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete

func (r *checkSuiteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// set up a convenient log object so we don't have to type request over and
	// over again
	log := log.FromContext(ctx)
	log.V(1).Info("Reconciling")

	// Get checkSuite obj
	suite := &v1alpha1.CheckSuite{}
	if err := r.Get(ctx, req.NamespacedName, suite); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an
		// immediate requeue (we'll need to wait for a new notification), and we
		// can get them on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Find connected workspaces
	workspaces := &v1alpha1.WorkspaceList{}
	connected := workspaces
	if err := r.Client.List(ctx, workspaces); err != nil {
		return ctrl.Result{}, err
	}
	for _, ws := range workspaces.Items {
		if ws.Spec.VCS.Repository == suite.Spec.CloneURL {
			connected.Items = append(connected.Items, ws)
		}
	}

	if len(connected.Items) == 0 {
		// No point proceeding if there are no connected workspaces
		return ctrl.Result{}, nil
	}

	// Ensure repo is cloned
	repo, err := r.repoManager.clone(
		suite.Spec.CloneURL,
		suite.Spec.Branch,
		suite.Spec.SHA,
		suite.Spec.Owner,
		suite.Spec.Repo, suite.Spec.InstallID)
	if err != nil {
		return ctrl.Result{}, err
	}
	suite.Status.RepoPath = repo.path

	// Ensure there is a CheckRun for each connected workspace
	for _, ws := range connected.Items {
		check := &v1alpha1.CheckRun{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ws.Namespace,
				Name:      fmt.Sprintf("%d-%s", suite.Spec.CheckSuiteSuiteID, ws.Name),
			},
			Spec: v1alpha1.CheckSpec{
				CheckSuiteRef: suite.Name,
			},
		}

		if err := controllerutil.SetOwnerReference(suite, check, r.Scheme); err != nil {
			return ctrl.Result{}, nil
		}

		err := r.Client.Get(ctx, client.ObjectKeyFromObject(check), check)
		if kerrors.IsNotFound(err) {
			if err := r.Client.Create(ctx, check); err != nil {
				return ctrl.Result{}, nil
			}
		} else if err != nil {
			return ctrl.Result{}, nil
		}
	}

	return ctrl.Result{}, r.Status().Update(ctx, suite)
}

func (r *checkSuiteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	blder := ctrl.NewControllerManagedBy(mgr)

	// Watch for changes to primary resource CheckSuite
	blder = blder.For(&v1alpha1.CheckSuite{})

	return blder.Complete(r)
}
