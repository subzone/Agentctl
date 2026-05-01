---
title: Troubleshooting
---

[← Docs home](./)

# Troubleshooting

Common issues and how to fix them.

## "ollama daemon is not reachable at http://localhost:11434"

The wizard couldn't talk to Ollama after install. Almost always one of:

1. **Daemon not running.** On macOS, `brew services start ollama` may
   have failed silently in earlier versions. Run it manually:

   ```bash
   brew services start ollama
   ```

   Or, for a one-off session, in a separate terminal:

   ```bash
   ollama serve
   ```

   Leave that running, then re-run `m init` (or just `m`).

2. **Different host.** If your Ollama runs on a remote box, set
   `OLLAMA_HOST=http://host:11434` before invoking `m`.

3. **First-run race.** v0.0.1's wizard waited only 2 seconds before
   giving up. v0.0.2 polls for 15 s with backoff and falls back to
   launching `ollama serve` itself. Update with the latest `.pkg` or
   `.deb`.

## "ollama pull qwen3-coder:7b: pull model manifest: file does not exist"

Hit if you're on v0.0.1 and chose option 1 in the wizard. The
hardcoded size tags didn't actually exist on Ollama's library.

**Fix:** Update to v0.0.2+. Or in v0.0.1, re-run `m init`, choose
option 1, then **option 4 (custom)** at the size prompt, and type
`qwen3-coder` (no size suffix).

If you've already saved a bad config, edit it directly:

```bash
$EDITOR "~/Library/Application Support/m/config.yaml"   # macOS
$EDITOR ~/.config/m/config.yaml                         # Linux
```

Set `model: qwen3-coder` and re-run `ollama pull qwen3-coder` from a
shell.

## macOS Gatekeeper: "cannot be opened because the developer cannot be verified"

Expected — the `.pkg` is unsigned until the project pays for an Apple
Developer Program membership.

**Right-click → Open** on the `.pkg` to bypass once. Or:

```bash
xattr -d com.apple.quarantine ~/Downloads/m_*.pkg
```

Then double-click normally. The installed binary itself doesn't
trigger Gatekeeper.

## "ANTHROPIC_API_KEY is not set" / "OPENAI_API_KEY is not set"

The provider tried to construct itself but no key was found. Either:

1. **No key in keychain.** Run `m init` and pick the provider; it'll
   prompt for the key and store it.

2. **Wrong provider in config.** If `config.yaml` says
   `provider: anthropic` but you only have an OpenAI key in the
   keychain, switch the provider field or run `m init`.

3. **Sandboxed env.** Some sandboxes block keychain access; in that
   case set the env var explicitly before invoking `m`:

   ```bash
   ANTHROPIC_API_KEY=sk-ant-... m
   ```

## "secret-tool not found: install libsecret-tools (apt) or libsecret (dnf/pacman)"

You're on Linux without `libsecret`. The wizard offers to install it:

```
About to run: sudo apt-get install -y libsecret-tools
Proceed (you may be prompted for your sudo password)? [Y/n]:
```

If you bail, install manually:

| Distro | Command |
|---|---|
| Debian / Ubuntu | `sudo apt-get install libsecret-tools` |
| Fedora / RHEL | `sudo dnf install libsecret` |
| Arch | `sudo pacman -S libsecret` |
| Alpine | `sudo apk add libsecret` |

Headless Linux without a D-Bus / gnome-keyring session can't store
keys — set `ANTHROPIC_API_KEY` / `OPENAI_API_KEY` env vars instead.

## "agent failed validation"

The agent file's frontmatter has missing required fields or
unrecognized fields. Run:

```bash
m validate my-agent.md
```

It prints each issue with the field name and reason. Common causes:

- Missing `name`, `type`, or `model`
- `type:` other than `agent`
- `model` not in `provider/model` form
- Unknown field (typo in YAML)

## "no input"

The wizard hit EOF on stdin before getting an answer. Usually means
you piped an empty input or the parent shell closed stdin. Run it
interactively:

```bash
m init
```

(Don't pipe anything in.)

## Bare `m` shows no banner / starts chat instantly

You're past the first run — the banner is shown, the wizard isn't
re-run. To re-trigger the wizard:

```bash
m init
```

To force a "first install" experience, delete the config file:

```bash
rm "~/Library/Application Support/m/config.yaml"   # macOS
rm ~/.config/m/config.yaml                         # Linux
```

## Release notes don't appear after update

Two possibilities:

1. **Not a tagged release build.** If `m --version` reports `dev`,
   you're on a built-from-source binary; release notes are skipped to
   avoid polluting state. Install via `.pkg` / `.deb`.

2. **State already at current version.** `m` only shows notes once
   per version. Force re-display by deleting state:

   ```bash
   rm "~/Library/Application Support/m/state.yaml"   # macOS
   rm ~/.config/m/state.yaml                         # Linux
   ```

   Or just run `m changelog` to see the full history regardless of
   state.

## Still stuck

[Open an issue](https://github.com/subzone/m/issues/new) with:

- `m --version` output
- OS + version
- The exact command and full error output

## Next steps

- **[Configuration](configuration.html)** — file layout and env vars
- **[Providers](providers.html)** — provider-specific notes
