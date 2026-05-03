# m

`m` is an MD-driven agent CLI for code, infrastructure, and automation work.
Agents are plain Markdown files with YAML frontmatter; the CLI runs them
against your choice of model backend.

## Quick install

**macOS** — download the `.pkg` from the [latest release][releases] and
double-click. Installs `/usr/local/bin/m`.

**Linux (Debian/Ubuntu)** — download the `.deb` and:

```bash
sudo dpkg -i m_*_linux_amd64.deb
```

## First run

```bash
m
```

A four-option wizard runs on first launch — pick Ollama+Qwen3-Coder,
Anthropic, OpenAI, or a LiteLLM proxy. After setup, every subsequent
`m` drops you straight into a chat with your default agent.

## Documentation

Full docs at **<https://subzone.github.io/m/>** — covering installation,
configuration, the four supported providers, custom agents, and
troubleshooting.

## License

MIT License. See LICENSE file for details.

[releases]: https://github.com/subzone/m/releases/latest
