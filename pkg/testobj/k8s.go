package testobj

import (
	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/globals"
	"github.com/leg100/stok/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Workspace(namespace, name string, opts ...func(*v1alpha1.Workspace)) *v1alpha1.Workspace {
	ws := &v1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, o := range opts {
		o(ws)
	}
	return ws
}

func WithPrivilegedCommands(cmds ...string) func(*v1alpha1.Workspace) {
	return func(ws *v1alpha1.Workspace) {
		ws.Spec.PrivilegedCommands = cmds
	}
}

func WithSecret(secret string) func(*v1alpha1.Workspace) {
	return func(ws *v1alpha1.Workspace) {
		ws.Spec.SecretName = secret
	}
}

func WithServiceAccount(account string) func(*v1alpha1.Workspace) {
	return func(ws *v1alpha1.Workspace) {
		ws.Spec.ServiceAccountName = account
	}
}

func WithHandshake(timeout string) func(*v1alpha1.Workspace) {
	return func(ws *v1alpha1.Workspace) {
		ws.Spec.AttachSpec = v1alpha1.AttachSpec{
			Handshake:        true,
			HandshakeTimeout: timeout,
		}
	}
}

func WithActive(run string) func(*v1alpha1.Workspace) {
	return func(ws *v1alpha1.Workspace) {
		ws.Status.Active = run
	}
}

func WithQueue(run ...string) func(*v1alpha1.Workspace) {
	return func(ws *v1alpha1.Workspace) {
		ws.Status.Queue = run
	}
}

func WithStorageClass(class string) func(*v1alpha1.Workspace) {
	return func(ws *v1alpha1.Workspace) {
		ws.Spec.Cache.StorageClass = class
	}
}

func WithBackendType(t string) func(*v1alpha1.Workspace) {
	return func(ws *v1alpha1.Workspace) {
		ws.Spec.Backend.Type = t
	}
}

func WithBackendConfig(cfg map[string]string) func(*v1alpha1.Workspace) {
	return func(ws *v1alpha1.Workspace) {
		ws.Spec.Backend.Config = cfg
	}
}

func WithApprovals(run ...string) func(*v1alpha1.Workspace) {
	return func(ws *v1alpha1.Workspace) {
		if ws.Annotations == nil {
			ws.Annotations = make(map[string]string)
		}
		for _, r := range run {
			ws.Annotations[v1alpha1.ApprovedAnnotationKey(r)] = "approved"
		}
	}
}

func RunPod(namespace, name string, opts ...func(*corev1.Pod)) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					// NOTE: The pod is both running and terminated in order to pass tests. The
					// alternative is to use a complicated set of reactors, which are known not to
					// play well with k8s informers:
					// https://github.com/kubernetes/kubernetes/pull/95897
					Name: globals.RunnerContainerName,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 0,
						},
					},
				},
			},
		},
	}
	for _, option := range opts {
		option(pod)
	}
	return pod
}

func WorkspacePod(namespace, name string, opts ...func(*corev1.Pod)) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1alpha1.WorkspacePodName(name),
			Namespace: namespace,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					// NOTE: The pod is both running and terminated in order to pass tests. The
					// alternative is to use a complicated set of reactors, which are known not to
					// play well with k8s informers:
					// https://github.com/kubernetes/kubernetes/pull/95897
					Name: globals.RunnerContainerName,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 0,
						},
					},
				},
			},
		},
	}
	for _, option := range opts {
		option(pod)
	}
	return pod
}

func WithPhase(phase corev1.PodPhase) func(*corev1.Pod) {
	return func(pod *corev1.Pod) {
		// Only set a phase if non-empty
		if phase != "" {
			pod.Status.Phase = phase
		}
	}
}

func WithExitCode(code int32) func(*corev1.Pod) {
	return func(pod *corev1.Pod) {
		k8s.ContainerStatusByName(pod, globals.RunnerContainerName).State.Terminated.ExitCode = code
	}
}

func Run(namespace, name string, command string, opts ...func(*v1alpha1.Run)) *v1alpha1.Run {
	run := &v1alpha1.Run{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		RunSpec: v1alpha1.RunSpec{
			Command: command,
		},
	}

	for _, o := range opts {
		o(run)
	}

	return run
}

func WithWorkspace(workspace string) func(*v1alpha1.Run) {
	return func(run *v1alpha1.Run) {
		run.RunSpec.Workspace = workspace
	}
}

func WithRunPhase(phase v1alpha1.RunPhase) func(*v1alpha1.Run) {
	return func(run *v1alpha1.Run) {
		// Only set a phase if non-empty
		if phase != "" {
			run.Phase = phase
		}
	}
}

func Secret(namespace, name string, opts ...func(*corev1.Secret)) *corev1.Secret {
	var secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		StringData: map[string]string{
			"google_application_credentials.json": "abc",
		},
	}
	for _, o := range opts {
		o(secret)
	}

	return secret
}

func WithStringData(k, v string) func(*corev1.Secret) {
	return func(secret *corev1.Secret) {
		if secret.StringData == nil {
			secret.StringData = make(map[string]string)
		}
		secret.StringData[k] = v
	}
}
