//go:build windows

package userconfig

import (
	"errors"
	"os/exec"
	"strings"
)

var errWinCredNotFound = errors.New("credential not found in Windows Credential Manager")

// Windows uses cmdkey.exe for credential storage (no external deps).
// Credentials are stored in Windows Credential Manager.

const credPrefix = "agentctl:"

func saveKey(provider, key string) error {
	target := credPrefix + string(provider)
	cmd := exec.Command("cmdkey", "/generic:"+target, "/user:agentctl", "/pass:"+key)
	return cmd.Run()
}

func getKey(provider string) (string, error) {
	target := credPrefix + string(provider)
	// cmdkey /list doesn't expose passwords, use PowerShell CredentialManager
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		`(Get-StoredCredential -Target '`+target+`').GetNetworkCredential().Password`)
	out, err := cmd.Output()
	if err != nil {
		// Fallback: try environment variable
		return "", err
	}
	result := strings.TrimSpace(string(out))
	if result == "" {
		return "", errWinCredNotFound
	}
	return result, nil
}

func deleteKey(provider string) error {
	target := credPrefix + string(provider)
	cmd := exec.Command("cmdkey", "/delete:"+target)
	return cmd.Run()
}
