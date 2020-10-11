package app

import (
	"context"
)

// A stok app is an abstraction to permit its decoupling from the cmd pkg. Cmd is responsible for
// parsing flags, setting options and 'selecting' an app, and its tests test that functionality
// and that functionality alone; whereas the app and its tests are responsible for the lifecycle of
// the app itself (and its often complex interactions with kubernetes for which testing is
// complex), unburdening the cmd pkg of that responsibility.
type App interface {
	Run(context.Context) error
}

type NewApp func(*Options) App
