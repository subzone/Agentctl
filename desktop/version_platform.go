//go:build !darwin

package desktop

func platformBundleVersion() string {
	return ""
}
