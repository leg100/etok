package apps

import (
	"context"

	"github.com/leg100/stok/pkg/options"
)

type App interface {
	Run(context.Context) error
}

type NewFunc func(context.Context, *options.StokOptions) (App, error)
