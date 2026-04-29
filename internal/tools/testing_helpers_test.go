package tools

import "os"

func writeFile(path, contents string) error {
	return os.WriteFile(path, []byte(contents), 0o644)
}
