package controllers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/leg100/stok/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func BenchmarkBpRun(b *testing.B) {
	b.SetParallelism(1)

	bp := newBenchmarkPod(b)
	defer bp.CoreV1().Pods("default").DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: "benchmark=true"})

	for n := 0; n < b.N; n++ {
		fmt.Printf("running run(%d)\n", n)

		name := "benchmark-" + strconv.Itoa(n) + "-" + util.GenerateRandomString(5)
		pod := NewPodBuilder("default", name, "eu.gcr.io/automatize-admin/stok:v0.2.7-dirty-vwju3").AddRunnerContainer([]string{}).
			AddWorkspace().
			AddCredentials("stok").
			Build(false)
		pod.Spec.Containers[0].Command = []string{"stok", "runner"}
		pod.Spec.Containers[0].Args = []string{
			"--kind", "Shell",
			"--no-wait",
			"--",
			"uname"}
		pod.SetLabels(map[string]string{"benchmark": "true"})

		bp.run(n, pod)
		fmt.Printf("finished run(%d)\n", n)
	}

}

type benchmarkPod struct {
	kubernetes.Interface
	w watch.Interface
}

func newBenchmarkPod(b *testing.B) *benchmarkPod {
	config := config.GetConfigOrDie()
	kc, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	w, err := kc.CoreV1().Pods("default").Watch(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err)
	}
	return &benchmarkPod{Interface: kc, w: w}
}

func (bp *benchmarkPod) run(n int, pod *corev1.Pod) {
	_, err := bp.CoreV1().Pods("default").Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}

	for {
		event := <-bp.w.ResultChan()
		switch event.Type {
		case watch.Deleted:
			panic(fmt.Errorf("pod deleted"))
		}

		switch t := event.Object.(type) {
		case *corev1.Pod:
			if !strings.HasPrefix(t.GetName(), fmt.Sprintf("benchmark-%d", n)) {
				fmt.Printf("want benchmark-%d-*, got %s; ignoring\n", n, t.GetName())
				continue
			}
			switch t.Status.Phase {
			case corev1.PodSucceeded:
				return
			case corev1.PodFailed:
				panic(fmt.Errorf("pod failed"))
			default:
				continue
			}
		}
	}
	panic(fmt.Errorf("should never reach here"))
}
