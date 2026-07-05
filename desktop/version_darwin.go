//go:build darwin

package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// platformBundleVersion reads CFBundleShortVersionString from the running .app bundle.
func platformBundleVersion() string {
	ex, err := os.Executable()
	if err != nil {
		return ""
	}
	ex, err = filepath.EvalSymlinks(ex)
	if err != nil {
		return ""
	}
	// .../AgentCTL.app/Contents/MacOS/m
	infoPlist := filepath.Join(filepath.Dir(ex), "..", "Info.plist")
	infoPlist, err = filepath.Abs(infoPlist)
	if err != nil {
		return ""
	}
	if _, err := os.Stat(infoPlist); err != nil {
		return ""
	}
	plistBase := strings.TrimSuffix(infoPlist, ".plist")
	out, err := exec.Command("defaults", "read", plistBase, "CFBundleShortVersionString").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
