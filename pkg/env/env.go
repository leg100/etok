package env

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// Env package handles the serializing of workspace information to the
// environment file.  Etok relies on this file to determine both the current
// workspace and kubernetes namespace in use.
//
// The format is <namespace>/<workspace>.

const (
	environmentFile = ".terraform/environment"
)

// A string identifying a workspace, with helper functions to read and write the
// string to the environment file.
type Env struct {
	Namespace, Workspace string
}

func New(namespace, workspace string) (*Env, error) {
	return &Env{Namespace: namespace, Workspace: workspace}, nil
}

func (e *Env) String() string {
	return fmt.Sprintf("%s/%s", e.Namespace, e.Workspace)
}

func Read(path string) (env *Env, err error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	bytes, err := ioutil.ReadFile(filepath.Join(absPath, environmentFile))
	if err != nil {
		return nil, err
	}

	namespace := strings.Split(string(bytes), "/")[0]
	workspace := strings.Split(string(bytes), "/")[1]

	return New(namespace, workspace)
}

func (e *Env) Write(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	envPath := filepath.Join(absPath, environmentFile)
	if err := os.MkdirAll(filepath.Dir(envPath), 0755); err != nil {
		return err
	}

	return ioutil.WriteFile(envPath, []byte(e.String()), 0644)
}

func WriteEnvFile(path, namespace, workspace string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	envPath := filepath.Join(absPath, environmentFile)
	if err := os.MkdirAll(filepath.Dir(envPath), 0755); err != nil {
		return err
	}

	return ioutil.WriteFile(envPath, []byte((&Env{namespace, workspace}).String()), 0644)
}
