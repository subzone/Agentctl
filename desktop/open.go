package desktop

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// OpenPath reveals a path in the system file manager.
func (a *App) OpenPath(path string) error {
	if path == "" {
		return fmt.Errorf("path required")
	}
	if _, err := os.Stat(path); err != nil {
		// Open parent if exact path missing (e.g. sessions dir before first save).
		if os.IsNotExist(err) {
			if err := os.MkdirAll(path, 0o700); err != nil {
				return fmt.Errorf("path not found: %s", path)
			}
		} else {
			return err
		}
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("explorer", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}
