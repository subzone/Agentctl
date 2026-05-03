---
title: Model comparison & cost guide
---

[← Docs home](./)

# Model comparison & cost guide

`m` supports 6 providers and dozens of models. This page helps you pick
the right one for your use case and budget.

## Cost comparison

Prices are per million tokens (as of mid-2025). A typical coding session
uses 50K–200K input tokens and 10K–50K output tokens.

### Hosted providers

| Model | Input $/M | Output $/M | Context | Tool calling | Quality |
|---|---|---|---|---|---|
| **claude-sonnet-4-6** | $3.00 | $15.00 | 200K | ✅ Excellent | Best overall |
| **claude-haiku-4-5** | $1.00 | $5.00 | 200K | ✅ Reliable | Good, fast |
| **claude-opus-4** | $15.00 | $75.00 | 200K | ✅ Excellent | Highest quality |
| **gpt-4o** | $2.50 | $10.00 | 128K | ✅ Reliable | Excellent |
| **gpt-4o-mini** | $0.15 | $0.60 | 128K | ✅ Reliable | Good for simple tasks |
| **gpt-4.1** | $2.00 | $8.00 | 1M | ✅ Reliable | Excellent, huge context |
| **gpt-4.1-mini** | $0.40 | $1.60 | 1M | ✅ Reliable | Good, huge context |
| **gemini-2.5-flash** | $0.15 | $0.60 | 1M | ⚠️ Via compat | Good, very cheap |
| **gemini-2.5-pro** | $1.25 | $10.00 | 1M | ⚠️ Via compat | Excellent |
| **qwen-plus** (Alibaba) | $0.80 | $2.00 | 131K | ⚠️ Via compat | Good |
| **qwen-turbo** (Alibaba) | $0.30 | $0.60 | 131K | ⚠️ Via compat | OK |
| **deepseek-v3** (via DashScope) | $0.27 | $1.10 | 64K | ✅ Yes | Good |
| **deepseek-chat** (direct API) | $0.14 | $0.28 | 64K | ✅ Yes | Good |

### Local (free)

| Model | RAM needed | Tool calling | Quality |
|---|---|---|---|
| **qwen3-coder** (Ollama) | ~18 GB | ⚠️ Inconsistent | Good for code |
| **qwen2.5-coder:7b** (Ollama) | ~5 GB | ⚠️ Weak | OK for simple tasks |
| **llama3:8b** (Ollama) | ~5 GB | ❌ No | Chat only |

## Cost estimates for typical sessions

| Session type | Tokens (in/out) | Claude Sonnet | Haiku 4.5 | GPT-4o-mini | Qwen-turbo | DeepSeek-v3 |
|---|---|---|---|---|---|---|
| Quick question | 5K / 1K | $0.03 | $0.01 | $0.001 | $0.002 | $0.002 |
| Code review | 50K / 10K | $0.30 | $0.10 | $0.01 | $0.02 | $0.02 |
| Full dev session (1hr) | 200K / 50K | $1.35 | $0.45 | $0.06 | $0.09 | $0.11 |
| Heavy refactoring | 500K / 100K | $3.00 | $1.00 | $0.14 | $0.21 | $0.25 |

## Recommendations by use case

### Best quality (money no object)
- **claude-sonnet-4-6** or **claude-opus-4**
- Best tool calling, best reasoning, best multilingual support
- Use for: complex refactoring, architecture decisions, code review

### Best value for dev work
- **claude-haiku-4-5-20251001** — fast, reliable tools, $1/M input
- Use for: daily coding, debugging, file editing

### Cheapest hosted
- **deepseek-v3** via DashScope — $0.27/M input, good tool support
- **gpt-4o-mini** — $0.15/M input, reliable but less capable
- **gemini-2.5-flash** — $0.15/M input, 1M context, but tool calling via compat
- Use for: high-volume tasks, CI/CD automation, bulk operations

### Free (local)
- **ollama/qwen2.5-coder:7b** — works on 8GB+ machines
- Tool calling is unreliable; best for simple chat, not dev workflows
- Use for: offline work, privacy-sensitive tasks, experimentation

### Multilingual (Serbian, etc.)
- **claude-sonnet-4-6** — best multilingual, maintains character
- **claude-haiku-4-5** — good multilingual, cheaper
- **deepseek-v3** — decent multilingual, very cheap
- **qwen-plus** — trained on multilingual data, good for Asian languages
- Local models (7B) — poor multilingual support

## Switching models

Switch mid-session without restarting:

```
» /model anthropic/claude-haiku-4-5-20251001
switched to anthropic/claude-haiku-4-5-20251001
```

Or set a permanent default:

```bash
m config
# Choose: d) Set default model
```

## Custom agent example: Steva Đubre

`m` supports custom agent personalities. Here's an example — a grumpy
Serbian DevOps engineer:

```bash
m chat examples/agents/steva-djubre.md
```

```
» Zdravo Steva, možeš li da pogledaš zašto mi ne radi deploy?
Jao brate, opet deploy ne radi? Ajde da vidim šta si zeznuo ovaj put...
→ fs_list {"path": "."}
← 245 bytes
→ fs_read {"path": "Dockerfile"}
← 1200 bytes
Ma naravno, ko normalan stavlja hardkodiran port u Dockerfile?
Evo, popravljam, ali ti duguješ pivo za ovo.
→ fs_write {"path": "Dockerfile", "mode": "patch", ...}
```

Steva responds exclusively in Serbian, uses tools proactively, and
maintains his grumpy character throughout. He works best with
**claude-sonnet-4-6** (best Serbian + tool calling) or
**claude-haiku-4-5** (cheaper, still good).

To make Steva your default agent:

```bash
m config
# Choose: g) Set default agent
# Path: /path/to/examples/agents/steva-djubre.md
```

### Creating your own custom agent

Any `.md` file with YAML frontmatter works. See
[Custom agents](agents.html) for the full guide. Key fields:

```yaml
---
name: my-agent
type: agent
model: anthropic/claude-sonnet-4-6
tools: [shell, fs_read, fs_write, fs_list, git, test_run]
temperature: 0.7
---
Your system prompt here. Define the personality, rules, and workflow.
```

## Provider setup

| Provider | How to get a key | Wizard option |
|---|---|---|
| Anthropic | [console.anthropic.com](https://console.anthropic.com/) | `m init` → 2 |
| OpenAI | [platform.openai.com/api-keys](https://platform.openai.com/api-keys) | `m init` → 3 |
| Google Gemini | [aistudio.google.com/apikey](https://aistudio.google.com/apikey) | `m init` → 4 |
| Alibaba DashScope | [dashscope.console.aliyun.com](https://dashscope.console.aliyun.com/) | `m init` → 5 |
| Ollama | No key needed | `m init` → 1 |
| LiteLLM | Depends on your proxy | `m init` → 6 |

The wizard scans each provider's API for available models after you
paste your key — no need to memorize model IDs.

## Next steps

- **[Providers](providers.html)** — detailed per-provider documentation
- **[Custom agents](agents.html)** — write your own agents
- **[Configuration](configuration.html)** — config files and env vars
