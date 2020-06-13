package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	// regex pattern for validating kubernetes resource names
	kNameRegex = "[a-z0-9]([-a-z0-9]*[a-z0-9])?"
)

// regex for validating stok workspace names (i.e. <namespace>/<workspace>)
var wsRegex = regexp.MustCompile("^" + kNameRegex + "/" + kNameRegex + "$")

// combined form: <namespace>/<workspace>
// TODO: consider struct instead
type namespacedWorkspace string

func (ws namespacedWorkspace) validate() error {
	if wsRegex.MatchString(string(ws)) {
		return nil
	} else {
		return fmt.Errorf("workspace does not match regex %s", wsRegex.String())
	}
}

func newNamespacedWorkspace(namespace, name string) namespacedWorkspace {
	return namespacedWorkspace(fmt.Sprintf("%s/%s", namespace, name))
}

func (ws *namespacedWorkspace) Set(val string) error {
	*ws = namespacedWorkspace(val)
	return ws.validate()
}

func (ws *namespacedWorkspace) String() string {
	return string(*ws)
}

func (ws *namespacedWorkspace) Type() string {
	return "namespacedWorkspace"
}

func (ws namespacedWorkspace) getNamespace() string {
	return strings.Split(string(ws), "/")[0]
}

func (ws namespacedWorkspace) getWorkspace() string {
	return strings.Split(string(ws), "/")[1]
}

func (ws namespacedWorkspace) writeEnvironmentFile(path string) error {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	envPath := filepath.Join(absolutePath, environmentFile)
	if err := os.MkdirAll(filepath.Dir(envPath), 0755); err != nil {
		return err
	}

	if err := ioutil.WriteFile(envPath, []byte(ws), 0644); err != nil {
		return err
	}

	return nil
}

func readEnvironmentFile(path string) (namespacedWorkspace, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	bytes, err := ioutil.ReadFile(filepath.Join(absolutePath, environmentFile))
	if err != nil {
		return "", err
	}

	return namespacedWorkspace(bytes), nil
}
