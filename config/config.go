package config

import (
	"embed"
	iofs "io/fs"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

//go:embed crd/bases/*.yaml
//go:embed operator/*.yaml
var operatorResources embed.FS

// Utility func for retrieving list of files from an embedded fs.
func GetOperatorResources() ([][]byte, error) {
	var resources [][]byte

	err := iofs.WalkDir(operatorResources, ".", func(path string, d iofs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		res, err := iofs.ReadFile(operatorResources, path)
		if err != nil {
			return err
		}
		resources = append(resources, res)

		return nil
	})
	return resources, err
}
