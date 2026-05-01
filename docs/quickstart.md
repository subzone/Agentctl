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

The REPL prompt is `»`. Type a message and hit Enter; the model replies
inline. Slash commands:

- `/exit` or `/quit` — end the session
- `/reset` — clear conversation history
- `/help` — list commands

```
» hello
Hello! How can I help you today?
» /exit
```

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
