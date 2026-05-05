package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// checkForUpdate queries GitHub for the latest release and prints a notice
// if a newer version is available. Checks at most once per day (cached).
// Runs silently — never blocks startup or prints errors.
func checkForUpdate(out io.Writer, currentVersion string) {
	if currentVersion == "dev" || currentVersion == "" {
		return
	}
	if !shouldCheck() {
		return
	}
	go func() {
		latest := fetchLatestVersion()
		if latest == "" || latest == currentVersion {
			return
		}
		if compareVersions(latest, currentVersion) > 0 {
			fmt.Fprintf(out, "\033[2m↑ update available: v%s → v%s (brew upgrade subzone/tap/m)\033[0m\n", currentVersion, latest)
		}
	}()
}

// shouldCheck returns true if we haven't checked in the last 24 hours.
func shouldCheck() bool {
	p := updateCheckPath()
	if p == "" {
		return false
	}
	info, err := os.Stat(p)
	if err != nil {
		return true // file doesn't exist, check now
	}
	return time.Since(info.ModTime()) > 24*time.Hour
}

// touchCheckFile updates the mtime of the check file.
func touchCheckFile() {
	p := updateCheckPath()
	if p == "" {
		return
	}
	dir := filepath.Dir(p)
	os.MkdirAll(dir, 0o700)
	os.WriteFile(p, []byte(time.Now().Format(time.RFC3339)), 0o600)
}

func updateCheckPath() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "m", ".last_update_check")
}

// fetchLatestVersion queries the GitHub API for the latest release tag.
func fetchLatestVersion() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.github.com/repos/subzone/Agentctl/releases/latest", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return ""
	}

	touchCheckFile()
	return strings.TrimPrefix(release.TagName, "v")
}

// compareVersions compares two semver-like strings (e.g. "0.0.29" vs "0.0.28").
// Returns >0 if a > b, 0 if equal, <0 if a < b.
func compareVersions(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		ai, bi := 0, 0
		fmt.Sscanf(aParts[i], "%d", &ai)
		fmt.Sscanf(bParts[i], "%d", &bi)
		if ai != bi {
			return ai - bi
		}
	}
	return len(aParts) - len(bParts)
}
