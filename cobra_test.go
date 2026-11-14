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
	"bytes"
	"io"
	"testing"

	golanghelpers "github.com/mrsimonemms/golang-helpers"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// hiddenFlag is the flag the source-lookup tests below ask to hide
const hiddenFlag = "access-key"

// hideWithoutPanic turns an unexpected panic into a test failure, so one
// broken case doesn't abort the remaining cases
func hideWithoutPanic(t *testing.T, cmd *cobra.Command) {
	t.Helper()
	require.NotPanics(t, func() {
		golanghelpers.HideCommandOutput(cmd, hiddenFlag)
	})
}

// newCommandTree builds a fresh root -> serve command pair. The child is
// attached so it can inherit the root's persistent flags, but nothing is
// executed, so cobra has not merged any flag sets yet
func newCommandTree() (root, child *cobra.Command) {
	root = &cobra.Command{Use: commandName}
	child = &cobra.Command{Use: "serve", Run: func(*cobra.Command, []string) {}}
	root.AddCommand(child)
	return root, child
}

// TestHideCommandOutputFlagSources checks every place cobra can resolve a flag
// from. Each case runs before command execution, so cmd.Flags() only holds
// the command's own local flags. Without the persistent and inherited fallbacks
// the lookup would fail
func TestHideCommandOutputFlagSources(t *testing.T) {
	const original = "s3cr3t-7f2d9c41"

	t.Run("Local flag", func(t *testing.T) {
		cmd := &cobra.Command{Use: commandName}
		cmd.Flags().String(hiddenFlag, original, "An API key")

		hideWithoutPanic(t, cmd)

		flag := cmd.Flags().Lookup(hiddenFlag)
		assert.Equal(t, maskedValue, flag.DefValue)
		assert.Equal(t, original, flag.Value.String())
	})

	t.Run("Persistent flag on the same command", func(t *testing.T) {
		cmd := &cobra.Command{Use: commandName}
		cmd.PersistentFlags().String(hiddenFlag, original, "An API key")

		// Guard: flag sets are not merged yet, so the local lookup misses
		assert.Nil(t, cmd.Flags().Lookup(hiddenFlag))

		hideWithoutPanic(t, cmd)

		flag := cmd.PersistentFlags().Lookup(hiddenFlag)
		assert.Equal(t, maskedValue, flag.DefValue)
		assert.Equal(t, original, flag.Value.String())
	})

	t.Run("Persistent flag inherited from a parent", func(t *testing.T) {
		root, child := newCommandTree()
		root.PersistentFlags().String(hiddenFlag, original, "An API key")

		// Guard: the child knows nothing about the flag until a merge happens
		assert.Nil(t, child.Flags().Lookup(hiddenFlag))
		assert.Nil(t, child.PersistentFlags().Lookup(hiddenFlag))

		hideWithoutPanic(t, child)

		// The inherited flag is the parent's flag, so that's where it is masked
		flag := root.PersistentFlags().Lookup(hiddenFlag)
		assert.Equal(t, maskedValue, flag.DefValue)
		assert.Equal(t, original, flag.Value.String())
	})

	t.Run("Local flag shadowing a parent's persistent flag", func(t *testing.T) {
		const parentOriginal = "parent-9b1e44"

		root, child := newCommandTree()
		root.PersistentFlags().String(hiddenFlag, parentOriginal, "A parent API key")
		child.Flags().String(hiddenFlag, original, "A child API key")

		hideWithoutPanic(t, child)

		// The effective, local flag is masked...
		local := child.Flags().Lookup(hiddenFlag)
		assert.Equal(t, maskedValue, local.DefValue)
		assert.Equal(t, original, local.Value.String())

		// ...and the parent's flag it shadows is left alone
		parent := root.PersistentFlags().Lookup(hiddenFlag)
		assert.Equal(t, parentOriginal, parent.DefValue)
		assert.Equal(t, parentOriginal, parent.Value.String())
	})
}

func TestHideCommandOutputEmptyDefaultFromAnySource(t *testing.T) {
	t.Run("Persistent flag on the same command", func(t *testing.T) {
		cmd := &cobra.Command{Use: commandName}
		cmd.PersistentFlags().String(hiddenFlag, "", "An API key")

		hideWithoutPanic(t, cmd)

		assert.Equal(t, "", cmd.PersistentFlags().Lookup(hiddenFlag).DefValue)
	})

	t.Run("Persistent flag inherited from a parent", func(t *testing.T) {
		root, child := newCommandTree()
		root.PersistentFlags().String(hiddenFlag, "", "An API key")

		hideWithoutPanic(t, child)

		assert.Equal(t, "", root.PersistentFlags().Lookup(hiddenFlag).DefValue)
	})
}

