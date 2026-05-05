package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// detectProjectContext scans the working directory for common build files
// and returns a context string describing the project's language, build
// tool, and test command. Injected into the system prompt so the agent
// knows how to work with the project.
func detectProjectContext(dir string) string {
	var parts []string

	checks := []struct {
		file  string
		lang  string
		build string
		test  string
	}{
		{"go.mod", "Go", "go build ./...", "go test ./..."},
		{"Cargo.toml", "Rust", "cargo build", "cargo test"},
		{"package.json", "JavaScript/TypeScript", "npm install", "npm test"},
		{"pyproject.toml", "Python", "pip install -e .", "pytest"},
		{"requirements.txt", "Python", "pip install -r requirements.txt", "pytest"},
		{"pom.xml", "Java (Maven)", "mvn compile", "mvn test"},
		{"build.gradle", "Java (Gradle)", "./gradlew build", "./gradlew test"},
		{"Makefile", "", "make", "make test"},
		{"CMakeLists.txt", "C/C++", "cmake --build .", "ctest"},
		{"Gemfile", "Ruby", "bundle install", "bundle exec rspec"},
	}

	for _, c := range checks {
		if _, err := os.Stat(filepath.Join(dir, c.file)); err == nil {
			if c.lang != "" {
				parts = append(parts, fmt.Sprintf("Language: %s", c.lang))
			}
			parts = append(parts, fmt.Sprintf("Build: %s", c.build))
			parts = append(parts, fmt.Sprintf("Test: %s", c.test))
			parts = append(parts, fmt.Sprintf("Detected from: %s", c.file))
			break
		}
	}

	// Check for common config files.
	for _, f := range []string{".gitignore", "Dockerfile", ".github/workflows"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			parts = append(parts, fmt.Sprintf("Has: %s", f))
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return "\nProject context:\n" + strings.Join(parts, "\n")
}
