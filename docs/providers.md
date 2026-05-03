---
title: Providers
layout: default
nav_order: 5
---

# Providers

`m` supports six model backends. The first-launch wizard configures
one; you can switch later with `m config`, `m init`, or `/model` in chat.

## At a glance

| Provider | Cost | Privacy | Setup | Quality |
|---|---|---|---|---|
| Ollama + Qwen | Free | 100% local | Medium (~5–20 GB pull) | Good for code |
| Anthropic | Paid per-token | Sent to Anthropic | Low (paste key) | Excellent |
| OpenAI | Paid per-token | Sent to OpenAI | Low (paste key) | Excellent |
| Google Gemini | Paid per-token | Sent to Google | Low (paste key) | Excellent, 1M context |
| Alibaba Cloud | Paid per-token | Sent to Alibaba | Low (paste key) | Good for code (Qwen) |
| LiteLLM | Depends | Depends | Medium (need proxy) | Depends on backend |

## Ollama (local)

The wizard auto-installs Ollama if missing:

- macOS: `brew install ollama` (Homebrew required)
- Linux: `curl -fsSL https://ollama.com/install.sh | sh`

Both prompts confirm before running. After install, the wizard tries
to start the daemon via `brew services start ollama` (macOS) or relies
on the systemd unit set up by the install script (Linux). If neither
brings the daemon up, `m` falls back to launching `ollama serve`
itself as a background child for the rest of the session — note that
**this child dies with `m`**, so for persistence run `brew services
start ollama` (macOS) or `systemctl --user enable --now ollama`
(Linux) once setup is complete.

### Choosing a model

The wizard offers:

1. **`qwen3-coder`** (default) — latest tag, always exists in Ollama's
   library.
2. **`qwen2.5-coder:7b`** — small, well-tested fallback (~5 GB).
3. **Custom** — type any tag from
   <https://ollama.com/library/qwen3-coder> or any other model
   (e.g. `llama3:8b`, `mistral`, etc.).

Note: `m` doesn't restrict you to coder models. It only stores
`provider: ollama` and `model: <tag>` — anything Ollama can serve
works.

### Pointing at a remote Ollama

If the daemon runs on a different machine, set:

```bash
export OLLAMA_HOST=http://10.0.0.1:11434
m
```

Or stash it in `config.yaml`:

```yaml
provider: ollama
model: qwen3-coder
base_url: http://10.0.0.1:11434
```

### Tool support

Ollama tool calling requires Ollama 0.4+ and a model that advertises
function-calling. Qwen3-Coder does. If you pick a model that doesn't,
agent tool calls will fail at runtime — the chat still works, just
without `shell` / `fs_read` / etc.

## Anthropic (Claude)

The wizard prompts for an API key (input is hidden if you're on a
real terminal). Keys are validated for the `sk-ant-` prefix as a
warning — not rejected, in case the format ever changes.

Default model: `claude-sonnet-4-6`. Override per-session with
`M_MODEL=anthropic/claude-opus-4-7 m` or by editing `config.yaml`.

Get a key: <https://console.anthropic.com/>

## OpenAI (GPT)

Same flow as Anthropic. Default model: `gpt-4o-mini` (cheap and
capable). Override with `M_MODEL=openai/gpt-4o m` etc.

`m` honors `OPENAI_BASE_URL` too, so this provider works with Azure
OpenAI or any other OpenAI-compatible endpoint *if* you set the env
var explicitly. (For LiteLLM, prefer the dedicated LiteLLM provider —
it's cleaner.)

Get a key: <https://platform.openai.com/api-keys>

## Google Gemini

Gemini models are accessed through Google's OpenAI-compatible endpoint.
The wizard prompts for an API key and offers three models:

- **`gemini-2.5-flash`** (default) — fast, cheap, 1M context
- **`gemini-2.5-pro`** — highest quality, 1M context
- **`gemini-2.0-flash`** — previous generation, fast

Get a key: <https://aistudio.google.com/apikey>

Override per-session: `M_MODEL=gemini/gemini-2.5-pro m`

Gemini uses the OpenAI-compatible adapter with `WithCompat()` enabled,
which disables OpenAI-specific features (`stream_options`, `json_schema`
response format) that the Gemini endpoint doesn't support.

## Alibaba Cloud (DashScope)

Alibaba's Qwen models are accessed through the DashScope
OpenAI-compatible endpoint. The wizard offers:

- **`qwen-plus`** (default) — good balance of quality and cost
- **`qwen-turbo`** — fastest, cheapest
- **`qwen-max`** — highest quality

Get a key: <https://dashscope.console.aliyun.com/>

Override per-session: `M_MODEL=alibaba/qwen-plus m`

Uses the same OpenAI-compatible adapter as Gemini.

## LiteLLM (proxy / self-hosted)

[LiteLLM](https://github.com/BerriAI/litellm) exposes an
OpenAI-compatible `/v1/chat/completions` endpoint and routes to many
backend providers. Useful when:

- You want a single endpoint that fronts multiple providers
- You self-host inference and need an OpenAI-compatible gateway
- Your org runs a shared LLM proxy with billing/audit centralized

The wizard asks for:

- **Base URL** — e.g. `http://localhost:4000` (default LiteLLM port)
- **Model id** — whatever your proxy router exposes
- **API key** — leave blank if your proxy is unauthenticated; `m`
  stores `no-auth` as a placeholder so the Authorization header
  isn't empty

Under the hood the LiteLLM provider is the OpenAI client with
`OPENAI_BASE_URL` swapped for your proxy URL. Streaming, tool calling,
and stop reasons are all OpenAI-compatible.

## Switching providers

Two ways:

```bash
m init                    # full re-run of the wizard
```

or:

```bash
$EDITOR ~/.config/m/config.yaml      # Linux
$EDITOR "~/Library/Application Support/m/config.yaml"   # macOS
```

If you switch to a provider whose key isn't in the keychain yet,
the next `m` will fail with a clear "key not found" error. Just
run `m init` and pick that provider to add the key.

## Next steps

- **[Custom agents](agents.html)** — write your own MD agents.
- **[Troubleshooting](troubleshooting.html)** — provider-specific
  failures.