func TestHideCommandOutputUnknownFlagInTree(t *testing.T) {
	const registered = "region-key"

	tests := []struct {
		Name  string
		Setup func(root, child, sibling *cobra.Command)
		Key   string
	}{
		{
			Name: "Parent's persistent flag has a different name",
			Setup: func(root, _, _ *cobra.Command) {
				root.PersistentFlags().String(registered, "super-secret", "An API key")
			},
			Key: "mistyped",
		},
		{
			Name: "Parent's local flag is not inherited",
			Setup: func(root, _, _ *cobra.Command) {
				root.Flags().String(registered, "super-secret", "An API key")
			},
			Key: registered,
		},
		{
			Name: "Sibling's persistent flag is not inherited",
			Setup: func(_, _, sibling *cobra.Command) {
				sibling.PersistentFlags().String(registered, "super-secret", "An API key")
			},
			Key: registered,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			root, child := newCommandTree()
			sibling := &cobra.Command{Use: "other"}
			root.AddCommand(sibling)
			test.Setup(root, child, sibling)

			assert.PanicsWithValue(t, `golanghelpers: cannot hide unknown flag "`+test.Key+`"`, func() {
				golanghelpers.HideCommandOutput(child, test.Key)
			})

			// Whatever was registered keeps its real default
			for _, owner := range []*cobra.Command{root, sibling} {
				if flag := owner.Flags().Lookup(registered); flag != nil {
					assert.Equal(t, "super-secret", flag.DefValue)
				}
				if flag := owner.PersistentFlags().Lookup(registered); flag != nil {
					assert.Equal(t, "super-secret", flag.DefValue)
				}
			}
		})
	}
}

// TestHideCommandOutputRenderedHelp runs the real help output. The secret must
// not appear, the flag must still be listed, and the application must still
// receive the original value when the command runs
func TestHideCommandOutputRenderedHelp(t *testing.T) {
	const original = "s3cr3t-a84c0e19"

	tests := []struct {
		Name     string
		Register func(root, child *cobra.Command)
		Target   func(root, child *cobra.Command) *cobra.Command
		Args     []string
	}{
		{
			Name: "Local flag",
			Register: func(root, _ *cobra.Command) {
				root.Flags().String(hiddenFlag, original, "An API key")
			},
			Target: func(root, _ *cobra.Command) *cobra.Command { return root },
		},
		{
			Name: "Persistent flag on the same command",
			Register: func(root, _ *cobra.Command) {
				root.PersistentFlags().String(hiddenFlag, original, "An API key")
			},
			Target: func(root, _ *cobra.Command) *cobra.Command { return root },
		},
		{
			Name: "Inherited flag in child-command help",
			Register: func(root, _ *cobra.Command) {
				root.PersistentFlags().String(hiddenFlag, original, "An API key")
			},
			Target: func(_, child *cobra.Command) *cobra.Command { return child },
			Args:   []string{"serve"},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			t.Run("Help hides the secret", func(t *testing.T) {
				root, child := newCommandTree()
				test.Register(root, child)
				hideWithoutPanic(t, test.Target(root, child))

				var out bytes.Buffer
				root.SetOut(&out)
				root.SetErr(&out)
				root.SetArgs(append(append([]string{}, test.Args...), "--help"))
				assert.NoError(t, root.Execute())

				help := out.String()
				assert.NotContains(t, help, original)
				assert.Contains(t, help, "--"+hiddenFlag)
				assert.Contains(t, help, `(default "`+maskedValue+`")`)
			})

			t.Run("Application still reads the original value", func(t *testing.T) {
				root, child := newCommandTree()
				test.Register(root, child)
				target := test.Target(root, child)
				hideWithoutPanic(t, target)

				var got string
				var runErr error
				target.Run = func(cmd *cobra.Command, _ []string) {
					got, runErr = cmd.Flags().GetString(hiddenFlag)
				}

				root.SetOut(io.Discard)
				root.SetErr(io.Discard)
				root.SetArgs(test.Args)
				assert.NoError(t, root.Execute())
				assert.NoError(t, runErr)
				assert.Equal(t, original, got)
			})
		})
	}
}
