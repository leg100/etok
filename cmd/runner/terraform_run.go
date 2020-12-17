package runner

import (
	"context"
)

func execTerraformRun(ctx context.Context, exec Executor, cmd, workspace string, args []string) error {
	// Run terraform init
	if err := exec.run(ctx, []string{"terraform", "init", "-input=false", "-no-color", "-upgrade"}); err != nil {
		return err
	}

	// Run terraform workspace select. If that fails, run terraform workspace
	// new
	if err := exec.run(ctx, []string{"terraform", "workspace", "select", "-no-color", workspace}); err != nil {
		if err := exec.run(ctx, []string{"terraform", "workspace", "new", "-no-color", workspace}); err != nil {
			return err
		}
	}

	// Run user-requested terraform cmd
	if err := exec.run(ctx, append([]string{"terraform", cmd}, args...)); err != nil {
		return err
	}

	return nil
}
