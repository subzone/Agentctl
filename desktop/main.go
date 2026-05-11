// Desktop run function that can be called from cmd/m when -tags gui is used.

package desktop

import (
	"fmt"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

// Run starts the Wails desktop application.
// Called from cmd/m/gui_launch.go when GUI mode is selected.
// Note: For standalone desktop builds, use the root main.go instead.
func Run(version string) error {
	// Create application instance
	app := NewApp()

	// For GUI builds from cmd/m, we need to use runtime assets
	// This is a simplified version that works when launched from GUI mode
	err := wails.Run(&options.App{
		Title:     "AgentCTL",
		Width:     1200,
		Height:    800,
		MinWidth:  800,
		MinHeight: 600,
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.Startup,
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  false,
				HideTitleBar:               false,
				FullSizeContent:            false,
				UseToolbar:                 false,
				HideToolbarSeparator:       true,
			},
			About: &mac.AboutInfo{
				Title:   "AgentCTL",
				Message: "MD-driven agent for code, infra, and automation work\n\nVersion: " + version,
			},
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
		},
	})

	if err != nil {
		return fmt.Errorf("failed to run desktop app: %w", err)
	}
	
	return nil
}
