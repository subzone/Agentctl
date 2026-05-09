package userconfig

import "errors"

// ErrSecretToolMissing signals that the libsecret CLI (`secret-tool`) is
// not installed. The wizard catches this on Linux and offers to install it
// via the system package manager. Defined here (not in keychain_linux.go)
// so callers on every OS can reference it for cross-platform error
// matching with errors.Is.
var ErrSecretToolMissing = errors.New("secret-tool not found: install libsecret-tools (apt) or libsecret (dnf/pacman)")

// keychainService is the service name under which we register API keys in
// the OS keychain. We namespace per-provider so a user can have keys for
// multiple providers stored simultaneously without collision.
const keychainService = "m-cli"

// keychainAccount is the user/account part of the keychain entry. Combined
// with keychainService and a per-provider suffix, this gives us a stable,
// unique key per (provider) pair for the current OS user.
const keychainAccount = "default"

// SaveAPIKey stores an API key for a provider in the OS keychain. macOS uses
// the `security` CLI (always available); Linux uses `secret-tool` from
// libsecret-tools (must be installed — see keychain_linux.go).
func SaveAPIKey(provider Provider, key string) error {
	return saveKey(string(provider), key)
}

// GetAPIKey retrieves a previously stored API key. Returns ErrKeyNotFound
// when no entry exists for that provider.
func GetAPIKey(provider Provider) (string, error) {
	return getKey(string(provider))
}

// DeleteAPIKey removes a stored API key. Idempotent: missing entries are
// not an error.
func DeleteAPIKey(provider Provider) error {
	return deleteKey(string(provider))
}

// ErrKeyNotFound signals that no key is stored for the requested provider.
type errKeyNotFound struct{ provider string }

func (e *errKeyNotFound) Error() string {
	return "no api key stored for provider " + e.provider
}

// IsKeyNotFound reports whether err indicates a missing keychain entry.
func IsKeyNotFound(err error) bool {
	_, ok := err.(*errKeyNotFound)
	return ok
}

// SaveAPIKeyByName stores a secret by arbitrary name (for MCP tokens etc).
func SaveAPIKeyByName(name, key string) error {
	return saveKey(name, key)
}

// GetAPIKeyByName retrieves a secret by arbitrary name.
func GetAPIKeyByName(name string) (string, error) {
	return getKey(name)
}
