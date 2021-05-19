package github

import (
	"context"
	"io"
	"path/filepath"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/archive"
	"github.com/leg100/etok/pkg/builders"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// check runs accordingly.
type checkRunReconciler struct {
	client.Client
	sender
	streamer
	stripRefreshing bool
}

// Constructor for run reconciler
func newCheckRunReconciler(rclient client.Client, kclient kubernetes.Interface, sdr sender, stripRefreshing bool) *checkRunReconciler {
	return &checkRunReconciler{
		Client:          rclient,
		sender:          sdr,
		streamer:        &podStreamer{client: kclient},
		stripRefreshing: stripRefreshing,
	}
}

// +kubebuilder:rbac:groups=etok.dev,resources=checkruns,verbs=get;update;patch
// +kubebuilder:rbac:groups=etok.dev,resources=checksuites,verbs=get
// +kubebuilder:rbac:groups=etok.dev,resources=workspaces,verbs=get
// +kubebuilder:rbac:groups=etok.dev,resources=runs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=create
// +kubebuilder:rbac:groups="",resources=pods/logs,verbs=get

func (r *checkRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// set up a convenient log object so we don't have to type request over and
	// over again
	log := log.FromContext(ctx)
	log.V(1).Info("Reconciling")

	// Get check run resource
	res := &v1alpha1.CheckRun{}
	if err := r.Get(ctx, req.NamespacedName, res); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an
		// immediate requeue (we'll need to wait for a new notification), and we
		// can get them on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// Wrap resource
	cr := &checkRun{res}

	if cr.isCompleted() {
		// Check Run has completed (its current iteration) so nothing more to be
		// done
		return ctrl.Result{}, nil
	}

	// Get its check suite resource
	suite := &v1alpha1.CheckSuite{}
	if err := r.Get(ctx, client.ObjectKey{Name: cr.Spec.CheckSuiteRef}, suite); err != nil {
		return ctrl.Result{}, err
	}

	// Get its check suite's workspace resource
	ws := &v1alpha1.Workspace{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: cr.Spec.Workspace}, ws); err != nil {
		return ctrl.Result{}, err
	}

	// Check if Run resource exists for current iteration.
	run := &v1alpha1.Run{}
	var runNotFound bool
	if err := r.Get(ctx, client.ObjectKey{Namespace: cr.Namespace, Name: cr.etokRunName()}, run); err != nil {
		if kerrors.IsNotFound(err) {
			runNotFound = true
		} else {
			return ctrl.Result{}, err
		}
	}

	// Create Run resources / copy its logs. Any error is relayed to github.
	var logs = make([]byte, 0)
	var reconcileErr error
	if runNotFound {
		if err := r.createRunResources(ctx, suite, cr, ws); err != nil {
			reconcileErr = err
		}
	} else {
		if run.IsStreamable() {
			resp, err := r.Stream(ctx, req.NamespacedName)
			if err != nil {
				reconcileErr = err
			} else {
				logs, err = io.ReadAll(resp)
				defer resp.Close()
				if err != nil {
					reconcileErr = err
				}
			}
		}
	}

	// Construct update and send to github
	update := &checkRunUpdate{
		checkRun:     cr,
		suite:        suite,
		ws:           ws,
		run:          run,
		logs:         logs,
		reconcileErr: reconcileErr,
		maxFieldSize: defaultMaxFieldSize,
	}
	if err := r.send(suite.Spec.InstallID, update); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, reconcileErr
}

func (r *checkRunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	blder := ctrl.NewControllerManagedBy(mgr)

	// Watch for changes to primary resource Check
	blder = blder.For(&v1alpha1.CheckRun{})

	// Watch owned runs
	blder = blder.Owns(&v1alpha1.Run{})

	return blder.Complete(r)
}

// Create Run and ConfigMap resources in k8s
func (r *checkRunReconciler) createRunResources(ctx context.Context, suite *v1alpha1.CheckSuite, cr *checkRun, ws *v1alpha1.Workspace) error {
	configMap, err := archive.ConfigMap(cr.Namespace, cr.etokRunName(), filepath.Join(suite.Status.RepoPath, ws.Spec.VCS.WorkingDir), suite.Status.RepoPath)
	if err != nil {
		return err
	}

	run := builders.Run(cr.Namespace, cr.etokRunName(), ws.Name, "sh", cr.script()).Build()
	if err := r.Client.Create(ctx, run); err != nil {
		return err
	}

	if err := r.Client.Create(ctx, configMap); err != nil {
		return err
	}

	return nil
}
