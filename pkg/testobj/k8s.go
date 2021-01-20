package testobj

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"

	"github.com/leg100/etok/api/etok.dev/v1alpha1"
	"github.com/leg100/etok/pkg/globals"
	"github.com/leg100/etok/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Workspace(namespace, name string, opts ...func(*v1alpha1.Workspace)) *v1alpha1.Workspace {
	ws := &v1alpha1.Workspace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.WorkspaceSpec{
			Cache: v1alpha1.WorkspaceCacheSpec{
				// CRD schema default
				Size: "1Gi",
			},
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

func WithVariables(keyValues ...string) func(*v1alpha1.Workspace) {
	return func(ws *v1alpha1.Workspace) {
		for i := 0; i < len(keyValues); i += 2 {
			ws.Spec.Variables = append(ws.Spec.Variables, &v1alpha1.Variable{Key: keyValues[0], Value: keyValues[1]})
		}
	}
}

func WithBackupBucket(bucket string) func(*v1alpha1.Workspace) {
	return func(ws *v1alpha1.Workspace) {
		ws.Spec.BackupBucket = bucket
	}
}

func WithEnvironmentVariables(keyValues ...string) func(*v1alpha1.Workspace) {
	return func(ws *v1alpha1.Workspace) {
		for i := 0; i < len(keyValues); i += 2 {
			ws.Spec.Variables = append(ws.Spec.Variables, &v1alpha1.Variable{Key: keyValues[0], Value: keyValues[1], EnvironmentVariable: true})
		}
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

func WithCombinedQueue(run ...string) func(*v1alpha1.Workspace) {
	return func(ws *v1alpha1.Workspace) {
		if len(run) > 0 {
			ws.Status.Active = run[0]
		}
		ws.Status.Queue = run[1:]
	}
}

func WithStorageClass(class *string) func(*v1alpha1.Workspace) {
	return func(ws *v1alpha1.Workspace) {
		ws.Spec.Cache.StorageClass = class
	}
}

func WithTerraformVersion(version string) func(*v1alpha1.Workspace) {
	return func(ws *v1alpha1.Workspace) {
		ws.Spec.TerraformVersion = version
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
		TypeMeta: metav1.TypeMeta{
			APIVersion: "",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1alpha1.WorkspacePodName(name),
			Namespace: namespace,
			Labels:    map[string]string{"a": "b"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					// NOTE: The pod is both running and terminated in order to pass tests. The
					// alternative is to use a complicated set of reactors, which are known not to
					// play well with k8s informers:
					// https://github.com/kubernetes/kubernetes/pull/95897
					Name: "installer",
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

func WithRunnerExitCode(code int32) func(*corev1.Pod) {
	return func(pod *corev1.Pod) {
		k8s.ContainerStatusByName(pod, globals.RunnerContainerName).State.Terminated.ExitCode = code
	}
}

func WithInstallerExitCode(code int32) func(*corev1.Pod) {
	return func(pod *corev1.Pod) {
		k8s.ContainerStatusByName(pod, "installer").State.Terminated.ExitCode = code
	}
}

func Run(namespace, name string, command string, opts ...func(*v1alpha1.Run)) *v1alpha1.Run {
	run := &v1alpha1.Run{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		RunSpec: v1alpha1.RunSpec{
			Command:      command,
			ConfigMap:    name,
			ConfigMapKey: v1alpha1.RunDefaultConfigMapKey,
			AttachSpec: v1alpha1.AttachSpec{
				HandshakeTimeout: "10s",
			},
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

func WithArgs(args ...string) func(*v1alpha1.Run) {
	return func(run *v1alpha1.Run) {
		run.Args = args
	}
}

func WithConfigMapPath(path string) func(*v1alpha1.Run) {
	return func(run *v1alpha1.Run) {
		run.ConfigMapPath = path
	}
}

func Secret(namespace, name string, opts ...func(*corev1.Secret)) *corev1.Secret {
	var secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
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

func WithDataFromFile(k, path string) func(*corev1.Secret) {
	return func(secret *corev1.Secret) {
		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}
		data, _ := ioutil.ReadFile(path)
		secret.Data[k] = data
	}
}

func WithCompressedDataFromFile(k, path string) func(*corev1.Secret) {
	return func(secret *corev1.Secret) {
		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}
		f, _ := os.Open(path)
		buf := new(bytes.Buffer)
		gw := gzip.NewWriter(buf)
		io.Copy(gw, f)
		gw.Close()
		secret.Data[k] = buf.Bytes()
	}
}
