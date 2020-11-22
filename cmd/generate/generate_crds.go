package generate

import (
	"fmt"
	"io/ioutil"
	"net/http"

	cmdutil "github.com/leg100/stok/cmd/util"
	"github.com/leg100/stok/version"
	"github.com/spf13/cobra"
)

const allCrdsPath = "config/crd/bases/stok.goalspike.com_all.yaml"

var allCrdsURL = "https://raw.githubusercontent.com/leg100/stok/v" + version.Version + "/" + allCrdsPath

type GenerateCRDOptions struct {
	*cmdutil.Options

	// Path to local concatenated CRD schema
	LocalCRDPath string
	// Toggle reading CRDs from local file
	LocalCRDToggle bool
	// URL to concatenated CRD schema
	RemoteCRDURL string

	debug bool
}

func GenerateCRDCmd(opts *cmdutil.Options) (*cobra.Command, *GenerateCRDOptions) {
	o := &GenerateCRDOptions{Options: opts}
	cmd := &cobra.Command{
		Use:   "crds",
		Short: "Generate stok CRDs",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			var crds []byte

			if o.LocalCRDToggle {
				var err error
				crds, err = ioutil.ReadFile(o.LocalCRDPath)
				if err != nil {
					return err
				}
			} else {
				resp, err := http.Get(o.RemoteCRDURL)
				if err != nil {
					return err
				}
				if resp.StatusCode != 200 {
					return fmt.Errorf("failed to retrieve %s: status code: %d", o.RemoteCRDURL, resp.StatusCode)
				}

				crds, err = ioutil.ReadAll(resp.Body)
				if err != nil {
					return err
				}
			}

			fmt.Fprint(opts.Out, string(crds))

			return nil
		},
	}

	cmd.Flags().BoolVar(&o.LocalCRDToggle, "local", false, "Read CRDs from local file (default false)")
	cmd.Flags().StringVar(&o.LocalCRDPath, "path", allCrdsPath, "Path to local CRDs file")
	cmd.Flags().StringVar(&o.RemoteCRDURL, "url", allCrdsURL, "URL for CRDs file")

	return cmd, o
}
