package e2e

import (
	goctx "context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"cloud.google.com/go/storage"
	etokclient "github.com/leg100/etok/pkg/client"
	"github.com/leg100/etok/pkg/env"
)

const (
	backendBucket = "automatize-tfstate"
	backendPrefix = "e2e"
)

var (
	// absolute path to the etok binary
	buildPath string

	kubectx                = flag.String("context", "kind-kind", "Kubeconfig context to use for tests")
	disableNamespaceDelete = flag.Bool("disable-namespace-delete", false, "Disable automatic deletion of namespace at end of test")

	sclient *storage.Client
	client  *etokclient.Client
)

func TestMain(m *testing.M) {
	flag.Parse()

	var err error

	// Need absolute path because tests may change working directory
	buildPath, err = filepath.Abs("../../etok")
	if err != nil {
		errExit(err)
	}

	// Instantiate GCS client
	sclient, err = storage.NewClient(goctx.Background())
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

type test struct {
	name       string
	namespace  string
	workspace  string
	pty        bool
	privileged []string
}

func (t *test) tfWorkspace() string {
	return env.TerraformName(t.namespace, t.workspace)
}
