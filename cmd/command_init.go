// Code generated by go generate; DO NOT EDIT.
package cmd

import (
	"os"

	"github.com/leg100/stok/pkg/apis/stok/v1alpha1"
	"github.com/spf13/cobra"
)

var cmdInit = &cobra.Command{
	Use:   "init [global flags] -- [init args]",
	Short: "Run terraform init",
	PreRun: validatePath,
	Run: func(cmd *cobra.Command, args []string) {
		runApp(&v1alpha1.Init{}, "init", DoubleDashArgsHandler(os.Args))
	},
}

func init() {
	cmdInit.DisableFlagsInUseLine = true
	rootCmd.AddCommand(cmdInit)
}