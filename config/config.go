package config

import (
	"embed"
	"io/fs"
)

//go:embed crd/bases/*.yaml
//go:embed operator/*.yaml
var operatorResources embed.FS

//go:embed webhook/*.yaml
var webhookResources embed.FS

// Retrieve operator k8s resources from embedded fs
func GetOperatorResources() ([][]byte, error) {
	return getFiles(operatorResources)
}

// Retrieve webhook k8s resources from embedded fs
func GetWebhookResources() ([][]byte, error) {
	return getFiles(webhookResources)
}

// Utility func for retrieving files from embedded fs
func getFiles(embeddedfs embed.FS) ([][]byte, error) {
	var resources [][]byte

	err := fs.WalkDir(embeddedfs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		res, err := fs.ReadFile(embeddedfs, path)
		if err != nil {
			return err
		}
		resources = append(resources, res)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return resources, err
}
