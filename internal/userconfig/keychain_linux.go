//go:build linux

package userconfig

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// secretTool wraps the `secret-tool` CLI from libsecret-tools. The package
// is `libsecret-tools` on Debian/Ubuntu and `libsecret` on Fedora/Arch.
// Returns ErrSecretToolMissing if the binary is not on PATH so the wizard
// can offer to install it via the system package manager.
const secretTool = "secret-tool"

func saveKey(provider, key string) error {
	if _, err := exec.LookPath(secretTool); err != nil {
		return ErrSecretToolMissing
	}
	cmd := exec.Command(secretTool,
		"store",
		"--label", "m CLI api key for "+provider,
		"service", keychainService,
		"provider", provider,
		"account", keychainAccount,
	)
	cmd.Stdin = strings.NewReader(key)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("secret-tool store: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func getKey(provider string) (string, error) {
	if _, err := exec.LookPath(secretTool); err != nil {
		return "", ErrSecretToolMissing
	}
	cmd := exec.Command(secretTool,
		"lookup",
		"service", keychainService,
		"provider", provider,
		"account", keychainAccount,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		// secret-tool exits 1 with empty stdout when a lookup misses.
		var ee *exec.ExitError
		if errors.As(err, &ee) && stdout.Len() == 0 {
			return "", &errKeyNotFound{provider: provider}
		}
		return "", fmt.Errorf("secret-tool lookup: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	v := strings.TrimRight(stdout.String(), "\n")
	if v == "" {
		return "", &errKeyNotFound{provider: provider}
	}
	return v, nil
}

func deleteKey(provider string) error {
	if _, err := exec.LookPath(secretTool); err != nil {
		return ErrSecretToolMissing
	}
	cmd := exec.Command(secretTool,
		"clear",
		"service", keychainService,
		"provider", provider,
		"account", keychainAccount,
	)
	if err := cmd.Run(); err != nil {
		// `clear` is idempotent on missing entries.
		return nil
	}
	return nil
}
