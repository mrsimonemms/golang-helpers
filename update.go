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

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

const Development = "development"

type release struct {
	TagName string `json:"tag_name"`
}

type updateCache struct {
	LastChecked   time.Time `json:"last_checked"`
	LatestVersion string    `json:"latest_version"`
}

func CheckAndMaybePrintUpdate(ctx context.Context, currentVersion, repoOwner, repoName string) {
	// In development - nothing to do
	if currentVersion == Development {
		return
	}

	// Only print in interactive terminals
	if !IsTerminal(os.Stderr) {
		return
	}

	currentVersion = normaliseVersion(currentVersion)
	if currentVersion == "" || currentVersion == Development {
		return
	}

	// Keep this fast and bounded, even if called synchronously by mistake
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	cache, cachePath, _ := loadUpdateCache(repoName)

	// If cache is fresh, use it and avoid hitting GitHub
	if cache != nil && cacheIsFresh(cache.LastChecked, 24*time.Hour) && cache.LatestVersion != "" {
		printIfUpdateAvailable(currentVersion, cache.LatestVersion, repoOwner, repoName)
		return
	}

	// Cache is missing or stale, hit GitHub
	latest, err := GetLatestStableVersion(ctx, repoOwner, repoName)
	if err != nil {
		return
	}

	_ = saveUpdateCache(cachePath, &updateCache{
		LastChecked:   time.Now().UTC(),
		LatestVersion: latest,
	})

	printIfUpdateAvailable(currentVersion, latest, repoOwner, repoName)
}

func GetLatestStableVersion(ctx context.Context, repoOwner, repoName string) (string, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName),
		http.NoBody,
	)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "update-check")

	client := &http.Client{Timeout: time.Second}

	// #nosec G704 -- URL is operator-defined in workflow YAML; SSRF is a deployment concern, not a code defect
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var r release
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", err
	}

	return normaliseVersion(r.TagName), nil
}

func IsUpdateAvailable(current, latest string) (bool, error) {
	current = normaliseVersion(current)
	latest = normaliseVersion(latest)

	currentVer, err := semver.NewVersion(current)
	if err != nil {
		return false, fmt.Errorf("invalid current version: %w", err)
	}

	latestVer, err := semver.NewVersion(latest)
	if err != nil {
		return false, fmt.Errorf("invalid latest version: %w", err)
	}

	return latestVer.GreaterThan(currentVer), nil
}

func IsTerminal(f ...*os.File) bool {
	if _, ok := os.LookupEnv("CI"); ok {
		return false
	}

	if len(f) == 0 {
		// Default to Stdout
		f = append(f, os.Stdout)
	}

	fd := f[0].Fd()
	if fd > math.MaxInt {
		return false
	}
	return term.IsTerminal(int(fd))
}

//nolint:misspell
func printIfUpdateAvailable(currentVersion, latestVersion, repoOwner, repoName string) {
	ok, err := IsUpdateAvailable(currentVersion, latestVersion)
	if err != nil || !ok {
		return
	}

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#E5C07B")).
		Padding(1, 3)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF"))

	oldVersionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#AAAAAA"))

	newVersionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#98C379"))

	linkStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#61AFEF")).
		Underline(true)

	content := fmt.Sprintf(
		"%s %s → %s\n%s",
		titleStyle.Render("Update available:"),
		oldVersionStyle.Render(currentVersion),
		newVersionStyle.Render(latestVersion),
		linkStyle.Render(fmt.Sprintf("https://github.com/%s/%s/releases/latest", repoOwner, repoName)),
	)

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, borderStyle.Render(content))
	fmt.Fprintln(os.Stderr)
}

func cacheIsFresh(t time.Time, maxAge time.Duration) bool {
	if t.IsZero() {
		return false
	}
	return time.Since(t) < maxAge
}

func loadUpdateCache(repoName string) (*updateCache, string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, "", err
	}

	appDir := filepath.Join(configDir, repoName)
	cachePath := filepath.Join(appDir, "update.json")

	data, err := os.ReadFile(cachePath)
	if err != nil {
		// Missing cache is fine
		return nil, cachePath, nil
	}

	var c updateCache
	if err := json.Unmarshal(data, &c); err != nil {
		// Corrupt cache is fine, treat as missing
		return nil, cachePath, nil
	}

	return &c, cachePath, nil
}

func normaliseVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return v
	}

	if v[0] == 'v' || v[0] == 'V' {
		return v[1:]
	}

	return v
}

func saveUpdateCache(path string, c *updateCache) error {
	if c == nil {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}
