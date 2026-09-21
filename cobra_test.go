/*
 * Copyright 2023 Simon Emms <simon@simonemms.com>
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package golanghelpers_test

import (
	"testing"

	golanghelpers "github.com/mrsimonemms/golang-helpers"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

const (
	commandName = "app"
	maskedValue = "***"
)

func TestHideCommandOutput(t *testing.T) {
	const flagName = "api-key"

	tests := []struct {
		Name     string
		Default  string
		SetTo    string
		DefValue string
	}{
		{
			Name:     "Non-empty default is masked",
			Default:  "super-secret",
			DefValue: maskedValue,
		},
		{
			Name:     "Empty default is left alone",
			Default:  "",
			DefValue: "",
		},
		{
			Name:     "Value supplied on the command line is masked",
			Default:  "",
			SetTo:    "from-the-cli",
			DefValue: maskedValue,
		},
		{
			Name:     "Value supplied on the command line replaces the default",
			Default:  "super-secret",
			SetTo:    "from-the-cli",
			DefValue: maskedValue,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			cmd := &cobra.Command{Use: commandName}
			cmd.Flags().String(flagName, test.Default, "An API key")

			if test.SetTo != "" {
				assert.NoError(t, cmd.Flags().Set(flagName, test.SetTo))
			}

			golanghelpers.HideCommandOutput(cmd, flagName)

			flag := cmd.Flags().Lookup(flagName)
			assert.Equal(t, test.DefValue, flag.DefValue)

			// Only the help text is changed - the value stays readable
			expected := test.Default
			if test.SetTo != "" {
				expected = test.SetTo
			}
			assert.Equal(t, expected, flag.Value.String())

			value, err := cmd.Flags().GetString(flagName)
			assert.NoError(t, err)
			assert.Equal(t, expected, value)
		})
	}
}

func TestHideCommandOutputUsage(t *testing.T) {
	cmd := &cobra.Command{Use: commandName}
	cmd.Flags().String("token", "super-secret", "A token")

	assert.Contains(t, cmd.Flags().FlagUsages(), "super-secret")

	golanghelpers.HideCommandOutput(cmd, "token")

	// The secret no longer reaches the rendered help
	usage := cmd.Flags().FlagUsages()
	assert.NotContains(t, usage, "super-secret")
	assert.Contains(t, usage, maskedValue)
}

// TestHideCommandOutputUnknownFlag covers the deliberate fail-fast on an
// unregistered flag. Asking to hide a flag that doesn't exist is a programming
// error, so it panics rather than silently doing nothing
func TestHideCommandOutputUnknownFlag(t *testing.T) {
	tests := []struct {
		Name     string
		Register string
		Key      string
		Panic    string
	}{
		{
			Name:  "A command with no flags at all",
			Key:   "never-registered",
			Panic: `golanghelpers: cannot hide unknown flag "never-registered"`,
		},
		{
			Name:     "A command with other flags registered",
			Register: "api-key",
			Key:      "mistyped",
			Panic:    `golanghelpers: cannot hide unknown flag "mistyped"`,
		},
		{
			Name:     "An empty key",
			Register: "api-key",
			Key:      "",
			Panic:    `golanghelpers: cannot hide unknown flag ""`,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			cmd := &cobra.Command{Use: commandName}
			if test.Register != "" {
				cmd.Flags().String(test.Register, "super-secret", "An API key")
			}

			assert.PanicsWithValue(t, test.Panic, func() {
				golanghelpers.HideCommandOutput(cmd, test.Key)
			})

			// The registered flag is left untouched by the failed call
			if test.Register != "" {
				assert.Equal(t, "super-secret", cmd.Flags().Lookup(test.Register).DefValue)
			}
		})
	}
}
