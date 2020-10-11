package cmd

import "testing"

func TestTemplater(t *testing.T) {
	_ = &templater{UsageTemplate: MainUsageTemplate()}
}
