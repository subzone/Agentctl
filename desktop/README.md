# AgentCTL Desktop (v0.5.x)

The desktop app is a Wails-based GUI for AgentCTL — same config, agents, tools, and sessions as the CLI, with a full control-plane UI.

## Install

### macOS
```bash
# Download AgentCTL_*_macos.zip from GitHub Releases, unzip, drag to Applications
# Or Homebrew (when available):
brew install --cask agentctl
```

### Linux
```bash
# Download AgentCTL_*_linux_amd64.tar.gz from Releases
tar -xzf AgentCTL_*_linux_amd64.tar.gz
./m
```

### Windows
Download `AgentCTL_*_windows_amd64.zip`, unzip, and run the app.

## Launch

- **macOS / Windows:** open `AgentCTL.app` or the installed shortcut
- **CLI flag:** `m --gui` or `m gui` (same binary)

## Features (v0.5)

### Chat
- Visual chat with markdown, tool-call cards, and streaming
- Slash commands: `/reset`, `/retry`, `/model`, `/export`, `/help`
- `@file.go` mentions inline file content (same as CLI)
- Session persistence — resume from Home or sidebar
- Context inspector, MoE routing history, activity timeline

### Control plane
- **Home** — setup checklist, health, quick nav
- **Extensions** — Tools, Skills, MCP servers, Agent Studio (form editors + test bench)
- **Knowledge** — live graph when `km serve` is running
- **Personality** — per-agent tone, verbosity, instructions
- **Security** (Pro) — audit log tail and policy rules

### MCP
- Define servers in `~/.config/m/mcp/*.md`
- Live dashboard in Extensions → MCP and from the chat MCP pill
- Per-server connectivity test (“Test all”)

### Updates
- Top bar shows **↑ v{latest}** when a GitHub release is newer
- In-app download to `~/Downloads/AgentCTL-updates/` with install notes
- Also available under **Settings → General**

## Config paths

All desktop and CLI data lives under `~/.config/m/`:

| Path | Purpose |
|------|---------|
| `config.yaml` | Provider, model, base URL |
| `tools/` | Custom shell tools |
| `skills/` | Skill bodies (system prompt) |
| `mcp/` | MCP server definitions |
| `agents/` | Custom agent specs |
| `personas/` | Per-agent personality overlays |
| `sessions/` | Encrypted saved chats |

## Development

### Prerequisites
```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
cd frontend && npm install
```

### Run / build
```bash
make desktop-dev      # hot reload
make desktop-build    # production binary / .app
wails generate module # refresh JS bindings after Go API changes
```

### Layout
```
desktop/           # Go bridge (sessions, MCP, updates, audit)
  app.go           # Core session + agent APIs
  chat_cmds.go     # /reset, /retry, engine steps
  mcp_dashboard.go # Live MCP status
  upgrade*.go      # Check + download updates
frontend/src/
  App.svelte       # Shell: rail, tabs, routing
  components/
    ChatView.svelte      # Chat UI (extracted)
    MCPDashboard.svelte  # MCP live status
    UpdatePanel.svelte   # In-app updater
```

## Build tags

- Default CLI build: no GUI (`go build ./cmd/m`)
- Desktop: Wails embeds the Svelte frontend into `AgentCTL.app`

## FAQ

**Do GUI and CLI share state?**  
Yes — same `~/.config/m` tree and saved sessions.

**When do tool/skill/MCP changes apply?**  
Start a **New Chat** (Apply bar reminds you when config is dirty).

**Does the UI need network?**  
Only for LLM APIs and optional update checks. The UI is bundled offline.

## License

Same as the main project — see [LICENSE](../LICENSE).
