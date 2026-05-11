// GUI entry point - compiled only when building with -tags gui
//
//go:build gui

package main

import (
	"os"

	"github.com/subzone/Agentctl/desktop"
	"golang.org/x/term"
)

// hasGUISupport returns true when GUI code is compiled in.
func hasGUISupport() bool {
	return true
}

// shouldLaunchGUI determines if we should start the GUI instead of CLI.
// GUI is launched when:
//  - User explicitly passes --gui or gui subcommand
//  - Binary is double-clicked (no TTY) and a display server is available
func shouldLaunchGUI() bool {
	// Check for explicit GUI flag
	for _, arg := range os.Args[1:] {
		if arg == "--gui" || arg == "gui" {
			return true
		}
	}
	
	// Don't auto-launch GUI if running a specific subcommand
	if len(os.Args) > 1 {
		cmd := os.Args[1]
		if cmd != "" && cmd != "--gui" && cmd != "gui" && cmd[0] != '-' {
			// User is running a subcommand like "m run", "m chat", etc.
			return false
		}
	}
	
	// Auto-launch GUI if no TTY and display available (double-clicked)
	if !term.IsTerminal(int(os.Stdin.Fd())) && hasDisplayServer() {
		return true
	}
	
	return false
}

// hasDisplayServer checks if a display server is available for GUI.
func hasDisplayServer() bool {
	// Check for macOS window server
	if os.Getenv("__CFBundleIdentifier") != "" {
		return true
	}
	
	// Check for Linux X11/Wayland
	if os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != "" {
		return true
	}
	
	// Windows always has display available if not in Docker/SSH
	// (Windows containers don't typically run GUI apps)
	return false
}

// launchGUI starts the Wails desktop application.
func launchGUI() error {
	return desktop.Run(Version)
}
