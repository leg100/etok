package runner

import (
	"testing"

	"github.com/leg100/stok/api/stok.goalspike.com/v1alpha1"
	"github.com/leg100/stok/pkg/testobj"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestWorkspaceRunner(t *testing.T) {
	tests := []struct {
		name       string
		workspace  *v1alpha1.Workspace
		assertions func(*corev1.Pod)
	}{
		{
			name:      "Defaults",
			workspace: testobj.Workspace("default", "foo"),
			assertions: func(pod *corev1.Pod) {
				assert.Equal(t,
					&corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "workspace-foo",
							Namespace: "default",
							Labels:    map[string]string{"app": "stok", "component": "workspace", "version": "unknown", "workspace": "foo"},
						},
						Spec: corev1.PodSpec{
							RestartPolicy:      corev1.RestartPolicyNever,
							ServiceAccountName: "",
							Volumes: []corev1.Volume{
								{
									Name: "cache",
									VolumeSource: corev1.VolumeSource{
										PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
											ClaimName: "foo",
										},
									},
								},
								{
									Name: "backendconfig",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: v1alpha1.BackendConfigMapName("foo"),
											},
										},
									},
								},
							},
							InitContainers: []v1.Container{
								{
									Name:    "runner",
									Image:   "stok:latest",
									Command: []string{"stok", "runner"},
									Args: []string{
										"--",
										"sh",
										"-c",
										"terraform init -backend-config=backend.ini && \\\nterraform workspace select default-foo || terraform workspace new default-default",
									},
									WorkingDir: "/workspace",
									Env: []v1.EnvVar{
										{
											Name:  "STOK_HANDSHAKE",
											Value: "false",
										},
										{
											Name:  "STOK_HANDSHAKE_TIMEOUT",
											Value: "",
										},
									},
									VolumeMounts: []v1.VolumeMount{
										{
											Name:      "backendconfig",
											ReadOnly:  true,
											MountPath: "/workspace/backend.tf",
											SubPath:   "backend.tf",
										},
										{
											Name:      "backendconfig",
											ReadOnly:  true,
											MountPath: "/workspace/backend.ini",
											SubPath:   "backend.ini",
										},
										{
											Name:      "cache",
											ReadOnly:  false,
											MountPath: "/workspace/.terraform",
											SubPath:   ".terraform/",
										},
										{
											Name:      "cache",
											ReadOnly:  false,
											MountPath: "/workspace/terraform.tfstate.d",
											SubPath:   "terraform.tfstate.d/",
										},
									},
									TerminationMessagePolicy: "FallbackToLogsOnError",
									ImagePullPolicy:          "IfNotPresent",
									Stdin:                    true,
									StdinOnce:                false,
									TTY:                      true,
								},
							},
							Containers: []v1.Container{
								{
									Name:  "idler",
									Image: "stok:latest",
									Command: []string{
										"sh",
										"-c",
										"trap \"exit 0\" SIGTERM; while true; do sleep 1; done",
									},
									WorkingDir:               "",
									TerminationMessagePolicy: "FallbackToLogsOnError",
									ImagePullPolicy:          "IfNotPresent",
									Stdin:                    false,
									StdinOnce:                false,
									TTY:                      false,
								},
							},
						},
					}, pod)
			},
		},
		{
			name:      "Local backend",
			workspace: testobj.Workspace("foo", "bar", testobj.WithBackendType("local")),
			assertions: func(pod *corev1.Pod) {
				// Default local backend needs local state directory
				require.True(t, ContainsVolumeMount(pod.Spec.InitContainers[0], corev1.VolumeMount{
					Name:      "cache",
					MountPath: "/workspace/terraform.tfstate.d",
					SubPath:   "terraform.tfstate.d/",
				}))
			},
		},
		{
			name:      "Remote backend",
			workspace: testobj.Workspace("foo", "bar", testobj.WithBackendType("gcs")),
			assertions: func(pod *corev1.Pod) {
				// Remote backends do not have a local state directory
				require.False(t, ContainsVolumeMount(pod.Spec.InitContainers[0], corev1.VolumeMount{
					Name:      "cache",
					MountPath: "/workspace/terraform.tfstate.d",
					SubPath:   "terraform.tfstate.d/",
				}))
			},
		},
		{
			name:      "Custom terraform version",
			workspace: testobj.Workspace("foo", "bar", testobj.WithTerraformVersion("0.12.17")),
			assertions: func(pod *corev1.Pod) {
				// Specifying a custom terraform version creates a dedicated
				// volume mount for caching the bin when it is downloaded
				require.True(t, ContainsVolumeMount(pod.Spec.InitContainers[0], corev1.VolumeMount{
					Name:      "cache",
					MountPath: "/terraform-bins",
					SubPath:   "terraform-bins/",
				}))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assertions(NewWorkspaceRunner(tt.workspace).Pod("stok:latest"))
		})
	}
}

//	want := `curl -LOs
//	https://releases.hashicorp.com/terraform/0.12.17/terraform_0.12.17_linux_amd64.zip
//	&& \
//curl -LOs
//https://releases.hashicorp.com/terraform/0.12.17/terraform_0.12.17_SHA256SUMS
//&& \ sed -n "/terraform_0.12.17_linux_amd64.zip/p"
//terr//aform_0.12.17_SHA256SUMS | sha256sum -c && \ mkdir -p /terraform-bins &&
//\ unzip //terraform_0.12.17_linux_amd64.zip -d /terraform-bins && \ rm
//ter//raform_0.12.17_linux_amd64.zip && \ rm terraform_0.12.17_SHA256SUMS`
//require.Equal(t, want, TerraformDownloadScript("0.12.17")) }
