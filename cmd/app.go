package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/apex/log"
	"github.com/leg100/stok/crdinfo"
	"github.com/leg100/stok/pkg/apis"
	v1alpha1clientset "github.com/leg100/stok/pkg/client/clientset/typed/stok/v1alpha1"
	"github.com/leg100/stok/pkg/controller/command"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubectl/pkg/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	runtimeconfig "sigs.k8s.io/controller-runtime/pkg/client/config"

	//needed for kube configs using the gcp helper (for gke)
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

func InitClient() (*client.Client, *kubernetes.Clientset, *v1alpha1clientset.StokV1alpha1Client, error) {
	// adds core GVKs
	s := scheme.Scheme
	// adds CRD GVKs
	apis.AddToScheme(s)

	// controller-runtime dynamic client
	config := runtimeconfig.GetConfigOrDie()
	client, err := client.New(config, client.Options{Scheme: s})
	if err != nil {
		return nil, nil, nil, err
	}

	// client-go client
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, nil, err
	}

	// generated clientset
	clientset, err := v1alpha1clientset.NewForConfig(config)
	if err != nil {
		return nil, nil, nil, err
	}

	return &client, kubeClient, clientset, nil
}

// These are things that are constructed after startup and therefore cannot be validated upfront
// (and maybe shouldn't be stuck in a struct!)
type app struct {
	args      []string
	resources []runtime.Object
	// embed?
	client client.Client
	// embed?
	kubeClient kubernetes.Interface
	clientset  v1alpha1clientset.StokV1alpha1Interface
	crd        crdinfo.CRDInfo
}

func newApp(crdname string, args []string) *app {
	// initialise both controller-runtime client and client-go client
	client, kubeClient, clientset, err := InitClient()
	if err != nil {
		log.WithError(err).Error("")
		os.Exit(1)
	}

	crd, ok := crdinfo.Inventory[crdname]
	if !ok {
		log.Errorf("Could not find crd '%s'", crd)
		os.Exit(1)
	}

	return &app{
		args:       args,
		client:     *client,
		kubeClient: kubeClient,
		clientset:  clientset,
		crd:        crd,
	}
}

func (app *app) run(command command.Command) error {
	defer app.cleanup()

	stopChan := make(chan os.Signal, 1)
	// Stop on SIGINTs and SIGTERMs.
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stopChan
		app.cleanup()
		os.Exit(3)
	}()

	err := app.createCommand(command)
	if err != nil {
		return err
	}
	// the pod name is the same as the command name
	podName := command.GetName()

	tarball, err := createTar()
	if err != nil {
		return err
	}

	_, err = app.createConfigMap(command, tarball)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"id": command.GetName(),
	}).Info("Initialising")

	log.Debug("Monitoring workspace queue...")
	_, err = app.waitForWorkspaceReady(command, viper.GetDuration("timeout-queue"))
	if err != nil {
		if err == wait.ErrWaitTimeout {
			return fmt.Errorf("timed out waiting for workspace to be available")
		} else {
			return err
		}
	}

	log.Debug("Waiting for pod to be running and ready...")
	pod, err := app.waitForPod(podName, podRunningAndReady, viper.GetDuration("timeout-pod"))
	if err != nil {
		if err == wait.ErrWaitTimeout {
			return fmt.Errorf("timed out waiting for pod %s to be running and ready", podName)
		} else {
			return err
		}
	}

	log.Debug("Retrieving log stream...")
	req := app.kubeClient.CoreV1().Pods(viper.GetString("namespace")).GetLogs(podName, &corev1.PodLogOptions{Follow: true})
	logs, err := req.Stream()
	if err != nil {
		return err
	}
	defer logs.Close()

	done := make(chan error)
	go func() {
		log.WithFields(log.Fields{
			"pod": podName,
		}).Info("Attaching")

		drawDivider()

		err := app.handleAttachPod(pod)
		if err != nil {
			log.Warn("Failed to attach to pod TTY; falling back to streaming logs")
			_, err = io.Copy(os.Stdout, logs)
			done <- err
		} else {
			done <- nil
		}
	}()

	// let operator know we're now streaming logs
	log.Debug("Telling the operator I'm ready to receive logs...")
	retry.RetryOnConflict(retry.DefaultRetry, func() error {
		log.Debug("Attempt to update command resource...")
		key := types.NamespacedName{Name: command.GetName(), Namespace: viper.GetString("namespace")}
		if err = app.client.Get(context.Background(), key, command); err != nil {
			done <- err
		} else {
			annotations := command.GetAnnotations()
			if annotations == nil {
				annotations = make(map[string]string)
			}
			annotations["stok.goalspike.com/client"] = "Ready"
			command.SetAnnotations(annotations)

			return app.client.Update(context.Background(), command)
		}
		return nil
	})

	return <-done
}
