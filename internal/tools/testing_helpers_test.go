package tools

import (
	"encoding/json"
	"os"
)

func writeFile(path, contents string) error {
	return os.WriteFile(path, []byte(contents), 0o644)
}

// jsonPath returns a JSON-safe path string (escapes backslashes on Windows).
func jsonPath(path string) string {
	b, _ := json.Marshal(path)
	// strip surrounding quotes
	return string(b[1 : len(b)-1])
}
