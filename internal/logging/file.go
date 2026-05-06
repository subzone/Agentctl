package logging

import (
	"io"
	"os"
	"path/filepath"
	"sort"
)

const maxLogFiles = 5

// LogDir returns the log directory path.
func LogDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "m", "logs")
}

// InitWithFile sets up logging to both stderr and a log file.
// Returns a closer function. If file creation fails, logs to stderr only.
func InitWithFile(debug bool) func() {
	dir := LogDir()
	if dir == "" {
		Init(os.Stderr, debug)
		return func() {}
	}
	os.MkdirAll(dir, 0o700)
	rotateLogFiles(dir)

	path := filepath.Join(dir, "m.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		Init(os.Stderr, debug)
		return func() {}
	}

	w := io.MultiWriter(os.Stderr, f)
	Init(w, debug)
	return func() { f.Close() }
}

func rotateLogFiles(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var logs []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".log" {
			logs = append(logs, filepath.Join(dir, e.Name()))
		}
	}
	if len(logs) < maxLogFiles {
		return
	}
	sort.Strings(logs)
	for _, old := range logs[:len(logs)-maxLogFiles+1] {
		os.Remove(old)
	}
}
