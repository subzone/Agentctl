# Security Policy

## Supported Versions

| Version | Supported          |
|---------|--------------------|
| 0.0.x   | :white_check_mark: |

## Reporting a Vulnerability

**Do not open a public issue for security vulnerabilities.**

Instead, please email **security@agentctl.dev** or use
[GitHub's private vulnerability reporting](https://github.com/subzone/Agentctl/security/advisories/new).

Include:
- Description of the vulnerability
- Steps to reproduce
- Impact assessment
- Suggested fix (if any)

You'll receive an acknowledgment within 48 hours. We aim to release a fix
within 7 days for critical issues.

## Scope

Security-relevant areas of AgentCTL:

- **API key storage** — keys are stored in the OS keychain (macOS Keychain /
  Linux libsecret), never in plaintext config files.
- **Session encryption** — saved sessions use AES-256-GCM with a key stored
  in the OS keychain.
- **Shell tool** — executes arbitrary commands. The TUI prompts for
  confirmation on destructive tools (shell, fs_write, git).
- **File operations** — fs_write shows a diff preview before writing.
- **Web fetch** — fetches URLs; no credentials are sent.

## Out of Scope

- Vulnerabilities in upstream LLM providers (Anthropic, OpenAI, etc.)
- Social engineering attacks via crafted prompts (prompt injection)
- Denial of service via large inputs (bounded by provider context limits)
