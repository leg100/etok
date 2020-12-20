package generate

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/leg100/etok/cmd/flags"
	"github.com/leg100/etok/pkg/labels"
	"github.com/leg100/etok/pkg/version"
	"github.com/spf13/cobra"

	cmdutil "github.com/leg100/etok/cmd/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

const clusterRolePath = "config/rbac/role.yaml"

var clusterRoleURL = "https://raw.githubusercontent.com/leg100/etok/v" + version.Version + "/" + clusterRolePath

type generateOperatorOptions struct {
	*cmdutil.Options

	name      string
	namespace string
	image     string

	// Path to local generated cluster role definition
	localClusterRolePath string
	// Toggle reading cluster role from local file
	localClusterRoleToggle bool
	// URL to cluster role definition
	remoteClusterRoleURL string
}

func generateOperatorCmd(opts *cmdutil.Options) (*cobra.Command, *generateOperatorOptions) {
	o := &generateOperatorOptions{Options: opts}
	cmd := &cobra.Command{
		Use:   "operator",
		Short: "Generate operator's kubernetes resources",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			return o.Generate()
		},
	}

	flags.AddNamespaceFlag(cmd, &o.namespace)
	cmd.Flags().StringVar(&o.name, "name", "etok-operator", "Name for kubernetes resources")
	cmd.Flags().StringVar(&o.image, "image", version.Image, "Docker image used for both the operator and the runner")

	cmd.Flags().BoolVar(&o.localClusterRoleToggle, "local", false, "Read cluster role definition from local file (default false)")
	cmd.Flags().StringVar(&o.localClusterRolePath, "path", clusterRolePath, "Path to local cluster role definition")
	cmd.Flags().StringVar(&o.remoteClusterRoleURL, "url", clusterRoleURL, "URL for cluster role definition")

	return cmd, o
}

func (o *generateOperatorOptions) Generate() error {
	if err := o.clusterRole(); err != nil {
		return err
	}

	resources := []interface{}{
		o.deployment(),
		o.serviceAccount(),
		o.clusterRoleBinding(),
	}

	var sb strings.Builder
	for _, res := range resources {
		sb.WriteString("---\n")
		y, err := toYaml(res)
		if err != nil {
			return err
		}
		sb.WriteString(string(y))
	}
	fmt.Fprint(o.Out, sb.String())

	return nil
}

// Operator's ClusterRole. Unlike the other resources this is read from a YAML file in the repo,
// which in turn is generated with `make manifests`.
func (o *generateOperatorOptions) clusterRole() error {
	var clusterRole []byte

	if o.localClusterRoleToggle {
		var err error
		clusterRole, err = ioutil.ReadFile(o.localClusterRolePath)
		if err != nil {
			return err
		}
	} else {
		resp, err := http.Get(o.remoteClusterRoleURL)
		if err != nil {
			return err
		}
		if resp.StatusCode != 200 {
			return fmt.Errorf("failed to retrieve %s: status code: %d", o.remoteClusterRoleURL, resp.StatusCode)
		}

		clusterRole, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
	}

	fmt.Fprint(o.Out, string(clusterRole))

	return nil
}

func (o *generateOperatorOptions) clusterRoleBinding() *rbacv1.ClusterRoleBinding {
	binding := &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: o.name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      o.name,
				Namespace: o.namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     o.name,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
	// Set etok's common labels
	labels.SetCommonLabels(binding)
	// Permit filtering etok resources by component
	labels.SetLabel(binding, labels.OperatorComponent)

	return binding
}

func (o *generateOperatorOptions) serviceAccount() *corev1.ServiceAccount {
	serviceAccount := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.name,
			Namespace: o.namespace,
		},
	}

	// Set etok's common labels
	labels.SetCommonLabels(serviceAccount)
	// Permit filtering etok resources by component
	labels.SetLabel(serviceAccount, labels.OperatorComponent)

	return serviceAccount
}

func (o *generateOperatorOptions) deployment() *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.name,
			Namespace: o.namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: o.name,
					Containers: []corev1.Container{
						{
							Name:            "etok-operator",
							Image:           o.image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"etok"},
							Args:            []string{"operator"},
							Env: []corev1.EnvVar{
								{
									Name:  "WATCH_NAMESPACE",
									Value: "",
								},
								{
									Name: "POD_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name:  "OPERATOR_NAME",
									Value: "etok",
								},
								{
									Name:  "ETOK_IMAGE",
									Value: o.image,
								},
							},
							TerminationMessagePolicy: "FallbackToLogsOnError",
						},
					},
				},
			},
		},
	}

	// Set etok's common labels
	labels.SetCommonLabels(deployment)
	// Permit filtering etok resources by component
	labels.SetLabel(deployment, labels.OperatorComponent)

	// Label selector for operator pod.  It must match the pod template's labels.
	selector := labels.MakeLabels(
		labels.App,
		labels.OperatorComponent,
	)
	deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: selector}
	deployment.Spec.Template.Labels = selector

	return deployment
}

// Convert struct to YAML, leveraging JSON struct tags by first converting to JSON
func toYaml(obj interface{}) ([]byte, error) {
	j, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	y, err := yaml.JSONToYAML(j)
	if err != nil {
		return nil, err
	}

	return y, nil
}

func int32Ptr(i int32) *int32 { return &i }
