package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/leg100/stok/api/command"
	v1alpha1 "github.com/leg100/stok/api/v1alpha1"
	"github.com/leg100/stok/scheme"
	"github.com/leg100/stok/util/slice"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type CommandReconciler struct {
	client.Client
	Kind         string
	ResourceType string
	Scheme       *runtime.Scheme
	Log          logr.Logger
	Image        string
}

func NewCommandReconciler(c client.Client, kind, image string) *CommandReconciler {
	return &CommandReconciler{
		Client:       c,
		Scheme:       scheme.Scheme,
		Kind:         kind,
		ResourceType: command.CommandKindToType(kind),
		Log:          ctrl.Log.WithName("controllers").WithName(command.CommandKindToCLI(kind)),
		Image:        image,
	}
}

func (r *CommandReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues(r.ResourceType, req.NamespacedName)

	// Create command obj of kind
	cmd, err := command.NewCommandFromGVK(r.Scheme, v1alpha1.SchemeGroupVersion.WithKind(r.Kind))
	if err != nil {
		return ctrl.Result{}, err
	}

	// Fetch command obj
	if err := r.Get(context.TODO(), req.NamespacedName, cmd); err != nil {
		log.Error(err, "unable to fetch command obj")

		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Command completed, nothing more to be done
	if cmd.GetConditions().IsTrueFor(v1alpha1.ConditionCompleted) {
		return ctrl.Result{}, nil
	}

	// Fetch its Workspace object
	ws := &v1alpha1.Workspace{}
	if err := r.Get(context.TODO(), types.NamespacedName{Name: cmd.GetWorkspace(), Namespace: req.Namespace}, ws); err != nil {
		return ctrl.Result{}, err
	}

	// Check workspace queue position
	pos := slice.StringIndex(ws.Status.Queue, cmd.GetName())
	switch {
	case pos == 0:
		// Currently scheduled to run; get or create pod
		opts := podOpts{
			cmd:                cmd,
			workspaceName:      ws.GetName(),
			serviceAccountName: ws.Spec.ServiceAccountName,
			secretName:         ws.Spec.SecretName,
			pvcName:            ws.GetName(),
			configMapName:      cmd.GetConfigMap(),
			configMapKey:       cmd.GetConfigMapKey(),
		}
		return r.reconcilePod(req, &opts)
	case pos > 0:
		// Queued
		cmd.SetPhase(v1alpha1.CommandPhaseQueued)
	case pos < 0:
		// Not yet queued
		cmd.SetPhase(v1alpha1.CommandPhasePending)
	}

	return reconcile.Result{}, r.Status().Update(context.TODO(), cmd)
}

func (r *CommandReconciler) SetupWithManager(mgr ctrl.Manager) error {
	gvk := v1alpha1.SchemeGroupVersion.WithKind(r.Kind)
	o, err := r.Scheme.New(gvk)
	if err != nil {
		return err
	}

	oList, err := r.Scheme.New(v1alpha1.SchemeGroupVersion.WithKind(command.CollectionKind(r.Kind)))
	if err != nil {
		return err
	}

	// Watch changes to primary command resources
	blder := ctrl.NewControllerManagedBy(mgr).For(o)

	// Watch for changes to secondary resource Pods and requeue the owner command resource
	blder = blder.Owns(&corev1.Pod{})

	// Index field Spec.Workspace in order for the filtered watch below to work (I don't think it
	// works without this...)
	_ = mgr.GetFieldIndexer().IndexField(context.TODO(), o, "spec.workspace", func(o runtime.Object) []string {
		ws := o.(command.Interface).GetWorkspace()
		if ws == "" {
			return nil
		}
		return []string{ws}
	})

	// Watch for changes to resource Workspace and requeue the associated commands
	blder = blder.Watches(&source.Kind{Type: &v1alpha1.Workspace{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			err = r.List(context.TODO(), oList, client.InNamespace(a.Meta.GetNamespace()), client.MatchingFields{
				"spec.workspace": a.Meta.GetName(),
			})
			if err != nil {
				return []reconcile.Request{}
			}

			rr := []reconcile.Request{}
			meta.EachListItem(oList, func(o runtime.Object) error {
				cmd := o.(command.Interface)
				rr = append(rr, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      cmd.GetName(),
						Namespace: cmd.GetNamespace(),
					},
				})
				return nil
			})
			return rr
		}),
	})

	return blder.Complete(r)
}
