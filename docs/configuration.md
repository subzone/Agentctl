---
title: Configuration
---

[← Docs home](./)

# Configuration

`m` keeps its state in two YAML files plus the OS keychain. Nothing
else is touched: no shell rc files, no global env, no daemons of its
own.

## Files

| File | What it stores | Mode |
|---|---|---|
| `config.yaml` | provider choice, model id, optional base URL, default agent | 0600 |
| `state.yaml` | last version whose release notes have been shown | 0644 |
| `theme.yaml` | TUI color theme (optional, matrix theme used if absent) | 0644 |

### Locations

| OS | Path |
|---|---|
| macOS | `~/Library/Application Support/m/` |
| Linux | `~/.config/m/` |

These are whatever Go's [`os.UserConfigDir`][userconfigdir] reports —
the OS-conventional config directory.

[userconfigdir]: https://pkg.go.dev/os#UserConfigDir

### Example `config.yaml`

```yaml
provider: ollama
model: qwen3-coder
```

For LiteLLM:

```yaml
provider: litellm
model: gpt-4o
base_url: https://litellm.example.com
```

For Anthropic / OpenAI / Ollama, `base_url` is omitted (defaults are
fine).

### Example `state.yaml`

```yaml
last_seen_version: 0.0.12
```

### Example `theme.yaml`

```yaml
name: my-theme
banner: "#ff6600"
user: "#00ccff"
assistant: ""
tool: "#888888"
error: "#ff0000"
dim: "#555555"
prompt: "#00ccff"
```

Built-in themes: `matrix` (green monochrome, default), `default`
(blue/yellow), `minimal` (no color). Switch with `/theme <name>` in
chat or edit the file directly.

This is automatic; `m` writes it after showing release notes for a
version. Delete it to force the next launch to re-show notes.

## API keys

Hosted providers (Anthropic, OpenAI, LiteLLM) store keys in the **OS
keychain**, never in `config.yaml`. Implementation:

- **macOS** — `security` CLI (built in). Keys live in your login
  keychain under service `m-cli-<provider>` and account `default`.
  Visible in **Keychain Access.app** if you want to inspect them.
- **Linux** — `secret-tool` from `libsecret-tools`. The wizard
  auto-installs this if missing (`apt`, `dnf`, `pacman`, `apk`).

Reading keys back:

```bash
# macOS
security find-generic-password -a default -s m-cli-anthropic -w

# Linux
secret-tool lookup service m-cli provider anthropic account default
```

To delete a stored key:

```bash
# macOS
security delete-generic-password -a default -s m-cli-anthropic

# Linux
secret-tool clear service m-cli provider anthropic account default
```

## Environment variables

`m` recognizes a handful of env vars at startup. They override or
supplement what's in `config.yaml`.

| Variable | Purpose |
|---|---|
| `M_MODEL` | One-shot override; format `provider/model`. |
| `ANTHROPIC_API_KEY` | Anthropic key (falls back to keychain). |
| `OPENAI_API_KEY` | OpenAI key. |
| `OPENAI_BASE_URL` | Custom OpenAI-compatible endpoint. |
| `GEMINI_API_KEY` | Google Gemini key. |
| `GEMINI_BASE_URL` | Custom Gemini endpoint (rare). |
| `DASHSCOPE_API_KEY` | Alibaba DashScope key. |
| `DASHSCOPE_BASE_URL` | Custom DashScope endpoint (rare). |
| `OLLAMA_HOST` | Non-default Ollama host. |
| `LITELLM_API_KEY` | LiteLLM proxy key. |
| `LITELLM_BASE_URL` | LiteLLM proxy URL. |

Setting an env var means: `m` skips the keychain lookup for that
provider for the current process only.

## Re-configuring

To switch backends or change models, run:

```bash
m init
```

That overwrites `config.yaml` and replaces the keychain entry for the
chosen provider. Keys for *other* providers stay in the keychain
untouched.

## Manually editing `config.yaml`

It's plain YAML; you can edit it directly. The schema is:

```yaml
provider: ollama | anthropic | openai | litellm   # required
model: <provider-specific id>                     # required
base_url: <url>                                   # optional, mainly for litellm
default_agent: /path/to/agent.md                  # optional, custom default for bare `m`
```

If `default_agent` is set, bare `m` loads that file instead of the
embedded default. The agent's `model:` field is used as-is (not
overridden by the config's provider/model). Companion docs (subagents,
MCP servers) are resolved from the agent file's project root.

If you change the provider field manually but no key is stored for
that provider, the next `m` will fail with a clear "key not found"
error — run `m init` to add one.

## Next steps

- **[Providers](providers.html)** — per-provider details, model ids.
- **[Troubleshooting](troubleshooting.html)** — when keys aren't
  found, configs are stale, etc.
