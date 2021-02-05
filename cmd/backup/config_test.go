package backup

import (
	"context"
	"errors"
	"testing"

	"github.com/leg100/etok/pkg/testutil"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestConfig(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		assertions func(t *testutil.T, cmd *cobra.Command, cfg *Config)
		err        error
	}{
		{
			name: "no backup provider specified",
			assertions: func(t *testutil.T, cmd *cobra.Command, cfg *Config) {
				assert.Contains(t, cfg.GetEnvVars(cmd.Flags()), corev1.EnvVar{
					Name:  "ETOK_BACKUP_PROVIDER",
					Value: "",
				})
				provider, err := cfg.CreateSelectedProvider(context.Background())
				require.NoError(t, err)
				require.Nil(t, provider)
			},
		},
		{
			name: "valid config",
			args: []string{"--backup-provider=fake", "--fake-bucket=backups-bucket", "--fake-region=eu-west2"},
			assertions: func(t *testutil.T, cmd *cobra.Command, cfg *Config) {
				assert.Contains(t, cfg.GetEnvVars(cmd.Flags()), corev1.EnvVar{
					Name:  "ETOK_BACKUP_PROVIDER",
					Value: "fake",
				})
				assert.Contains(t, cfg.GetEnvVars(cmd.Flags()), corev1.EnvVar{
					Name:  "ETOK_FAKE_BUCKET",
					Value: "backups-bucket",
				})
				assert.Contains(t, cfg.GetEnvVars(cmd.Flags()), corev1.EnvVar{
					Name:  "ETOK_FAKE_REGION",
					Value: "eu-west2",
				})
				provider, err := cfg.CreateSelectedProvider(context.Background())
				require.NoError(t, err)
				require.NotNil(t, provider)
			},
		},
		{
			name: "invalid config",
			args: []string{"--backup-provider=fake", "--fake-bucket=backups-bucket"},
			err:  ErrInvalidConfig,
		},
		{
			name: "invalid provider",
			args: []string{"--backup-provider=hpcloud"},
			err:  ErrInvalidProvider,
		},
	}

	for _, tt := range tests {
		testutil.Run(t, tt.name, func(t *testutil.T) {
			cfg := NewConfig(providerMap{name: "fake", maker: newFakeFlags})
			assert.Equal(t, []string{"fake"}, cfg.providers)

			cmd := &cobra.Command{
				Use: "foo",
			}

			cfg.AddToFlagSet(cmd.Flags())

			cmd.SetArgs(tt.args)

			require.NoError(t, cmd.Execute())

			err := cfg.Validate(cmd.Flags())
			if !assert.True(t, errors.Is(err, tt.err)) {
				t.Errorf("no error in %v's chain matches %v", err, tt.err)
			}

			if tt.assertions != nil {
				tt.assertions(t, cmd, cfg)
			}
		})
	}
}
