package cmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/leg100/stok/pkg/k8s"
	"github.com/spf13/cobra"
)

const (
	environmentFile = ".terraform/environment"
)

func workspaceCmd(f k8s.FactoryInterface, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Stok workspace management",
	}
	cmd.AddCommand(newNewWorkspaceCmd(f, out), newListWorkspaceCmd(f, out), newDeleteWorkspaceCmd(f), newSelectWorkspaceCmd(), newShowWorkspaceCmd())

	return cmd
}

func writeEnvironmentFile(path, namespace, name string) error {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	envPath := filepath.Join(absolutePath, environmentFile)
	if err := os.MkdirAll(filepath.Dir(envPath), 0755); err != nil {
		return err
	}

	if err := ioutil.WriteFile(envPath, []byte(fmt.Sprintf("%s/%s", namespace, name)), 0644); err != nil {
		return err
	}

	return nil
}

func readEnvironmentFile(path string) (string, string, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", "", err
	}

	bytes, err := ioutil.ReadFile(filepath.Join(absolutePath, environmentFile))
	if err != nil {
		return "", "", err
	}

	parts := strings.Split(string(bytes), "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("Unexpected content in %s: %s", path, string(bytes))
	}

	return parts[0], parts[1], nil
}
