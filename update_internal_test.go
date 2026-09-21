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

package golanghelpers

// These helpers are unexported and, because CheckAndMaybePrintUpdate returns
// early in any non-interactive environment, unreachable from an external test
// package. They own the cache file handling, so they are tested in-package

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// stderrToFile points os.Stderr at a file in a temp directory and returns a
// reader for whatever was written to it
func stderrToFile(t *testing.T) func() string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "stderr")
	f, err := os.Create(path)
	assert.NoError(t, err)

	prev := os.Stderr
	os.Stderr = f
	t.Cleanup(func() {
		os.Stderr = prev
		assert.NoError(t, f.Close())
	})

	return func() string {
		data, err := os.ReadFile(path)
		assert.NoError(t, err)

		return string(data)
	}
}

const (
	cacheFile   = "update.json"
	testVersion = "1.2.3"
	nextVersion = "3.2.1"
)

func TestNormaliseVersion(t *testing.T) {
	tests := []struct {
		Name     string
		Version  string
		Expected string
	}{
		{
			Name:     "Empty stays empty",
			Version:  "",
			Expected: "",
		},
		{
			Name:     "Whitespace only collapses to empty",
			Version:  "   \t\n ",
			Expected: "",
		},
		{
			Name:     "Bare version is untouched",
			Version:  "1.2.3",
			Expected: testVersion,
		},
		{
			Name:     "Lower case v is stripped",
			Version:  "v1.2.3",
			Expected: testVersion,
		},
		{
			Name:     "Upper case V is stripped",
			Version:  "V1.2.3",
			Expected: testVersion,
		},
		{
			Name:     "Surrounding whitespace is trimmed before the prefix",
			Version:  "  v1.2.3\n",
			Expected: testVersion,
		},
		{
			Name:     "Only one prefix is stripped",
			Version:  "vv1.2.3",
			Expected: "v1.2.3",
		},
		{
			Name:     "Pre-release and build metadata survive",
			Version:  "v1.2.3-rc1+build.5",
			Expected: "1.2.3-rc1+build.5",
		},
		{
			// Any leading v is stripped, so non-version input is mangled. Only
			// ever called with version strings
			Name:     "A leading v is stripped from arbitrary text",
			Version:  "version",
			Expected: "ersion",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			assert.Equal(t, test.Expected, normaliseVersion(test.Version))
		})
	}
}

func TestCacheIsFresh(t *testing.T) {
	const maxAge = 24 * time.Hour

	tests := []struct {
		Name    string
		Time    time.Time
		IsFresh bool
	}{
		{
			Name:    "The zero time is never fresh",
			Time:    time.Time{},
			IsFresh: false,
		},
		{
			Name:    "Just written is fresh",
			Time:    time.Now(),
			IsFresh: true,
		},
		{
			Name:    "Inside the window is fresh",
			Time:    time.Now().Add(-23 * time.Hour),
			IsFresh: true,
		},
		{
			Name:    "Beyond the window is stale",
			Time:    time.Now().Add(-25 * time.Hour),
			IsFresh: false,
		},
		{
			// The exact < vs <= boundary can't be pinned, as time passes
			// between building the table and the call
			Name:    "Just past the window is stale",
			Time:    time.Now().Add(-maxAge - time.Millisecond),
			IsFresh: false,
		},
		{
			Name:    "Just inside the window is fresh",
			Time:    time.Now().Add(-maxAge + time.Minute),
			IsFresh: true,
		},
		{
			Name:    "A clock skewed into the future is fresh",
			Time:    time.Now().Add(time.Hour),
			IsFresh: true,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			assert.Equal(t, test.IsFresh, cacheIsFresh(test.Time, maxAge))
		})
	}
}

func TestLoadUpdateCache(t *testing.T) {
	lastChecked := time.Date(2026, time.September, 21, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		Name     string
		Contents *string
		Expected *updateCache
	}{
		{
			Name:     "A missing cache is not an error",
			Contents: nil,
			Expected: nil,
		},
		{
			Name:     "A valid cache is decoded",
			Contents: Ptr(`{"last_checked":"2026-09-21T10:30:00Z","latest_version":"1.2.3"}`),
			Expected: &updateCache{LastChecked: lastChecked, LatestVersion: testVersion},
		},
		{
			Name:     "A corrupt cache is treated as missing",
			Contents: Ptr("this is not json"),
			Expected: nil,
		},
		{
			Name:     "An empty cache file is treated as missing",
			Contents: Ptr(""),
			Expected: nil,
		},
		{
			Name:     "A cache with unknown fields still decodes",
			Contents: Ptr(`{"latest_version":"2.0.0","something_else":true}`),
			Expected: &updateCache{LatestVersion: "2.0.0"},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			configDir := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", configDir)

			expectedPath := filepath.Join(configDir, "myapp", cacheFile)

			if test.Contents != nil {
				assert.NoError(t, os.MkdirAll(filepath.Dir(expectedPath), 0o755))
				assert.NoError(t, os.WriteFile(expectedPath, []byte(*test.Contents), 0o600))
			}

			cache, path, err := loadUpdateCache("myapp")

			assert.NoError(t, err)
			// The path is always returned so a fresh cache can be written
			assert.Equal(t, expectedPath, path)
			assert.Equal(t, test.Expected, cache)
		})
	}
}

func TestLoadUpdateCacheNoConfigDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")

	cache, path, err := loadUpdateCache("myapp")

	assert.Error(t, err)
	assert.Nil(t, cache)
	assert.Empty(t, path)
}

func TestSaveUpdateCache(t *testing.T) {
	t.Run("A nil cache is a no-op", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nested", cacheFile)

		assert.NoError(t, saveUpdateCache(path, nil))
		assert.NoFileExists(t, path)
	})

	t.Run("Missing parent directories are created", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "deeply", "nested", cacheFile)

		assert.NoError(t, saveUpdateCache(path, &updateCache{LatestVersion: testVersion}))
		assert.FileExists(t, path)
	})

	t.Run("The cache is written with owner-only permissions", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), cacheFile)

		assert.NoError(t, saveUpdateCache(path, &updateCache{LatestVersion: testVersion}))

		info, err := os.Stat(path)
		assert.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	})

	t.Run("The cache round-trips through loadUpdateCache", func(t *testing.T) {
		configDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", configDir)

		written := &updateCache{
			LastChecked:   time.Date(2026, time.September, 21, 10, 30, 0, 0, time.UTC),
			LatestVersion: nextVersion,
		}

		assert.NoError(t, saveUpdateCache(filepath.Join(configDir, "myapp", cacheFile), written))

		read, _, err := loadUpdateCache("myapp")
		assert.NoError(t, err)
		assert.Equal(t, written, read)
	})

	t.Run("The payload is the documented JSON shape", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), cacheFile)

		assert.NoError(t, saveUpdateCache(path, &updateCache{
			LastChecked:   time.Date(2026, time.September, 21, 10, 30, 0, 0, time.UTC),
			LatestVersion: nextVersion,
		}))

		data, err := os.ReadFile(path)
		assert.NoError(t, err)

		decoded := map[string]any{}
		assert.NoError(t, json.Unmarshal(data, &decoded))
		assert.Equal(t, map[string]any{
			"last_checked":   "2026-09-21T10:30:00Z",
			"latest_version": nextVersion,
		}, decoded)
	})

	t.Run("A directory that cannot be created is reported", func(t *testing.T) {
		// A regular file can't also be a parent directory
		blocker := filepath.Join(t.TempDir(), "blocker")
		assert.NoError(t, os.WriteFile(blocker, []byte("x"), 0o600))

		err := saveUpdateCache(filepath.Join(blocker, cacheFile), &updateCache{LatestVersion: testVersion})

		assert.ErrorContains(t, err, "not a directory")
	})
}

func TestPrintIfUpdateAvailable(t *testing.T) {
	tests := []struct {
		Name    string
		Current string
		Latest  string
		Printed bool
	}{
		{
			Name:    "An available update is announced",
			Current: "1.0.0",
			Latest:  "1.1.0",
			Printed: true,
		},
		{
			Name:    "Prefixes are normalised before comparing",
			Current: "v2.0.0",
			Latest:  "v2.0.1",
			Printed: true,
		},
		{
			Name:    "Nothing is printed when already up to date",
			Current: "3.0.0",
			Latest:  "3.0.0",
			Printed: false,
		},
		{
			Name:    "Nothing is printed when ahead of the latest release",
			Current: "4.1.0",
			Latest:  "4.0.0",
			Printed: false,
		},
		{
			Name:    "An unparseable version is swallowed rather than printed",
			Current: "not-a-version",
			Latest:  "5.0.0",
			Printed: false,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			stderr := stderrToFile(t)

			printIfUpdateAvailable(test.Current, test.Latest, "owner", "repo")

			out := stderr()
			if !test.Printed {
				assert.Empty(t, out)
				return
			}

			assert.Contains(t, out, "Update available:")
			assert.Contains(t, out, normaliseVersion(test.Current))
			assert.Contains(t, out, normaliseVersion(test.Latest))
			assert.Contains(t, out, "https://github.com/owner/repo/releases/latest")
		})
	}
}
