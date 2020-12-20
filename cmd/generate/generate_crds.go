package generate

import (
	"fmt"
	"io/ioutil"
	"net/http"

	cmdutil "github.com/leg100/etok/cmd/util"
	"github.com/leg100/etok/pkg/version"
	"github.com/spf13/cobra"
)

const allCrdsPath = "config/crd/bases/etok.dev_all.yaml"

var allCrdsURL = "https://raw.githubusercontent.com/leg100/etok/v" + version.Version + "/" + allCrdsPath

type generateCRDOptions struct {
	*cmdutil.Options

	// Path to local concatenated CRD schema
	localCRDPath string
	// Toggle reading CRDs from local file
	localCRDToggle bool
	// URL to concatenated CRD schema
	remoteCRDURL string
}

func generateCRDCmd(opts *cmdutil.Options) (*cobra.Command, *generateCRDOptions) {
	o := &generateCRDOptions{Options: opts}
	cmd := &cobra.Command{
		Use:   "crds",
		Short: "Generate etok CRDs",
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			var crds []byte

			if o.localCRDToggle {
				var err error
				crds, err = ioutil.ReadFile(o.localCRDPath)
				if err != nil {
					return err
				}
			} else {
				resp, err := http.Get(o.remoteCRDURL)
				if err != nil {
					return err
				}
				if resp.StatusCode != 200 {
					return fmt.Errorf("failed to retrieve %s: status code: %d", o.remoteCRDURL, resp.StatusCode)
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

	cmd.Flags().BoolVar(&o.localCRDToggle, "local", false, "Read CRDs from local file (default false)")
	cmd.Flags().StringVar(&o.localCRDPath, "path", allCrdsPath, "Path to local CRDs file")
	cmd.Flags().StringVar(&o.remoteCRDURL, "url", allCrdsURL, "URL for CRDs file")

	return cmd, o
}
