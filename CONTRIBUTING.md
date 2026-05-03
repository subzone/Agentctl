# Contributing to AgentCTL

AgentCTL is early-stage software (~1 month of evenings). Bugs, design feedback,
and PRs are all welcome. Before opening a PR for a non-trivial change, please
open an issue first so we can align on scope — the architecture is small enough
that one wrong abstraction hurts.

## Quick Links

- **Issues:** https://github.com/subzone/Agentctl/issues
- **Discussions:** https://github.com/subzone/Agentctl/discussions
- **Architecture:** See [PLAN.md](PLAN.md) for the full design walk-through

## Reporting Bugs

1. Check existing issues to avoid duplicates.
2. Open a new issue with:
   - Clear title (e.g., "agent run fails with OpenAI model when tool_use follows tool_result")
   - Steps to reproduce
   - Expected behavior
   - Actual behavior (include error messages, logs)
   - Environment: OS, Go version, model/provider used

## Suggesting Features

1. Open a discussion first (not an issue) for major features.
2. Explain the use case, not just the solution.
3. Be patient — this is a side project with limited bandwidth.

## Pull Requests

### Before You Code

For anything beyond a typo fix or small bug:

1. Open an issue describing the problem and your proposed solution.
2. Wait for feedback. If the approach is approved, proceed.
3. If there's no response after a week, ping the issue or start a discussion.

This saves everyone time. The codebase is ~8.8k LOC — a wrong abstraction
affects multiple files.

### Code Style

- **Go:** Run `gofmt` (or `goimports`) before committing. Non-negotiable.
- **Lint:** `golangci-lint run ./...` must pass.
- **Tests:** All changes require tests. Run `go test ./...` before pushing.
- **Comments:** Document exported functions with godoc-style comments.
- **No SDK deps:** LLM clients are stdlib-only. Do not add Anthropic/OpenAI SDKs.

### Commit Messages

Follow conventional commits:

```
fix: nil check in engine tool dispatch
feat: add /compact slash command to truncate history
docs: update README with new agent examples
refactor: extract Session from engine.Run
test: add concurrent MCP call correlation tests
```

### PR Checklist

Before submitting:

- [ ] `go test ./...` passes
- [ ] `go vet ./...` passes
- [ ] `golangci-lint run ./...` passes (or `make lint`)
- [ ] New code has tests
- [ ] Commit messages follow conventional format
- [ ] PR description explains the change and references the issue

### Review Process

- PRs are reviewed within a few days (side project, not full-time).
- Be responsive to feedback. Address all comments before merging.
- Squash commits on merge unless there's a reason to keep history.

## Development Setup

```bash
git clone https://github.com/subzone/Agentctl.git
cd Agentctl
make build    # produces ./m binary
make test     # runs go test ./...
make lint     # runs golangci-lint
```

Requires Go 1.26+.

## Project Structure

```
cmd/m/                CLI entry, TUI, REPL, slash commands
internal/engine/      Session loop, tool dispatch, structured output
internal/llm/         Provider registry + 6 stdlib-only clients
internal/tools/       Built-in tool implementations
internal/mcp/         JSON-RPC stdio client, tool adapter
internal/config/      Frontmatter parsing, agent/MCP/skill schemas
internal/ports/       ConfigSource, Secrets, StateStore interfaces
internal/adapters/    Keychain (macOS/libsecret), file-backed stores
examples/agents/      27+ ready-to-use agents
examples/mcp/         3 MCP server definitions
docs/                 Static product site (EN + SR), GitHub Pages
```

**Hard rule:** Nothing in `internal/engine/` or `internal/tools/` imports
`client-go`, `controller-runtime`, or HTTP server libs. That's how the k8s
door stays open.

## Adding a New Agent

Agents are Markdown files with YAML frontmatter. Add one to `examples/agents/`:

```markdown
---
name: my-agent
type: agent
model: anthropic/claude-sonnet-4-6
tools:
  - shell
  - fs_read
temperature: 0.3
---
Your system prompt here.
```

Run with:

```bash
m chat examples/agents/my-agent.md
```

## Adding a New Provider

1. Create `internal/llm/myprovider/` directory.
2. Implement `llm.Provider` interface:
   - `Stream(ctx, req) <-chan Event`
   - Normalize stop reasons to engine's vocabulary
3. Register in `init()` via `llm.Register("myprovider", NewClient)`.
4. Blank-import in `cmd/m/main.go`.
5. Add tests using `httptest.Server`.
6. Update docs.

See `internal/llm/openai/` or `internal/llm/ollama/` for examples.

## Adding a New Tool

1. Create `internal/tools/mytool.go`.
2. Implement `tools.Tool` interface:
   - `Name() string`
   - `Schema() llm.ToolSchema`
   - `Run(ctx, input) (string, error)`
3. Add to `tools.Builtins()` map.
4. Add tests.
5. Update docs.

See `internal/tools/shell.go` or `internal/tools/fs_read.go` for examples.

## License

MIT. By contributing, you agree that your contributions will be licensed under
the same terms.

---

Questions? Open a discussion: https://github.com/subzone/Agentctl/discussions