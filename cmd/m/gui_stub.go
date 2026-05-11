// GUI stub - compiled when building WITHOUT -tags gui (default, headless builds)
//
//go:build !gui

package main

import "fmt"

// hasGUISupport returns false when GUI code is not compiled in.
func hasGUISupport() bool {
	return false
}

// shouldLaunchGUI always returns false in headless builds.
func shouldLaunchGUI() bool {
	return false
}

// launchGUI returns an error in headless builds.
func launchGUI() error {
	return fmt.Errorf("GUI support not compiled in. Rebuild with -tags gui to enable desktop mode")
}
