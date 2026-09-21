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
	"context"
	"io"
	"os"
	"testing"

	"github.com/Masterminds/semver/v3"
	golanghelpers "github.com/mrsimonemms/golang-helpers"
	"github.com/stretchr/testify/assert"
)

const (
	ciEnvVar     = "CI"
	ciTrue       = "true"
	validVersion = "9.0.0"
)

// captureStderr swaps os.Stderr for a pipe. As well as asserting on what is
// written, this guarantees stderr isn't a terminal, which keeps
// CheckAndMaybePrintUpdate away from the network
func captureStderr(t *testing.T) func() string {
	t.Helper()

	r, w, err := os.Pipe()
	assert.NoError(t, err)

	prev := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = prev })

	return func() string {
		assert.NoError(t, w.Close())

		out, err := io.ReadAll(r)
		assert.NoError(t, err)
		assert.NoError(t, r.Close())

		return string(out)
	}
}

func TestIsUpdateAvailable(t *testing.T) {
	tests := []struct {
		Name      string
		Current   string
		Latest    string
		Available bool
		ErrMsg    string
	}{
		{
			Name:      "Newer patch is an update",
			Current:   "1.0.0",
			Latest:    "1.0.1",
			Available: true,
		},
		{
			Name:      "Identical versions are not an update",
			Current:   "2.3.4",
			Latest:    "2.3.4",
			Available: false,
		},
		{
			Name:      "Older latest is not an update",
			Current:   "3.1.0",
			Latest:    "3.0.9",
			Available: false,
		},
		{
			Name:      "Leading v is stripped from both sides",
			Current:   "v4.0.0",
			Latest:    "v4.1.0",
			Available: true,
		},
		{
			Name:      "Leading capital V and surrounding space are stripped",
			Current:   "  V5.0.0  ",
			Latest:    " 5.0.1 ",
			Available: true,
		},
		{
			Name:      "Stable release supersedes a pre-release",
			Current:   "6.0.0-rc1",
			Latest:    "6.0.0",
			Available: true,
		},
		{
			Name:      "Pre-release does not supersede a stable release",
			Current:   "7.0.0",
			Latest:    "7.0.0-rc1",
			Available: false,
		},
		{
			Name:      "Partial versions are coerced",
			Current:   "8.1",
			Latest:    "8.2",
			Available: true,
		},
		{
			Name:    "Unparseable current version is reported",
			Current: "not-a-version",
			Latest:  validVersion,
			ErrMsg:  "invalid current version: invalid semantic version",
		},
		{
			Name:    "Unparseable latest version is reported",
			Current: validVersion,
			Latest:  "bananas",
			ErrMsg:  "invalid latest version: invalid semantic version",
		},
		{
			Name:    "Empty current version is reported",
			Current: "",
			Latest:  validVersion,
			ErrMsg:  "invalid current version: invalid semantic version",
		},
		{
			Name:    "Empty latest version is reported",
			Current: validVersion,
			Latest:  "",
			ErrMsg:  "invalid latest version: invalid semantic version",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			available, err := golanghelpers.IsUpdateAvailable(test.Current, test.Latest)

			if test.ErrMsg != "" {
				assert.EqualError(t, err, test.ErrMsg)
				assert.False(t, available)

				// The semver failure is wrapped, not flattened to a string
				assert.ErrorIs(t, err, semver.ErrInvalidSemVer)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.Available, available)
		})
	}
}

func TestIsTerminal(t *testing.T) {
	tests := []struct {
		Name string
		CI   *string
		File func(t *testing.T) []*os.File
	}{
		{
			Name: "CI short-circuits before the file is inspected",
			CI:   golanghelpers.Ptr(ciTrue),
			File: func(t *testing.T) []*os.File { return []*os.File{tempFile(t)} },
		},
		{
			Name: "An empty CI value still counts as being set",
			CI:   golanghelpers.Ptr(""),
			File: func(t *testing.T) []*os.File { return []*os.File{tempFile(t)} },
		},
		{
			Name: "A regular file is not a terminal",
			File: func(t *testing.T) []*os.File { return []*os.File{tempFile(t)} },
		},
		{
			Name: "A pipe is not a terminal",
			File: func(t *testing.T) []*os.File {
				r, w, err := os.Pipe()
				assert.NoError(t, err)
				t.Cleanup(func() {
					assert.NoError(t, r.Close())
					assert.NoError(t, w.Close())
				})
				return []*os.File{r}
			},
		},
		{
			Name: "No argument defaults to stdout",
			File: func(t *testing.T) []*os.File { return nil },
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			// Always register the restore, then set or clear as needed
			t.Setenv(ciEnvVar, "placeholder")
			if test.CI == nil {
				assert.NoError(t, os.Unsetenv(ciEnvVar))
			} else {
				t.Setenv(ciEnvVar, *test.CI)
			}

			assert.False(t, golanghelpers.IsTerminal(test.File(t)...))
		})
	}
}

func TestGetLatestStableVersion(t *testing.T) {
	tests := []struct {
		Name    string
		Ctx     func(t *testing.T) context.Context
		Owner   string
		Repo    string
		ErrMsg  string
		Contain string
	}{
		{
			Name:    "An unbuildable URL is reported before any request is made",
			Ctx:     func(t *testing.T) context.Context { return t.Context() },
			Owner:   "bad\nowner",
			Repo:    "repo",
			Contain: "net/url: invalid control character in URL",
		},
		{
			Name: "A cancelled context stops the request",
			Ctx: func(t *testing.T) context.Context {
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				return ctx
			},
			Owner:   "owner",
			Repo:    "repo",
			Contain: "context canceled",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			version, err := golanghelpers.GetLatestStableVersion(test.Ctx(t), test.Owner, test.Repo)

			assert.ErrorContains(t, err, test.Contain)
			assert.Empty(t, version)
		})
	}
}

func TestCheckAndMaybePrintUpdate(t *testing.T) {
	tests := []struct {
		Name    string
		Version string
		CI      string
	}{
		{
			Name:    "A development build is skipped",
			Version: golanghelpers.Development,
			CI:      ciTrue,
		},
		{
			Name:    "A real version is skipped when stderr is not a terminal",
			Version: "1.0.0",
			CI:      ciTrue,
		},
		{
			Name:    "A prefixed version is skipped when stderr is not a terminal",
			Version: "v1.0.0",
			CI:      ciTrue,
		},
		{
			Name:    "An empty version is skipped",
			Version: "",
			CI:      ciTrue,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			t.Setenv(ciEnvVar, test.CI)
			stderr := captureStderr(t)

			// No network, no cache IO and no output - this has to be safe to
			// call unconditionally from a CLI
			golanghelpers.CheckAndMaybePrintUpdate(t.Context(), test.Version, "owner", "repo")

			assert.Empty(t, stderr())
		})
	}
}

func tempFile(t *testing.T) *os.File {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "istty")
	assert.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, f.Close()) })

	return f
}
