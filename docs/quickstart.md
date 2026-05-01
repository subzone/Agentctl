---
title: Quickstart
---

[← Docs home](./)

# Quickstart

Once `m` is [installed](install.html), the first time you run it
launches the setup wizard. After that, bare `m` opens an interactive
chat.

## First launch

```bash
m
```

You'll see the banner and a four-option menu:

```
███╗   ███╗
████╗ ████║
██╔████╔██║
██║╚██╔╝██║
██║ ╚═╝ ██║
╚═╝     ╚═╝

Welcome to m. Choose your model backend:
  1) Ollama + Qwen3-Coder      — local, free, ~5–20 GB download
  2) Anthropic (Claude)        — best quality, paid API
  3) OpenAI (GPT)              — paid API
  4) LiteLLM proxy             — self-hosted / custom endpoint

Choice [1-4]:
```

Each option's flow is detailed in [Providers](providers.html). A quick
summary:

| Option | What it asks for | What gets installed |
|---|---|---|
| Ollama + Qwen | Tag (default `qwen3-coder`) | Ollama (via `brew` or `curl`), then the chosen Qwen tag |
| Anthropic | API key (hidden input) | Nothing; key saved to OS keychain |
| OpenAI | API key (hidden input) | Nothing; key saved to OS keychain |
| LiteLLM | base URL + key | Nothing; key saved to OS keychain |

After the wizard finishes, `m` writes a small config file (location
under [Configuration](configuration.html)) and drops you straight into
chat.

## Chatting

In an interactive terminal, `m` opens a full-screen TUI:

```
┌──────────────────────────────────────────────┐
│ ███╗   ███╗  ┌──────────┐  ┌──────────┐     │
│ ████╗ ████║  │ Model  … │  │ CPU  12% │     │
│ ██╔████╔██║  │ In     0 │  │ RAM  45% │     │
│ ██║╚██╔╝██║  │ Out    0 │  │ GPU  n/a │     │
│ ██║ ╚═╝ ██║  │ Total  0 │  │ Disk 78% │     │
│ ╚═╝     ╚═╝  │ Cost  $0 │  └──────────┘     │
│               └──────────┘                   │
│  /exit  /reset  /help                        │
├──────────────────────────────────────────────┤
│ chat scrolls here                            │
│ » hello                                      │
│ Hello! How can I help today?                 │
├──────────────────────────────────────────────┤
│ » _                                          │
└──────────────────────────────────────────────┘
```

The header shows the M banner, a token/cost box (model name, input/output
tokens, estimated cost), and system stats. The commands bar (`/exit`,
`/reset`, `/help`) is always visible. The chat viewport scrolls; input
is pinned at the bottom.

If you pipe input or run `m` in a script (any of stdin/stdout/stderr
not a TTY), it falls back to the line-oriented REPL — same prompt
(`»`), same slash commands, plain stream of text.

Slash commands work in both modes:

| Command | What it does |
|---|---|
| `/exit` or `/quit` | End the session |
| `/reset` | Clear conversation history |
| `/compact` | Truncate to last 4 exchanges (frees context window) |
| `/model <provider/model>` | Switch LLM mid-session (e.g. `/model ollama/qwen3-coder`) |
| `/help` | List commands |

The input line shows `ctx: N%` on the right — the percentage of the
model's context window currently consumed. When it climbs past 70–80%,
use `/compact` to free space or `/reset` to start fresh.

While the model is preparing its reply, an animated `thinking…`
indicator appears at the bottom of the chat area; it clears the moment
streaming starts.

**GPU stat:** shows `n/a` in v0.0.3. There's no clean public API for
Apple Silicon GPU usage, and Linux NVIDIA support via `nvidia-smi` is
planned for a later release.

**Ctrl-C** quits the TUI cleanly and restores your previous shell
content (alt-screen mode).

## Re-running the wizard

If you want to switch backends or change models later:

```bash
m init
```

That overwrites the config (and replaces the keychain entry, if you
pick a different hosted provider).

## One-off model override

To try a different model for a single session without changing your
saved config:

```bash
M_MODEL=anthropic/claude-opus-4-7 m
```

The `M_MODEL` value uses the same `provider/model` format as agent
files.

## Subcommands at a glance

`m` does more than the default chat. Full list:

```
m run <agent.md> "task"   # one-shot run
m chat <agent.md>          # REPL with a specific agent
m validate <agent.md>      # validate frontmatter without running
m init                     # re-run the setup wizard
m changelog                # print release history
```

See [Custom agents](agents.html) for writing your own `.md` files.

## Next steps

- **[Configuration](configuration.html)** — config file layout, env
  vars, key storage details.
- **[Providers](providers.html)** — full backend documentation.
- **[Troubleshooting](troubleshooting.html)** — daemon issues, missing
  keys, install errors.
