package env

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	environmentFile = ".terraform/environment"
)

var (
	etokenvRegex = regexp.MustCompile("^[a-z0-9-]+/[a-z0-9-]+$")
)

// A string identifying a namespaced workspace, according to the format $namespace/$workspace, with
// helper functions to read and write the string to the file .terraform/environment
type EtokEnv string

func ValidateAndParse(etokenv string) (namespace string, workspace string, err error) {
	if err = Validate(etokenv); err != nil {
		return workspace, namespace, err
	}
	parts := strings.Split(etokenv, "/")
	return parts[0], parts[1], nil
}

func Validate(etokenv string) error {
	if !etokenvRegex.MatchString(etokenv) {
		return fmt.Errorf("workspace must match pattern %s", etokenvRegex.String())
	}
	return nil
}

func WithOptionalNamespace(etokenv string) EtokEnv {
	parts := strings.Split(etokenv, "/")
	if len(parts) == 1 {
		return EtokEnv(fmt.Sprintf("default/%s", parts[0]))
	}
	return EtokEnv(etokenv)
}

func NewEtokEnv(namespace, workspace string) EtokEnv {
	return EtokEnv(fmt.Sprintf("%s/%s", namespace, workspace))
}

func (env EtokEnv) Namespace() string {
	return strings.Split(string(env), "/")[0]
}

func (env EtokEnv) Workspace() string {
	return strings.Split(string(env), "/")[1]
}

func ReadEtokEnv(path string) (EtokEnv, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	bytes, err := ioutil.ReadFile(filepath.Join(absPath, environmentFile))
	if err != nil {
		return "", err
	}

	return EtokEnv(string(bytes)), nil
}

func (env EtokEnv) Write(path string) error {
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

func WriteEnvFile(path, content string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	envPath := filepath.Join(absPath, environmentFile)
	if err := os.MkdirAll(filepath.Dir(envPath), 0755); err != nil {
		return err
	}

	return ioutil.WriteFile(envPath, []byte(content), 0644)
}
