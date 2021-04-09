package config

import (
	"embed"
	"io/fs"
)

//go:embed crd/bases/*.yaml
//go:embed operator/*.yaml
var operatorResources embed.FS

// Retrieve operator k8s resources from embedded fs
func GetOperatorResources() ([][]byte, error) {
	var resources [][]byte

	err := fs.WalkDir(operatorResources, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		res, err := fs.ReadFile(operatorResources, path)
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