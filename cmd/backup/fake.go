package backup

import (
	"context"
	"fmt"

	"github.com/leg100/etok/pkg/backup"
	"github.com/spf13/pflag"
)

type fakeFlags struct {
	bucket string
	region string
}

func newFakeFlags() flags {
	return &fakeFlags{}
}

func (f *fakeFlags) addToFlagSet(fs *pflag.FlagSet) {
	fs.StringVar(&f.bucket, "fake-bucket", "", "Specify fake bucket for terraform state backups")
	fs.StringVar(&f.region, "fake-region", "", "Specify fake region for terraform state backups")
}

func (f *fakeFlags) createProvider(ctx context.Context) (backup.Provider, error) {
	return &backup.FakeProvider{}, nil
}

func (f *fakeFlags) validate() error {
	if f.bucket == "" {
		return fmt.Errorf("%w: missing fake bucket name", ErrInvalidConfig)
	}
	if f.region == "" {
		return fmt.Errorf("%w: missing fake region name", ErrInvalidConfig)
	}
	return nil
}
