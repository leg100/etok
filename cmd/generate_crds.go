package cmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/leg100/stok/version"
	"github.com/spf13/cobra"
)

type crdsCmd struct {
	Local bool
	Path  string
	URL   string

	cmd *cobra.Command
}

const (
	allCrdsPath = "config/crd/bases/stok.goalspike.com_all.yaml"
)

var (
	allCrdsURL = "https://raw.githubusercontent.com/leg100/stok/v" + version.Version + "/" + allCrdsPath
)

func newCrdsCmd(out io.Writer) *cobra.Command {
	cc := &crdsCmd{}
	cc.cmd = &cobra.Command{
		Use:   "crds",
		Short: "Generate stok CRDs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cc.generate(out)
		},
	}

	cc.cmd.Flags().BoolVar(&cc.Local, "local", false, "Read CRDs from local file (default false)")
	cc.cmd.Flags().StringVar(&cc.Path, "path", allCrdsPath, "Path to local CRDs file")
	cc.cmd.Flags().StringVar(&cc.URL, "url", allCrdsURL, "URL for CRDs file")

	return cc.cmd
}

func (o *crdsCmd) generate(out io.Writer) error {
	var crds []byte

	if o.Local {
		// Avoid stupid "crds: declared but not used" error
		err := error(nil)

		crds, err = ioutil.ReadFile(o.Path)
		if err != nil {
			return err
		}
	} else {
		resp, err := http.Get(o.URL)
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
