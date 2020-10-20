package cmd

import (
	"github.com/leg100/stok/cmd/flags"
	"github.com/leg100/stok/version"
)

var (
	root = NewCmd("stok")
)

func init() {
	root.WithPersistentFlags(flags.Common).
		WithVersionFlag(version.PrintableVersion)
}
