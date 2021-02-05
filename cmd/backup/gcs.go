package backup

import (
	"context"
	"fmt"

	"github.com/leg100/etok/pkg/backup"
	"github.com/spf13/pflag"
)

func init() {
	addProvider("gcs", newGcsFlags)
}

type gcsFlags struct {
	bucket string
}

func newGcsFlags() flags {
	return &gcsFlags{}
}

func (f *gcsFlags) addToFlagSet(fs *pflag.FlagSet) {
	fs.StringVar(&f.bucket, "gcs-bucket", "", "Specify gcs bucket for terraform state backups")
}

func (f *gcsFlags) createProvider(ctx context.Context) (backup.Provider, error) {
	return backup.NewGCSProvider(ctx, f.bucket, nil)
}

func (f *gcsFlags) validate() error {
	if f.bucket == "" {
		return fmt.Errorf("%w: missing gcs bucket name", ErrInvalidConfig)
	}
	return nil
}
