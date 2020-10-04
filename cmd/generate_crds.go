package cmd

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/leg100/stok/pkg/options"
	"github.com/leg100/stok/version"
)

const allCrdsPath = "config/crd/bases/stok.goalspike.com_all.yaml"

var allCrdsURL = "https://raw.githubusercontent.com/leg100/stok/v" + version.Version + "/" + allCrdsPath

func init() {
	generateCmd.AddChild(
		NewCmd("crds").
			WithShortUsage("new <[namespace/]workspace>").
			WithShortHelp("Generate stok CRDs").
			WithFlags(func(fs *flag.FlagSet, opts *options.StokOptions) {
				fs.BoolVar(&opts.LocalCRDToggle, "local", false, "Read CRDs from local file (default false)")
				fs.StringVar(&opts.LocalCRDPath, "path", allCrdsPath, "Path to local CRDs file")
				fs.StringVar(&opts.RemoteCRDURL, "url", allCrdsURL, "URL for CRDs file")
			}).
			WithOneArg().
			WithExec(func(ctx context.Context, opts *options.StokOptions) error {
				var crds []byte

				if opts.LocalCRDToggle {
					var err error
					crds, err = ioutil.ReadFile(opts.LocalCRDPath)
					if err != nil {
						return err
					}
				} else {
					resp, err := http.Get(opts.RemoteCRDURL)
					if err != nil {
						return err
					}

					crds, err = ioutil.ReadAll(resp.Body)
					if err != nil {
						return err
					}
				}

				fmt.Fprint(opts.Out, string(crds))

				return nil
			}))
}
