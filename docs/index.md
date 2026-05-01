---
title: m — MD-driven agent CLI
---

# m

`m` is an MD-driven agent CLI for code, infrastructure, and automation
work. Agents are plain Markdown files with YAML frontmatter; the CLI
runs them against your choice of model backend.

## Why m

- **One file, one agent.** Drop an `.md` file with frontmatter into a
  repo, run `m run agent.md "task"`, get streamed output.
- **6 providers.** Ollama (local), Anthropic, OpenAI, Google Gemini,
  Alibaba Cloud, or any OpenAI-compatible proxy via LiteLLM.
- **Tools + MCP + sub-agents.** Agents can call shell, read/write files,
  list directories, talk to MCP servers, and delegate to other agents.
- **Hub-and-spoke.** Orchestrator agents delegate to specialist spokes
  that return structured JSON with citations and confidence levels.
- **TUI with live stats.** Token count, cost estimate, context window %,
  system stats, theming (matrix/default/minimal + custom).

## Get started

1. **[Install](install.html)** the `.pkg` (macOS) or `.deb` (Linux).
2. Run `m` — the **[setup wizard](quickstart.html)** walks you through
   picking a model backend.
3. Type to chat. `/exit` when done.

## Documentation

- [Installation](install.html) — `.pkg`, `.deb`, build from source
- [Quickstart](quickstart.html) — first run, wizard, commands, TUI
- [Architecture](architecture.html) — how it's all wired up
- [Configuration](configuration.html) — config file, themes, env vars
- [Providers](providers.html) — Ollama, Anthropic, OpenAI, Gemini, Alibaba, LiteLLM
- [Custom agents](agents.html) — writing agents, tools, hub-and-spoke
- [Troubleshooting](troubleshooting.html) — common issues
- [Changelog](changelog.html) — release history

## Source

GitHub: [subzone/m](https://github.com/subzone/m)
