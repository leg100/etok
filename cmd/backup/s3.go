package backup

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/leg100/etok/pkg/backup"
	"github.com/spf13/pflag"
)

func init() {
	addProvider("s3", newS3Flags)
}

type s3Flags struct {
	bucket string
	region string
}

func newS3Flags() flags {
	return &s3Flags{}
}

func (f *s3Flags) addToFlagSet(fs *pflag.FlagSet) {
	fs.StringVar(&f.bucket, "s3-bucket", "", "Specify s3 bucket for terraform state backups")
	fs.StringVar(&f.region, "s3-region", "", "Specify s3 region for terraform state backups")
}

func (f *s3Flags) createProvider(ctx context.Context) (backup.Provider, error) {
	return backup.NewS3Provider(ctx, f.bucket, &aws.Config{Region: aws.String(f.region)})
}

func (f *s3Flags) validate() error {
	if f.bucket == "" {
		return fmt.Errorf("%w: missing s3 bucket name", ErrInvalidConfig)
	}
	if f.region == "" {
		return fmt.Errorf("%w: missing s3 region name", ErrInvalidConfig)
	}
	return nil
}
