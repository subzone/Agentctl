package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	goRuntime "runtime"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// UpdateDownloadResult is returned when an update package is downloaded.
type UpdateDownloadResult struct {
	Path       string `json:"path"`
	Filename   string `json:"filename"`
	ReleaseURL string `json:"releaseUrl"`
	Notes      string `json:"notes"`
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// DownloadUpdate fetches the platform release asset for the latest version.
func (a *App) DownloadUpdate() (*UpdateDownloadResult, error) {
	info := a.CheckForUpdate()
	if !info.UpdateAvailable {
		return nil, fmt.Errorf("already up to date (v%s)", info.Current)
	}

	rel, err := fetchRelease(info.Latest)
	if err != nil {
		return nil, err
	}
	assetName := pickUpdateAsset(info.Latest)
	var downloadURL string
	for _, asset := range rel.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}
	if downloadURL == "" {
		return nil, fmt.Errorf("release asset not found: %s", assetName)
	}

	destDir, err := updateDownloadDir()
	if err != nil {
		return nil, err
	}
	dest := filepath.Join(destDir, assetName)

	ctx, cancel := context.WithTimeout(a.ctx, 8*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return nil, err
	}

	total := resp.ContentLength
	var written int64
	buf := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				os.Remove(tmp)
				return nil, werr
			}
			written += int64(n)
			if total > 0 {
				pct := float64(written) / float64(total) * 100
				runtime.EventsEmit(a.ctx, "updateProgress", map[string]any{
					"phase":    "downloading",
					"percent":  pct,
					"written":  written,
					"total":    total,
					"filename": assetName,
				})
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			f.Close()
			os.Remove(tmp)
			return nil, readErr
		}
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return nil, err
	}
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return nil, err
	}

	runtime.EventsEmit(a.ctx, "updateProgress", map[string]any{
		"phase":    "done",
		"percent":  100,
		"path":     dest,
		"filename": assetName,
	})

	savePendingUpdate(info.Latest, dest)

	notes := installNotes(assetName)
	return &UpdateDownloadResult{
		Path:       dest,
		Filename:   assetName,
		ReleaseURL: info.ReleaseURL,
		Notes:      notes,
	}, nil
}

func fetchRelease(version string) (*ghRelease, error) {
	tag := version
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.github.com/repos/subzone/Agentctl/releases/tags/"+tag, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release lookup failed: HTTP %d", resp.StatusCode)
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func pickUpdateAsset(version string) string {
	switch goRuntime.GOOS {
	case "darwin":
		return fmt.Sprintf("AgentCTL_%s_macos.zip", version)
	case "windows":
		return fmt.Sprintf("AgentCTL_%s_windows_amd64.zip", version)
	default:
		return fmt.Sprintf("AgentCTL_%s_linux_amd64.tar.gz", version)
	}
}

func updateDownloadDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Downloads", "AgentCTL-updates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func installNotes(filename string) string {
	switch {
	case strings.HasSuffix(filename, ".zip") && strings.Contains(filename, "macos"):
		return "Unzip the download, then drag AgentCTL.app to /Applications. Quit this app before replacing."
	case strings.HasSuffix(filename, ".zip"):
		return "Unzip the download and run the new AgentCTL executable."
	default:
		return "Extract the archive and run ./m, or replace your existing install."
	}
}
