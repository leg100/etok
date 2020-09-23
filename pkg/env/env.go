package env

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

const (
	environmentFile = ".terraform/environment"
)

// A string identifying a namespaced workspace, according to the format $namespace/$workspace, with
// helper functions to read and write the string to the file .terraform/environment
type StokEnv string

func NewStokEnv(namespace, workspace string) StokEnv {
	return StokEnv(fmt.Sprintf("%s/%s", namespace, workspace))
}

func (env StokEnv) Namespace() string {
	return strings.Split(string(env), "/")[0]
}

func (env StokEnv) Workspace() string {
	return strings.Split(string(env), "/")[1]
}

func ReadStokEnv(path string) (StokEnv, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	bytes, err := ioutil.ReadFile(filepath.Join(absPath, environmentFile))
	if err != nil {
		return "", err
	}

	return StokEnv(string(bytes)), nil
}

func (env StokEnv) Write(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	envPath := filepath.Join(absPath, environmentFile)
	if err := os.MkdirAll(filepath.Dir(envPath), 0755); err != nil {
		return err
	}

	return ioutil.WriteFile(envPath, []byte(string(env)), 0644)
}
