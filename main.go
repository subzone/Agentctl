// Main entry point for Wails desktop builds.
// This is a simple wrapper that calls the desktop package.
package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"github.com/subzone/Agentctl/desktop"

	// Register all LLM providers (same as cmd/m/main.go)
	_ "github.com/subzone/Agentctl/internal/llm/alibaba"
	_ "github.com/subzone/Agentctl/internal/llm/anthropic"
	_ "github.com/subzone/Agentctl/internal/llm/cerebras"
	_ "github.com/subzone/Agentctl/internal/llm/gemini"
	_ "github.com/subzone/Agentctl/internal/llm/groq"
	_ "github.com/subzone/Agentctl/internal/llm/litellm"
	_ "github.com/subzone/Agentctl/internal/llm/mistral"
	_ "github.com/subzone/Agentctl/internal/llm/ollama"
	_ "github.com/subzone/Agentctl/internal/llm/openai"
)

//go:embed all:frontend/dist
var assets embed.FS

// Version is set at build time
var Version = "dev"

func main() {
	// Create application instance
	app := desktop.NewApp()
	app.SetProductVersion(Version)

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "AgentCTL",
		Width:     1200,
		Height:    800,
		MinWidth:  800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 15, G: 23, B: 42, A: 1},
		OnStartup:        app.Startup,
		Bind: []interface{}{
			app,
		},
		Debug: options.Debug{
			// Only auto-open the web inspector when explicitly debugging.
			// Shipping builds should not pop devtools on launch.
			OpenInspectorOnStartup: os.Getenv("M_DEBUG") != "",
		},
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  true,
				HideTitleBar:               false,
				FullSizeContent:            true,
				UseToolbar:                 false,
				HideToolbarSeparator:       true,
			},
			About: &mac.AboutInfo{
				Title:   "AgentCTL",
				Message: "MD-driven agent for code, infra, and automation work\n\nVersion: " + Version,
			},
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
		},
	})

	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
