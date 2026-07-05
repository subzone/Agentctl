package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type pendingUpdate struct {
	Version      string `json:"version"`
	Path         string `json:"path"`
	DownloadedAt string `json:"downloadedAt"`
}

func pendingUpdatePath() string {
	base := configBaseDir()
	if base == "" {
		return ""
	}
	return filepath.Join(base, "m", ".update_pending")
}

func configBaseDir() string {
	if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
		return base
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return base
}

func loadPendingUpdate() *pendingUpdate {
	p := pendingUpdatePath()
	if p == "" {
		return nil
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var st pendingUpdate
	if err := json.Unmarshal(data, &st); err != nil {
		return nil
	}
	if st.Version == "" {
		return nil
	}
	return &st
}

func savePendingUpdate(version, path string) {
	p := pendingUpdatePath()
	if p == "" {
		return
	}
	st := pendingUpdate{
		Version:      normalizeVersion(version),
		Path:         path,
		DownloadedAt: time.Now().Format(time.RFC3339),
	}
	data, err := json.Marshal(st)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	_ = os.WriteFile(p, data, 0o600)
}

func clearPendingUpdate() {
	p := pendingUpdatePath()
	if p == "" {
		return
	}
	_ = os.Remove(p)
}

func clearPendingUpdateIfInstalled(current, latest string) {
	st := loadPendingUpdate()
	if st == nil {
		return
	}
	cur := normalizeVersion(current)
	latest = normalizeVersion(latest)
	pending := normalizeVersion(st.Version)
	if compareSemver(cur, pending) >= 0 || compareSemver(cur, latest) >= 0 {
		clearPendingUpdate()
	}
}
