package env

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Env package handles the serializing of the etok workspace name to the
// environment file. Both terraform and etok rely on this file to determine the
// current workspace in use. Etok relies on it to also determine the current
// kubernetes namespace.
//
// The workspace name format is <namespace>_<name>. The namespace and name can
// contain alphanumerics and hyphens. The format is designed to keep it
// compatible with what terraform deems permissible:
//
//  	https://www.terraform.io/docs/cloud/workspaces/naming.html

const (
	environmentFile  = ".terraform/environment"
	componentPattern = "[a-z0-9](?:[a-z0-9-]*[a-z0-9])?"
)

var (
	// componentRegex is the regex that both namespace and workspace must match:
	// it must start and end with an alphanumeric and can contain alphanumerics
	// and the hyphen.
	componentRegex = regexp.MustCompile(fmt.Sprintf("^%s$", componentPattern))

	envRegex = regexp.MustCompile(fmt.Sprintf("^%s_%[1]s$", componentPattern))
)

// A string identifying a workspace, with helper functions to read and write the
// string to the environment file.
type Env struct {
	Namespace, Workspace string
}

func New(namespace, workspace string) (*Env, error) {
	if err := validateComponent(namespace); err != nil {
		return nil, fmt.Errorf("namespace validation failed: %w", err)
	}

	if err := validateComponent(workspace); err != nil {
		return nil, fmt.Errorf("workspace validation failed: %w", err)
	}

	return &Env{Namespace: namespace, Workspace: workspace}, nil
}

func validateComponent(component string) error {
	if !componentRegex.MatchString(component) {
		return fmt.Errorf("%s failed to match pattern %s", component, componentRegex.String())
	}
	return nil
}

func validateEnv(env string) error {
	if !envRegex.MatchString(env) {
		return fmt.Errorf("failed to match pattern %s", envRegex.String())
	}
	return nil
}

// TerraformName provides a terraform-compatible workspace name. The full
// kubernetes name is <namespace>/<name> but terraform doesn't permit '/', so we
// convert it to an '_', which is permissible according to this doc:
//
// https://www.terraform.io/docs/cloud/workspaces/naming.html
func TerraformName(namespace, workspace string) string {
	return fmt.Sprintf("%s_%s", namespace, workspace)
}

func (e *Env) String() string {
	return TerraformName(e.Namespace, e.Workspace)
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

	if err := validateEnv(string(bytes)); err != nil {
		return nil, err
	}

	namespace := strings.Split(string(bytes), "_")[0]
	workspace := strings.Split(string(bytes), "_")[1]

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

	return ioutil.WriteFile(envPath, []byte(TerraformName(namespace, workspace)), 0644)
}
