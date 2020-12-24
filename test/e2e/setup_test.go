package e2e

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	etokclient "github.com/leg100/etok/pkg/client"

	// Import all GCP client auth plugin
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

var (
	// absolute path to the etok binary
	buildPath string

	kubectx                = flag.String("context", "kind-kind", "Kubeconfig context to use for tests")
	disableNamespaceDelete = flag.Bool("disable-namespace-delete", false, "Disable automatic deletion of namespace at end of test")

	client *etokclient.Client
)

func TestMain(m *testing.M) {
	flag.Parse()

	var err error

	// Need absolute path because tests may change working directory
	buildPath, err = filepath.Abs("../../etok")
	if err != nil {
		errExit(err)
	}

	// Instantiate etok client
	client, err = etokclient.NewClientCreator().Create(*kubectx)
	if err != nil {
		errExit(err)
	}

	os.Exit(m.Run())
}

func errExit(err error) {
	fmt.Fprintf(os.Stderr, "failed to instantiate etok client: %v\n", err)
	os.Exit(1)
}
