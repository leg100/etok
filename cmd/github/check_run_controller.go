package github

import (
	"context"
	"io"
	"path/filepath"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/cmd/github/client"
	"github.com/leg100/etok/pkg/archive"
	"github.com/leg100/etok/pkg/builders"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	checkrunControllerLabels = map[string]string{
		"app.kubernetes.io/created-by": "checkrun-controller",
	}
)

type sender interface {
	Send(int64, string, client.Invokable) error
}

// check runs accordingly.
type checkRunReconciler struct {
	runtimeclient.Client
	sender
	streamer
	stripRefreshing bool
}

// Constructor for run reconciler
func newCheckRunReconciler(rclient runtimeclient.Client, kclient kubernetes.Interface, sdr sender, stripRefreshing bool) *checkRunReconciler {
	return &checkRunReconciler{
		Client:          rclient,
		sender:          sdr,
		streamer:        &podStreamer{client: kclient},
		stripRefreshing: stripRefreshing,
	}
}

// +kubebuilder:rbac:groups=etok.dev,resources=checkruns,verbs=get;list;watch
// +kubebuilder:rbac:groups=etok.dev,resources=checkruns/status,verbs=update;patch
// +kubebuilder:rbac:groups=etok.dev,resources=checksuites,verbs=get
// +kubebuilder:rbac:groups=etok.dev,resources=workspaces,verbs=get
// +kubebuilder:rbac:groups=etok.dev,resources=runs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=create
// +kubebuilder:rbac:groups="",resources=pods/logs,verbs=get

func (r *checkRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// set up a convenient log object so we don't have to type request over and
	// over again
	log := log.FromContext(ctx)
	log.V(3).Info("Reconciling")

	// Get check run resource
	res := &v1alpha1.CheckRun{}
	if err := r.Get(ctx, req.NamespacedName, res); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an
		// immediate requeue (we'll need to wait for a new notification), and we
		// can get them on deleted requests.
		return ctrl.Result{}, runtimeclient.IgnoreNotFound(err)
	}
	// Wrap resource
	cr := &checkRun{res}

	// Go no further if current iteration has completed
	if cr.isCompleted() {
		return ctrl.Result{}, nil
	}

	// Get its check suite resource
	suite := &v1alpha1.CheckSuite{}
	if err := r.Get(ctx, runtimeclient.ObjectKey{Name: cr.Spec.CheckSuiteRef}, suite); err != nil {
		return ctrl.Result{}, err
	}

	// Get its check suite's workspace resource
	ws := &v1alpha1.Workspace{}
	if err := r.Get(ctx, runtimeclient.ObjectKey{Namespace: req.Namespace, Name: cr.Spec.Workspace}, ws); err != nil {
		return ctrl.Result{}, err
	}

	// Construct run obj
	run := &v1alpha1.Run{}
	run.SetNamespace(cr.Namespace)
	run.SetName(cr.etokRunName())

	// Check if Run resource exists for current iteration.
	var runNotFound bool
	if err := r.Get(ctx, runtimeclient.ObjectKeyFromObject(run), run); err != nil {
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
	} else if run.IsStreamable() {
		resp, err := r.Stream(ctx, runtimeclient.ObjectKeyFromObject(run))
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

	// Construct update
	update := &checkRunUpdate{
		checkRun:     cr,
		suite:        suite,
		ws:           ws,
		run:          run,
		logs:         logs,
		reconcileErr: reconcileErr,
		maxFieldSize: defaultMaxFieldSize,
	}

	// Ensure multiple check runs are not created in the GH API!
	send := false
	if cr.isCreated() {
		send = true
	} else if !cr.isCreateRequested() {
		cr.setCreateRequested()
		send = true
	}

	// Update status info
	cr.setStatus(update.status())
	cr.setConclusion(update.conclusion())
	cr.setIterationStatus(update.status() == "completed")

	if err := r.Status().Update(ctx, cr.CheckRun); err != nil {
		return ctrl.Result{}, err
	}

	// Send update to Github API
	if send {
		if err := r.Send(suite.Spec.InstallID, "github.com", update); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Complete reconcile. Any error from earlier will be logged and will
	// trigger another reconcile.
	return ctrl.Result{}, reconcileErr
}

func (r *checkRunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	blder := ctrl.NewControllerManagedBy(mgr)

	// Watch for changes to primary resource Check
	blder = blder.For(&v1alpha1.CheckRun{})

	blder = blder.Watches(&source.Kind{Type: &v1alpha1.Run{}}, &handler.EnqueueRequestForOwner{OwnerType: &v1alpha1.CheckRun{}, IsController: false})

	return blder.Complete(r)
}

// Create Run and ConfigMap resources in k8s
func (r *checkRunReconciler) createRunResources(ctx context.Context, suite *v1alpha1.CheckSuite, cr *checkRun, ws *v1alpha1.Workspace) error {
	configMap, err := archive.ConfigMap(cr.Namespace, cr.etokRunName(), filepath.Join(suite.Status.RepoPath, ws.Spec.VCS.WorkingDir), suite.Status.RepoPath)
	if err != nil {
		return err
	}

	// Build run resource
	runBldr := builders.Run(cr.Namespace, cr.etokRunName(), ws.Name, "sh", cr.script())
	for k, v := range checkrunControllerLabels {
		runBldr = runBldr.SetLabel(k, v)
	}
	run := runBldr.Build()

	if err := controllerutil.SetOwnerReference(cr.CheckRun, run, r.Scheme()); err != nil {
		return err
	}

	if err := r.Client.Create(ctx, run); err != nil {
		return err
	}

	if err := r.Client.Create(ctx, configMap); err != nil {
		return err
	}

	return nil
}
