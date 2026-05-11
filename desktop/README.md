# Desktop Version

The desktop version of AgentCTL provides a modern GUI interface while maintaining 100% compatibility with the CLI version.

## Installation

### macOS
```bash
# Download and install the .dmg or .pkg from releases
# Or via Homebrew (coming soon):
brew install --cask agentctl
```

### Linux
```bash
# Download .deb, .rpm, or .AppImage from releases
# Debian/Ubuntu:
sudo dpkg -i m-desktop_0.0.36_amd64.deb

# Fedora/RHEL:
sudo rpm -i m-desktop-0.0.36.x86_64.rpm

# AppImage (universal):
chmod +x m-desktop-0.0.36.AppImage
./m-desktop-0.0.36.AppImage
```

### Windows
```powershell
# Download and run the .msi installer from releases
# Or use the standalone .exe
```

## Usage

The desktop version includes both GUI and CLI in a single binary:

### GUI Mode
```bash
# Launch GUI explicitly:
m --gui
m gui

# On macOS/Windows: Double-click the app icon
# On Linux: Launch from applications menu or desktop
```

### CLI Mode (Same as Always)
```bash
# All existing CLI commands work unchanged:
m chat
m run agent.md "task"
m pipe "explain this"
cat error.log | m pipe
```

## Features

### GUI Features
- **Visual Chat Interface**: Rich markdown rendering with syntax highlighting
- **Agent Manager**: Browse and select from built-in and custom agents
- **Tool Execution Visualization**: See tool calls in real-time
- **Settings Panel**: Configure providers and API keys visually
- **File References**: Drag & drop agent files
- **Session History**: Persistent chat sessions

### Shared Features (GUI & CLI)
- Same configuration files (`~/.agentctl/config.yaml`)
- Same agent definitions (Markdown files)
- Same session history
- Same cost tracking
- Same MCP server support

## Development

### Prerequisites
```bash
# Install Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Install frontend dependencies
cd frontend && npm install
```

### Build & Run
```bash
# Development mode (hot reload)
make desktop-dev

# Build for current platform
make desktop-build

# Build for all platforms
make desktop-build-all
```

### Project Structure
```
desktop/          # Desktop app bridge
  ├── app.go      # App logic (session management, config)
  └── main.go     # Wails entry point

frontend/         # Svelte UI
  ├── src/
  │   ├── App.svelte           # Main app
  │   └── components/
  │       ├── Chat.svelte      # Chat interface
  │       ├── Sidebar.svelte   # Agent browser
  │       └── Settings.svelte  # Settings panel
  └── package.json

cmd/m/
  ├── main.go         # Entry point with dual-mode detection
  ├── gui_launch.go   # GUI launcher (build tag: gui)
  └── gui_stub.go     # GUI stub (build tag: !gui)
```

## Build Tags

The project uses Go build tags to conditionally compile GUI support:

- **Default (headless)**: `go build` → CLI only (~8 MB)
- **With GUI**: `go build -tags gui` → CLI + GUI (~18 MB)
- **Docker**: Always headless (uses `-tags headless` explicitly)

## Platform-Specific Notes

### macOS
- Uses native WebKit (WKWebView)
- Supports Touch Bar (future)
- Menu bar integration
- App signing for Gatekeeper

### Linux
- Uses WebKitGTK
- Desktop entry for application menus
- AppImage for universal compatibility
- Wayland and X11 support

### Windows
- Uses WebView2 (Edge)
- MSI installer with auto-update support
- System tray integration (future)

## FAQ

**Q: Can I use both GUI and CLI at the same time?**  
A: Yes! They share the same configuration and session history.

**Q: Does GUI require internet for the UI?**  
A: No, the UI is bundled. Only LLM API calls require internet.

**Q: Can I run CLI commands from the GUI?**  
A: Not yet, but this is planned. For now, use your terminal.

**Q: Does the Docker version include GUI?**  
A: No, Docker builds are headless-only for minimal size.

**Q: Can I build desktop version without Wails?**  
A: No, Wails is required for desktop builds. CLI builds don't need it.

## Contributing

Desktop-specific contributions welcome! See [CONTRIBUTING.md](../CONTRIBUTING.md).

## License

Same as main project - see [LICENSE](../LICENSE).
