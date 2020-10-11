/*
Copyright 2019 The Skaffold Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package testutil

import "os"

// SetEnvs takes a map of key values to set using os.Setenv and returns
// a function that can be called to reset the envs to their previous values,
// or unset envs where previously unset
func (t *T) SetEnvs(envs map[string]string) {
	prevEnvs := map[string]string{}
	unsetEnvs := []string{}
	for key := range envs {
		if val, ok := os.LookupEnv(key); ok {
			// Remember previously set env var
			prevEnvs[key] = val
		} else {
			// Remember previously unset env var
			unsetEnvs = append(unsetEnvs, key)
		}
	}

	t.Cleanup(func() { setEnvs(t, prevEnvs); unset(t, unsetEnvs) })

	setEnvs(t, envs)
}

func setEnvs(t *T, envs map[string]string) {
	for key, value := range envs {
		if err := os.Setenv(key, value); err != nil {
			t.Error(err)
		}
	}
}

func unset(t *T, envs []string) {
	for _, name := range envs {
		if err := os.Unsetenv(name); err != nil {
			t.Error(err)
		}
	}
}
