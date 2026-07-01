package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// UpdateInfo describes an available AgentCTL release.
type UpdateInfo struct {
	Current       string `json:"current"`
	Latest        string `json:"latest"`
	UpdateAvailable bool   `json:"updateAvailable"`
	ReleaseURL    string `json:"releaseUrl"`
	DesktopURL    string `json:"desktopUrl"`
	DownloadNotes string `json:"downloadNotes"`
}

// CheckForUpdate queries GitHub for the latest release tag.
func (a *App) CheckForUpdate() UpdateInfo {
	current := a.productVersion()
	info := UpdateInfo{
		Current:    current,
		Latest:     current,
		ReleaseURL: "https://github.com/subzone/Agentctl/releases/latest",
	}
	if current == "" || current == "dev" {
		return info
	}
	latest := fetchLatestReleaseTag()
	if latest == "" {
		return info
	}
	info.Latest = strings.TrimPrefix(latest, "v")
	if compareSemver(info.Latest, info.Current) > 0 {
		info.UpdateAvailable = true
		info.ReleaseURL = "https://github.com/subzone/Agentctl/releases/tag/" + latest
		info.DesktopURL = info.ReleaseURL
		info.DownloadNotes = "Download AgentCTL_" + info.Latest + " for your platform from the release page, or run: brew upgrade subzone/tap/m"
	}
	touchUpdateCheck()
	return info
}

func (a *App) productVersion() string {
	if a.version != "" {
		return a.version
	}
	return "0.6.0"
}

func fetchLatestReleaseTag() string {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.github.com/repos/subzone/Agentctl/releases/latest", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return ""
	}
	defer resp.Body.Close()
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return ""
	}
	return body.TagName
}

func touchUpdateCheck() {
	base, err := os.UserConfigDir()
	if err != nil {
		return
	}
	p := filepath.Join(base, "m", ".last_update_check")
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	_ = os.WriteFile(p, []byte(time.Now().Format(time.RFC3339)), 0o600)
}

// compareSemver returns 1 if a>b, -1 if a<b, 0 if equal (numeric dot parts).
func compareSemver(a, b string) int {
	a = strings.TrimPrefix(a, "v")
	b = strings.TrimPrefix(b, "v")
	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var na, nb int
		if i < len(pa) {
			fmt.Sscanf(pa[i], "%d", &na)
		}
		if i < len(pb) {
			fmt.Sscanf(pb[i], "%d", &nb)
		}
		if na > nb {
			return 1
		}
		if na < nb {
			return -1
		}
	}
	return 0
}
