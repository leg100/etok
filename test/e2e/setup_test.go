package e2e

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"cloud.google.com/go/storage"
	etokclient "github.com/leg100/etok/pkg/client"
	"google.golang.org/api/iterator"

	// Import all GCP client auth plugin
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

var (
	// absolute path to the etok binary
	buildPath string

	kubectx      = flag.String("context", "kind-kind", "Kubeconfig context to use for tests")
	backupBucket = flag.String("backup-bucket", "", "GCS bucket for terraform state backups")

	client *etokclient.Client

	sclient *storage.Client
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

	// Instantiate storage client
	sclient, err = storage.NewClient(context.Background())
	if err != nil {
		errExit(err)
	}

	// Ensure backup bucket is specified via a CLI flag, or failing that, as an
	// environment variable.
	if *backupBucket == "" {
		*backupBucket = os.Getenv("BACKUP_BUCKET")
		if *backupBucket == "" {
			errExit(errors.New("you need to specify a backup bucket"))
		}
	}

	// Scrub backup bucket
	bh := sclient.Bucket(*backupBucket)
	it := bh.Objects(context.Background(), &storage.Query{Prefix: ""})
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			errExit(err)
		}
		if err := bh.Object(attrs.Name).Delete(context.Background()); err != nil {
			errExit(err)
		}
	}

	os.Exit(m.Run())
}

func errExit(err error) {
	fmt.Fprintf(os.Stderr, "failed to instantiate etok client: %v\n", err)
	os.Exit(1)
}
