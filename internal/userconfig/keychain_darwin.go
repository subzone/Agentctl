//go:build darwin

package userconfig

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// saveKey writes (or updates, via -U) a generic password entry. The service
// name encodes the provider so multiple keys can coexist. -w passes the
// secret on the command line which is briefly visible in `ps`; the
// alternative (-w with stdin) requires interactive input. For a CLI used
// on a personal machine this is the trade-off most macOS tools make.
func saveKey(provider, key string) error {
	cmd := exec.Command("security",
		"add-generic-password",
		"-a", keychainAccount,
		"-s", serviceName(provider),
		"-w", key,
		"-U",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("security add-generic-password: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// getKey reads a stored password. -w prints just the password (no metadata).
// Exit code 44 ("not found") is mapped to ErrKeyNotFound so callers can
// distinguish missing vs. broken.
func getKey(provider string) (string, error) {
	cmd := exec.Command("security",
		"find-generic-password",
		"-a", keychainAccount,
		"-s", serviceName(provider),
		"-w",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 44 {
			return "", &errKeyNotFound{provider: provider}
		}
		return "", fmt.Errorf("security find-generic-password: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}

// deleteKey removes the entry; missing entries (exit 44) are silently ok.
func deleteKey(provider string) error {
	cmd := exec.Command("security",
		"delete-generic-password",
		"-a", keychainAccount,
		"-s", serviceName(provider),
	)
	err := cmd.Run()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 44 {
			return nil
		}
		return fmt.Errorf("security delete-generic-password: %w", err)
	}
	return nil
}

func serviceName(provider string) string {
	return keychainService + "-" + provider
}
