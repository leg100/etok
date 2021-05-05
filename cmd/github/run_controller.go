package github

import (
	"context"
	"fmt"
	"io"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/globals"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Streamer for streaming logs from k8s. Abstracted to an interface for testing
// purposes.
type streamer interface {
	Stream(context.Context, client.ObjectKey) (io.ReadCloser, error)
}

type podStreamer struct {
	client kubernetes.Interface
}

func (s *podStreamer) Stream(ctx context.Context, key client.ObjectKey) (io.ReadCloser, error) {
	opts := corev1.PodLogOptions{Container: globals.RunnerContainerName}

	return s.client.CoreV1().Pods(key.Namespace).GetLogs(key.Name, &opts).Stream(ctx)
}

// Reconciler for watching updates to runs and updating corresponding github
// check runs accordingly.
type runReconciler struct {
	client.Client
	sender
	streamer
}

// Constructor for run reconciler
func newRunReconciler(rclient client.Client, kclient kubernetes.Interface, sdr sender) *runReconciler {
	return &runReconciler{
		Client:   rclient,
		sender:   sdr,
		streamer: &podStreamer{client: kclient},
	}
}

// +kubebuilder:rbac:groups=etok.dev,resources=runs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=etok.dev,resources=runs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete

func (r *runReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// set up a convenient log object so we don't have to type request over and
	// over again
	log := log.FromContext(ctx)
	log.V(1).Info("Reconciling")

	// Get run obj
	var run v1alpha1.Run
	if err := r.Get(ctx, req.NamespacedName, &run); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an
		// immediate requeue (we'll need to wait for a new notification), and we
		// can get them on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Skip completed runs
	if val, ok := run.GetLabels()[checkStatusLabelName]; ok && val == "completed" {
		return ctrl.Result{}, nil
	}

	// Construct check-run from run resource
	checkRun, err := newCheckFromResource(&run, nil)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Stream logs from pod and copy to check run obj
	if run.IsStreamable() {
		resp, err := r.Stream(ctx, req.NamespacedName)
		if err != nil {
			return ctrl.Result{}, err
		}

		out, err := io.ReadAll(resp)
		defer resp.Close()
		if err != nil {
			return ctrl.Result{}, err
		}
		// Copy logs into check-run's buffer
		checkRun.out = out
	}

	// Get github install ID from run
	installID, err := getInstallID(&run)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Send checkrun obj to github
	if err := r.send(installID, checkRun); err != nil {
		return ctrl.Result{}, err
	}

	if run.IsDone() {
		// Add label so that reconciler knows it can be skipped in future
		run.SetLabels(labels.Merge(run.GetLabels(), map[string]string{checkStatusLabelName: "completed"}))
		if err := r.Update(ctx, &run); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *runReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// We're only interested in resources triggered as the result of github
	// events
	pred, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{MatchLabels: map[string]string{githubTriggeredLabelName: "true"}})
	if err != nil {
		return err
	}

	// Watch changes to run resources
	blder := ctrl.NewControllerManagedBy(mgr).For(&v1alpha1.Run{}, builder.WithPredicates(pred))

	return blder.Complete(r)
}

func requestFromObject(obj client.Object) reconcile.Request {
	return reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
		},
	}
}

func getInstallID(run *v1alpha1.Run) (int64, error) {
	// Get relevant github client for the installation
	id, ok := run.GetLabels()[githubAppInstallIDLabelName]
	if !ok {
		return 0, fmt.Errorf("run missing label %s", githubAppInstallIDLabelName)
	}

	return strconv.ParseInt(id, 10, 64)
}
