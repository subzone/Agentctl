# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.0.19] - 2026-07-04

### Added
- Homebrew install: `brew tap subzone/tap && brew install subzone/tap/m`
- Auto-update Homebrew formula on every release via CI
- SECURITY.md, CODE_OF_CONDUCT.md, CODEOWNERS
- Issue templates (bug report, feature request) and PR template
- CI workflow on every push/PR (vet, lint, race tests, build, validate agents)
- Dependabot for Go modules and GitHub Actions
- Branch protection, tag protection, repo topics
- `.goreleaser.yml` explicit config

## [0.0.18] - 2026-07-04

### Added
- Session persistence with AES-256-GCM encryption and autosave
- `/save`, `/sessions`, `/resume` slash commands
- `web_fetch` tool — fetch URLs and extract readable text (stdlib-only)
- `/models` command — numbered model picker with live API scanning
- `/themes` command — 9 built-in themes (nord, dracula, gruvbox, tokyonight, catppuccin, solarized)
- `m list` — agent discovery command
- Token-based context compaction with per-model context windows
- Reasoning model support (MiniMax-M2.5, DeepSeek-R1 thinking indicator)
- Alibaba token plan support (custom base URL, GLM-5, MiniMax-M2.5)
- Screenshot gallery on docs landing page
- SECURITY.md, CODE_OF_CONDUCT.md, issue/PR templates
- CI workflow on every push/PR (not just tags)
- Dependabot for Go modules and GitHub Actions

### Changed
- TUI tool confirmation: auto-approve read-only tools, y/n for destructive
- DashScope model filtering: 154 → 45 models (skip non-chat, deduplicate)
- Context compaction now token-based instead of message-count-based

### Fixed
- TUI scrolling jitter during streaming
- `/themes` command caught by `/theme` prefix handler
- Path traversal in EncryptedFileStore session IDs
- Data race in autosave goroutine (snapshot before launch)

## [0.0.17] - 2026-05-03

### Added
- Product landing page at subzone.github.io/Agentctl
- 7-page documentation site (EN + SR)
- DevOps agents: k8s-debug, terraform-plan, helm-deploy
- Jira/Confluence MCP integration and ticket agents
- Steva Đubre & Steve Trash personality agents
- GLM model pricing and context windows

## [0.0.16] - 2026-05-01

### Added
- Live model scanning for all hosted providers
- Wizard: paste key → scan API → pick from numbered list
- API key fallback: keychain first, then environment variable
- `/models` slash command (initial version)
- Delegate tool model override

## [0.0.15] - 2026-04-30

### Added
- Alibaba Cloud (DashScope) provider
- LiteLLM proxy provider
- Gemini provider via OpenAI-compat endpoint

## [0.0.14] - 2026-04-28

### Added
- Full-screen TUI with bubbletea
- Token/cost/context indicators
- Theme system (matrix, default, minimal)
- System stats (CPU/RAM/GPU/Disk)

[0.0.19]: https://github.com/subzone/Agentctl/compare/v0.0.18...v0.0.19
[0.0.18]: https://github.com/subzone/Agentctl/compare/v0.0.17...v0.0.18
[0.0.17]: https://github.com/subzone/Agentctl/compare/v0.0.16...v0.0.17
[0.0.16]: https://github.com/subzone/Agentctl/compare/v0.0.15...v0.0.16
[0.0.15]: https://github.com/subzone/Agentctl/compare/v0.0.14...v0.0.15
[0.0.14]: https://github.com/subzone/Agentctl/releases/tag/v0.0.14
