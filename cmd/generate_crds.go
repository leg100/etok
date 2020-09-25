package cmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/leg100/stok/version"
	"github.com/spf13/cobra"
)

const allCrdsPath = "config/crd/bases/stok.goalspike.com_all.yaml"

var allCrdsURL = "https://raw.githubusercontent.com/leg100/stok/v" + version.Version + "/" + allCrdsPath

func newCrdsCmd(out io.Writer) *cobra.Command {
	var local bool
	var path string
	var url string

	cmd := &cobra.Command{
		Use:   "crds",
		Short: "Generate stok CRDs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateCrds(local, path, url, out)
		},
	}

	cmd.Flags().BoolVar(&local, "local", false, "Read CRDs from local file (default false)")
	cmd.Flags().StringVar(&path, "path", allCrdsPath, "Path to local CRDs file")
	cmd.Flags().StringVar(&url, "url", allCrdsURL, "URL for CRDs file")

	return cmd
}

func generateCrds(local bool, path, url string, out io.Writer) error {
	var crds []byte

	if local {
		// Avoid stupid "crds: declared but not used" error
		err := error(nil)

		crds, err = ioutil.ReadFile(path)
		if err != nil {
			return err
		}
	} else {
		resp, err := http.Get(url)
		if err != nil {
			return err
		}

		crds, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
	}

	fmt.Fprint(out, string(crds))

	return nil
}
