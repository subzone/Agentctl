//go:build !darwin && !linux

package userconfig

import "errors"

// Stubs for unsupported platforms. The wizard refuses to run on these so
// the binary still builds; users on Windows/BSD/etc. set provider env
// vars manually.
var errUnsupported = errors.New("keychain storage is supported on macOS and Linux only — set provider env vars manually")

func saveKey(provider, key string) error    { return errUnsupported }
func getKey(provider string) (string, error) { return "", errUnsupported }
func deleteKey(provider string) error        { return errUnsupported }
