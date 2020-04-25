package command

import (
	"context"
	"fmt"
	"path/filepath"

	terraformv1alpha1 "github.com/leg100/stok/pkg/apis/terraform/v1alpha1"
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

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

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
	err = c.Watch(&source.Kind{Type: &terraformv1alpha1.Command{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Command
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &terraformv1alpha1.Command{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to resource Workspace and requeue the associated Commands
	err = c.Watch(&source.Kind{Type: &terraformv1alpha1.Workspace{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			rc := r.(*ReconcileCommand)
			cmdList := &terraformv1alpha1.CommandList{}
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
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileCommand) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Command")

	// Fetch the Command instance
	instance := &terraformv1alpha1.Command{}
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
	workspace := &terraformv1alpha1.Workspace{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Labels["workspace"], Namespace: request.Namespace}, workspace)
	if err != nil {
		if errors.IsNotFound(err) {
			err = r.updateCondition(instance, "Ready", corev1.ConditionFalse, "WorkspaceNotFound", fmt.Sprintf("Workspace %s not found", workspace.Name))
			if err != nil {
				return reconcile.Result{}, err
			} else {
				// Don't requeue; wait until workspace obj triggers a request
				return reconcile.Result{}, nil
			}
		}
		return reconcile.Result{}, err
	}

	// Get workspace's secret resource
	secret := &corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: workspace.Spec.SecretName, Namespace: request.Namespace}, secret)
	if err != nil {
		return reconcile.Result{}, err
	}

	err = r.updateCondition(instance, "Ready", corev1.ConditionTrue, "WorkspaceFound", fmt.Sprintf("Workspace %s found", workspace.Name))
	if err != nil {
		return reconcile.Result{}, err
	}

	if isFirstInQueue(workspace.Status.Queue, instance.Name) {
		err = r.managePod(request, instance, secret)
		if err != nil {
			return reconcile.Result{}, err
		}
		reqLogger.Info("Looking up ClientReady Annotation", "Request.Namespace", request.Namespace, "Request.Name", request.Name)
		val, ok := instance.Annotations["stok.goalspike.com/client"]
		if ok {
			reqLogger.Info("ClientReady Annotation is not set")
			if val == "Ready" {
				reqLogger.Info("Setting ClientReady", "Request.Namespace", request.Namespace, "Request.Name", request.Name)
				// logs are being streamed to the client, we can let terraform begin
				err = r.updateCondition(instance, "ClientReady", corev1.ConditionTrue, "ClientReceivingLogs", "Logs are being streamed to the client")
				if err != nil {
					return reconcile.Result{}, err
				}
			}
		}
	} else {
		err = r.updateCondition(instance, "Active", corev1.ConditionFalse, "Enqueued", "TODO: Provide queue position")
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileCommand) updateCondition(command *terraformv1alpha1.Command, conditionType string, conditionStatus corev1.ConditionStatus, reason string, msg string) error {
	condition := status.Condition{
		Type:    status.ConditionType(conditionType),
		Status:  conditionStatus,
		Reason:  status.ConditionReason(reason),
		Message: msg,
	}
	_ = command.Status.Conditions.SetCondition(condition)
	return r.client.Status().Update(context.TODO(), command)
}

func isFirstInQueue(queue []string, name string) bool {
	if len(queue) > 0 && queue[0] == name {
		return true
	} else {
		return false
	}
}

func (r *ReconcileCommand) managePod(request reconcile.Request, command *terraformv1alpha1.Command, secret *corev1.Secret) error {
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
		err = r.updateCondition(command, "Active", corev1.ConditionTrue, "PodCreated", "")
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		switch found.Status.Phase {
		case corev1.PodFailed:
			err = r.updateCondition(command, "Active", corev1.ConditionFalse, "PodFailed", "")
			if err != nil {
				return err
			}
			err = r.updateCondition(command, "Completed", corev1.ConditionTrue, "PodFailed", "")
			if err != nil {
				return err
			}
		case corev1.PodSucceeded:
			err = r.updateCondition(command, "Active", corev1.ConditionFalse, "PodSucceeded", "")
			if err != nil {
				return err
			}
			err = r.updateCondition(command, "Completed", corev1.ConditionTrue, "PodSucceeded", "")
			if err != nil {
				return err
			}
		default:
			// PodPending PodPhase = "Pending"
			// PodRunning PodPhase = "Running"
			// PodUnknown PodPhase = "Unknown"
			err = r.updateCondition(command, "Active", corev1.ConditionTrue, string(found.Status.Phase), "")
			if err != nil {
				return err
			}
		}
		// Pod already exists - don't requeue
		reqLogger.Info("Skip reconcile: Pod already exists", "Pod.Namespace", found.Namespace, "Pod.Name", found.Name)
	}
	return nil
}

// newPodForCR returns a busybox pod with the same name/namespace as the cr
func newPodForCR(cr *terraformv1alpha1.Command, secret *corev1.Secret) (*corev1.Pod, error) {
	// TODO: fold tarScript and initcontainers into one and only one main container
	tarScript := fmt.Sprintf("tar zxf /tarball/%s -C /workspace", cr.Spec.ConfigMapKey)

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
			InitContainers: []corev1.Container{
				{
					Name:                     "get-tarball",
					Image:                    "busybox",
					ImagePullPolicy:          corev1.PullIfNotPresent,
					Command:                  []string{"sh"},
					Args:                     []string{"-ec", tarScript},
					TerminationMessagePolicy: "FallbackToLogsOnError",
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "tarball",
							MountPath: filepath.Join("/tarball", cr.Spec.ConfigMapKey),
							SubPath:   cr.Spec.ConfigMapKey,
						},
						{
							Name:      "workspace",
							MountPath: "/workspace",
						},
					},
					WorkingDir: "/workspace",
				},
				{
					Name:                     "kubectl",
					Image:                    "bitnami/kubectl:1.17",
					ImagePullPolicy:          corev1.PullIfNotPresent,
					Command:                  []string{"sh"},
					Args:                     []string{"-ec", "cp /opt/bitnami/kubectl/bin/kubectl /kubectl"},
					TerminationMessagePolicy: "FallbackToLogsOnError",
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "kubectl",
							MountPath: "/kubectl",
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:    "terraform",
					Image:   "hashicorp/terraform:0.12.21",
					Command: []string{"sh"},
					Args:  []string{"-eco pipefail", tfScript},
					Stdin: true,
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
							Name:      "kubectl",
							MountPath: "/kubectl",
						},
					},
					WorkingDir: "/workspace",
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "kubectl",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
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
