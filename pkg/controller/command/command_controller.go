package command

import (
	"context"
	"path/filepath"

	v1alpha1 "github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	"github.com/operator-framework/operator-sdk/pkg/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_command")

// Add creates a new Command Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileCommand{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("command-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Command
	err = c.Watch(&source.Kind{Type: &v1alpha1.Command{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner Command
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1alpha1.Command{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to resource Workspace and requeue the associated Commands
	err = c.Watch(&source.Kind{Type: &v1alpha1.Workspace{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			rc := r.(*ReconcileCommand)
			cmdList := &v1alpha1.CommandList{}
			err = rc.client.List(context.TODO(), cmdList, client.InNamespace(a.Meta.GetNamespace()), client.MatchingLabels{
				"workspace": a.Meta.GetName(),
			})
			if err != nil {
				return []reconcile.Request{}
			}

			rr := []reconcile.Request{}
			for _, cmd := range cmdList.Items {
				rr = append(rr, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      cmd.GetName(),
						Namespace: cmd.GetNamespace(),
					},
				})
			}
			return rr
		}),
	})
	if err != nil {
		return err
	}
	return nil
}

// blank assignment to verify that ReconcileCommand implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileCommand{}

// ReconcileCommand reconciles a Command object
type ReconcileCommand struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Command object and makes changes based on the state read
// and what is in the Command.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileCommand) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.V(1).Info("Reconciling Command")

	// Fetch the Command instance
	instance := &v1alpha1.Command{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Fetch its Workspace object
	workspace := &v1alpha1.Workspace{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Labels["workspace"], Namespace: request.Namespace}, workspace)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Get workspace's secret resource
	secret := &corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: workspace.Spec.SecretName, Namespace: request.Namespace}, secret)
	if err != nil {
		return reconcile.Result{}, err
	}

	if isFirstInQueue(workspace.Status.Queue, instance.Name) {
		err = r.managePod(request, instance, secret)
		if err != nil {
			return reconcile.Result{}, err
		}
		val, ok := instance.Annotations["stok.goalspike.com/client"]
		if ok {
			if val == "Ready" {
				// logs are being streamed to the client, we can let terraform begin
				updated, err := r.updateCondition(instance, "ClientReady", corev1.ConditionTrue, "ClientReceivingLogs", "Logs are being streamed to the client")
				if err != nil {
					return reconcile.Result{}, err
				}
				if updated {
					reqLogger.Info("Client is ready to receive logs", "Request.Namespace", request.Namespace, "Request.Name", request.Name)
				}
			}
		}
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileCommand) updateCondition(command *v1alpha1.Command, conditionType string, conditionStatus corev1.ConditionStatus, reason string, msg string) (bool, error) {
	c := status.Condition{
		Type:    status.ConditionType(conditionType),
		Status:  conditionStatus,
		Reason:  status.ConditionReason(reason),
		Message: msg,
	}
	cc := command.Status.Conditions.GetCondition(c.Type)
	if cc != nil && cc.Status == c.Status {
		// condition with same type and status already exists, skip
		return false, nil
	} else {
		// either the condition is not set or the status is different
		// so set it anew
		_ = command.Status.Conditions.SetCondition(c)
		return true, r.client.Status().Update(context.TODO(), command)
	}
}

func isFirstInQueue(queue []string, name string) bool {
	if len(queue) > 0 && queue[0] == name {
		return true
	} else {
		return false
	}
}

func (r *ReconcileCommand) managePod(request reconcile.Request, command *v1alpha1.Command, secret *corev1.Secret) error {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)

	// Define a new Pod object
	pod, err := newPodForCR(command, secret)
	if err != nil {
		return err
	}

	// Set Command instance as the owner and controller
	if err := controllerutil.SetControllerReference(command, pod, r.scheme); err != nil {
		return err
	}

	// Check if this Pod already exists
	found := &corev1.Pod{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
		err = r.client.Create(context.TODO(), pod)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	switch found.Status.Phase {
	case corev1.PodFailed:
		_, err = r.updateCondition(command, "Completed", corev1.ConditionTrue, "PodFailed", "")
		if err != nil {
			return err
		}
	case corev1.PodSucceeded:
		_, err = r.updateCondition(command, "Completed", corev1.ConditionTrue, "PodSucceeded", "")
		if err != nil {
			return err
		}
	default:
		// PodPending PodPhase = "Pending"
		// PodRunning PodPhase = "Running"
		// PodUnknown PodPhase = "Unknown"
		return nil
	}

	return nil
}

// newPodForCR returns a busybox pod with the same name/namespace as the cr
func newPodForCR(cr *v1alpha1.Command, secret *corev1.Secret) (*corev1.Pod, error) {
	tfScript, err := generateScript(cr)
	if err != nil {
		return nil, err
	}

	labels := map[string]string{
		"app":       cr.Name,
		"workspace": cr.Labels["workspace"],
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: cr.Name,
			Containers: []corev1.Container{
				{
					Name:            "terraform",
					Image:           "leg100/terraform:latest",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         []string{"sh"},
					Args:            []string{"-o", "pipefail", "-ec", tfScript},
					Stdin:           true,
					Env: []corev1.EnvVar{
						{
							Name:  "GOOGLE_APPLICATION_CREDENTIALS",
							Value: "/credentials/google-credentials.json",
						},
					},
					TTY:                      true,
					TerminationMessagePolicy: "FallbackToLogsOnError",
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "workspace",
							MountPath: "/workspace",
						},
						{
							Name:      "cache",
							MountPath: "/workspace/.terraform",
						},
						{
							Name:      "credentials",
							MountPath: "/credentials",
						},
						{
							Name:      "tarball",
							MountPath: filepath.Join("/tarball", cr.Spec.ConfigMapKey),
							SubPath:   cr.Spec.ConfigMapKey,
						},
					},
					WorkingDir: "/workspace",
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "cache",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: cr.Labels["workspace"],
						},
					},
				},
				{
					Name: "workspace",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "tarball",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: cr.Spec.ConfigMap,
							},
						},
					},
				},
				{
					Name: "credentials",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: secret.Name,
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}, nil
}
