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
- **Bring your own model.** First-run wizard wires up Ollama (local),
  Anthropic, OpenAI, or any OpenAI-compatible proxy via LiteLLM.
- **Tools, MCP, sub-agents.** Agents can call shell, read files, talk
  to MCP servers, and delegate to other agents.

## Get started

1. **[Install](install.html)** the `.pkg` (macOS) or `.deb` (Linux)
   from the [latest release](https://github.com/subzone/m/releases/latest).
2. Run `m` — the **[setup wizard](quickstart.html)** walks you through
   picking a model backend.
3. Type to chat. `/exit` when done.

## Documentation

- [Installation](install.html) — `.pkg`, `.deb`, build from source
- [Quickstart](quickstart.html) — first run, the wizard, first chat
- [Configuration](configuration.html) — config file, state, env vars,
  key storage
- [Providers](providers.html) — Ollama, Anthropic, OpenAI, LiteLLM
- [Custom agents](agents.html) — writing your own `.md` agents
- [Troubleshooting](troubleshooting.html) — common issues
- [Changelog](changelog.html) — release history

## Source

GitHub: [subzone/m](https://github.com/subzone/m)
