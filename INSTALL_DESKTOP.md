# Desktop Installation & Quick Start

This guide covers installing AgentCTL with desktop GUI support.

## Installation

### macOS

#### Option 1: Homebrew (Recommended)
```bash
brew tap subzone/tap
brew install --cask subzone/tap/agentctl-desktop
```

#### Option 2: Direct Download
1. Download `m-macos-universal.app.tar.gz` from [releases](https://github.com/subzone/Agentctl/releases)
2. Extract: `tar -xzf m-macos-universal.app.tar.gz`
3. Move to Applications: `mv m.app /Applications/`
4. Add CLI to PATH:
   ```bash
   sudo ln -s /Applications/m.app/Contents/MacOS/m /usr/local/bin/m
   ```

### Linux

#### Debian/Ubuntu
```bash
# Download .deb from releases
wget https://github.com/subzone/Agentctl/releases/latest/download/m-desktop_0.0.36_amd64.deb
sudo dpkg -i m-desktop_0.0.36_amd64.deb
```

#### Fedora/RHEL
```bash
# Download .rpm from releases
wget https://github.com/subzone/Agentctl/releases/latest/download/m-desktop-0.0.36.x86_64.rpm
sudo rpm -i m-desktop-0.0.36.x86_64.rpm
```

#### AppImage (Universal)
```bash
# Download AppImage from releases
wget https://github.com/subzone/Agentctl/releases/latest/download/m-desktop-0.0.36.AppImage
chmod +x m-desktop-0.0.36.AppImage

# Run it
./m-desktop-0.0.36.AppImage

# Optional: Integrate with system
./m-desktop-0.0.36.AppImage --appimage-integrate
```

### Windows

#### Option 1: MSI Installer
1. Download `m-desktop-0.0.36.msi` from [releases](https://github.com/subzone/Agentctl/releases)
2. Run the installer
3. CLI `m.exe` is automatically added to PATH

#### Option 2: Standalone Executable
1. Download `m-windows-amd64.exe` from releases
2. Rename to `m.exe`
3. Place in a folder and add to PATH manually

## First Run

### GUI Mode

**Launch the app:**
- macOS: Open from Applications or Spotlight
- Linux: Launch from applications menu
- Windows: Click desktop shortcut or Start menu

**Setup wizard will appear:**
1. Choose your LLM provider (Ollama, OpenAI, Anthropic, etc.)
2. Enter API key (or select Ollama for local/free)
3. Select default model
4. Click "Save & Start"

**Start chatting:**
1. Select an agent from the sidebar (e.g., "coder.md")
2. Type your request
3. Watch the agent work!

### CLI Mode (Same as Before)

```bash
# First run setup
m
# → Will prompt for provider/API key setup

# Start using it
m chat                              # Interactive chat
m run examples/agents/coder.md "fix bug"  # One-shot task
echo "error log" | m pipe "explain"       # Pipe mode
```

## Usage Examples

### GUI Mode
- **Browse Agents**: Sidebar shows all available agents
- **Start Chat**: Click any agent to start a session
- **Settings**: Click ⚙️ Settings to change provider/model
- **Visual Tool Calls**: See `fs_read`, `shell`, etc. in real-time

### CLI Mode (Unchanged)
```bash
# All existing commands work:
m chat examples/agents/devops.md
m run agent.md "deploy to production"
m pipe "write commit message" < git-diff.txt
m test
m doctor
m cost
```

## Switching Between GUI and CLI

Both modes share the same configuration and data:

```bash
# Launch GUI
m --gui              # Or just 'm gui'

# Use CLI (default when in terminal)
m chat               # Stays in CLI
m run agent.md       # Stays in CLI

# Share sessions
# 1. Start chat in GUI
# 2. Session saved to ~/.agentctl/sessions/
# 3. Access same session in CLI (feature coming soon)
```

## Configuration

### Shared Config File
Both GUI and CLI use: `~/.agentctl/config.yaml`

```yaml
model: ollama/qwen3-coder
temperature: 0.7
max_tokens: 4096
providers:
  anthropic:
    api_key: sk-ant-...
  openai:
    api_key: sk-...
```

### Where Things Live
```
~/.agentctl/
├── config.yaml          # Settings (shared)
├── agents/              # Your custom agents (shared)
├── sessions/            # Session history (shared)
├── cost.json            # Cost tracking (shared)
└── .api_keys            # Encrypted keys (macOS keychain)
```

## Verification

### Check Installation
```bash
# CLI works?
m --version

# GUI works?
m --gui           # Should launch window

# Check config
m doctor          # Diagnostic report
```

### Test Both Modes
```bash
# 1. Test CLI
m run examples/agents/hello.md "say hello"

# 2. Test GUI
m --gui
# → Select "hello.md" agent
# → Type "say hello"
# → Should work identically
```

## Troubleshooting

### macOS: "App can't be opened because it's from an unidentified developer"
```bash
# Right-click app → Open (first time only)
# Or disable Gatekeeper check:
xattr -d com.apple.quarantine /Applications/m.app
```

### Linux: "error while loading shared libraries: libwebkit2gtk"
```bash
# Ubuntu/Debian:
sudo apt install webkit2gtk-4.0

# Fedora:
sudo dnf install webkit2gtk4.0

# Or use AppImage (includes all dependencies)
```

### Windows: "Missing WebView2 Runtime"
```bash
# Download and install WebView2 Runtime:
# https://developer.microsoft.com/microsoft-edge/webview2/
```

### GUI won't launch but CLI works
```bash
# Check display server
echo $DISPLAY          # Linux: should show :0 or :1
echo $WAYLAND_DISPLAY  # Linux Wayland

# Try explicit launch
m --gui                # See error message
```

### CLI command runs GUI instead
```bash
# Use specific subcommand to force CLI
m chat                 # Instead of just 'm'
m run agent.md         # Instead of just 'm'

# Or disable GUI mode via env var
M_NO_GUI=1 m           # Forces CLI always
```

## Next Steps

- **Create custom agents**: `m new my-agent`
- **Explore examples**: Browse `examples/agents/` directory
- **Read full docs**: https://subzone.github.io/Agentctl/
- **Join community**: GitHub Discussions

## Getting Help

- CLI: `m --help`
- GUI: Click Help menu → Documentation
- Issues: https://github.com/subzone/Agentctl/issues
- Docs: https://subzone.github.io/Agentctl/
