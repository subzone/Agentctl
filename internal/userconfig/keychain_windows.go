//go:build windows

package userconfig

import (
	"os/exec"
	"strings"
)

func saveKey(provider, key string) error {
	target := keychainService + "-" + provider
	cmd := exec.Command("cmdkey", "/generic:"+target, "/user:"+keychainAccount, "/pass:"+key)
	return cmd.Run()
}

func getKey(provider string) (string, error) {
	target := keychainService + "-" + provider
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		`(Get-StoredCredential -Target '`+target+`').GetNetworkCredential().Password`)
	out, err := cmd.Output()
	if err != nil {
		return "", &errKeyNotFound{provider: provider}
	}
	result := strings.TrimSpace(string(out))
	if result == "" {
		return "", &errKeyNotFound{provider: provider}
	}
	return result, nil
}

func deleteKey(provider string) error {
	target := keychainService + "-" + provider
	cmd := exec.Command("cmdkey", "/delete:"+target)
	return cmd.Run()
}
