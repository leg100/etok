package env

import (
	"errors"
	"fmt"
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

var (
	errInvalidFormat = errors.New("invalid format, expecting <namespace>/<workspace>")
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
	path = filepath.Join(path, environmentFile)

	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	parts := strings.Split(string(bytes), "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("%s: %w", path, errInvalidFormat)
	}

	return New(parts[0], parts[1])
}

func (e *Env) Write(path string) error {
	path = filepath.Join(path, environmentFile)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(e.String()), 0644)
}
