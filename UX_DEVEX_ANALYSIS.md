# UX/UI/DevEx Analysis & Improvement Roadmap

**Analiza iz ugla korisnika** - šta fali, šta je confusing, šta treba da se popravi.

**Status:** Alpha (v0.0.25) - mnogo toga radi, ali UX nije user-friendly.

---

## 🎯 Executive Summary

AgentCTL je **tool za developere**, ali UX je dizajniran za **autor** (developer koji piše za sebe). User koji prvi put pokrene `m` će se zbuniti:

1. Binary ime vs product ime (`m` vs AgentCTL)
2. First-run wizard je previše komplikovan (5+ minuta)
3. TUI header zauzima pola screen-a
4. Slash commands su previše da se memorise
5. Tool confirmation flow je confusing

**Goal:** User treba da može da pokrene `m` i dobije useful output za **< 2 minuta** bez da čita README.

---

## 🔴 HIGH PRIORITY UX ISSUES

### 1. Binary Name vs Product Name - TOTAL CONFUSSION

**Current:**
```
Binary: m
Product: AgentCTL
README: "The CLI binary is `m` for ergonomics"
```

**Problem:** User instalira `m`, traži `agentctl` u docs, neće ga naći. Ergonomics argument je valid for author (type `m` 50x/day), ali normal user traži `agentctl` ili `agent`.

**Fix:**
- [ ] Rename binary to `agentctl` (primary name)
- [ ] Add symlink/alias `m` (short form for power users)
- [ ] Update README: "Binary: agentctl (alias: m for ergonomics)"
- [ ] Update goreleaser.yaml to produce both binaries
- [ ] Update Homebrew formula to install both

**Effort:** Medium (build system changes)
**Impact:** High (branding clarity)

---

### 2. First-Run Wizard - PREVIŠE KOMPLIKOVANO

**Current:**
```go
// init.go
1. Pick provider (6 options: Ollama, Anthropic, OpenAI, Gemini, Alibaba, LiteLLM)
2. If Ollama: check/install ollama binary
3. Start ollama daemon (brew services or background)
4. Pick model tag (3 options + custom URL)
5. Pull model (5-20 GB download, no progress bar)
```

**Problem:** First run je 5+ minuta. User misli "šta je ovo, Linux kernel build?". Nema default choice, nema progress indicator.

**Fix:**
- [ ] Add "Quick Start" mode: default Ollama + `qwen3-coder` (one choice)
- [ ] Add "Advanced" mode: `/init advanced` for other providers
- [ ] Skip model pull if already installed (check with `ollama list`)
- [ ] Add progress bar for `ollama pull` (not just raw output)
- [ ] Add timeout warning if pull takes > 2 min
- [ ] Add cancel option during pull (Ctrl-C)

**Effort:** High (wizard redesign)
**Impact:** High (first impression)

**Implementation Notes:**
```go
// Proposed wizard flow:
func runWizardQuick(w *wiz) (*userconfig.Config, error) {
	fmt.Fprintln(w.out, "Quick setup: Ollama + Qwen3-Coder (local, free)")
	fmt.Fprintln(w.out, "  [Enter] to accept defaults")
	fmt.Fprintln(w.out, "  [A] for advanced setup (other providers)")
	
	choice, _ := w.prompt("Choice: ")
	if choice == "a" || choice == "A" {
		return runWizardAdvanced(w)
	}
	// Auto-detect/install Ollama, pull qwen3-coder, done
	return setupOllamaQuick(w)
}
```

---

### 3. TUI Header - PREVIŠE INFORMACIJA

**Current:**
```go
// tui.go View() - header has:
- Banner (6 lines ASCII art "M")
- Version line
- Copyright line
- Token box (In/Out/Total/Cost/Ctx %)
- Stats table (CPU/RAM/GPU/Disk)
- Provider/model label
- Command bar
- CWD label
// Total: 11+ rows on 80x24 terminal
```

**Problem:** Header zauzima pola screen-a. User ne vidi chat history. Stats table je useless (GPU/Disk irrelevant for chat).

**Fix:**
- [ ] Add compact mode: header = 1 line (provider/model + tokens + ctx %)
- [ ] Add toggle: `/header` or `H` key to switch modes
- [ ] Hide stats table default (show with `/stats` command)
- [ ] Show banner only on first run or `/banner` command
- [ ] Remove GPU/Disk from stats (keep CPU/RAM only)
- [ ] Add responsive layout: hide elements on small terminals (< 80 width)

**Effort:** Medium (TUI layout refactor)
**Impact:** High (screen real estate)

**Implementation Notes:**
```go
// Proposed compact header:
func (m tuiModel) renderCompactHeader() string {
	return fmt.Sprintf("%s/%s | In: %s Out: %s Cost: %s | Ctx: %d%% | /help",
		m.provider, m.model,
		formatTokens(m.usage.InputTokens),
		formatTokens(m.usage.OutputTokens),
		formatCost(estimateCost(m.usage, m.model)),
		contextPercent(m.lastIn, m.model))
}
```

---

### 4. Slash Commands - PREVIŠE DA SE MEMORIŠE

**Current:**
```
/exit /quit /reset /compact /undo /trust /debug /model /models /theme /themes /save /sessions /resume /config /spec /help
// 16+ commands
```

**Problem:**
- `/model` vs `/models` je confusing (switch vs list)
- Nema aliases for common commands
- `/help` lista all commands, no grouping

**Fix:**
- [ ] Group commands: `/session save`, `/session list`, `/session resume`
- [ ] Add aliases: `/q` = `/exit`, `/r` = `/reset`, `/u` = `/undo`
- [ ] Make `/model` auto-list if empty: `/model` = `/models`
- [ ] Update `/help` to show groups, not flat list
- [ ] Add `/help <group>` for detailed help (e.g., `/help session`)
- [ ] Add command autocomplete (Tab key)

**Effort:** Medium (command parser refactor)
**Impact:** High (discoverability)

**Proposed Command Groups:**
```
Session:
  /session save [label]  - save with optional label
  /session list          - list saved sessions
  /session resume <id>   - resume by id or number
  /session reset         - clear history (alias: /r)

Model:
  /model [provider/model] - switch or list if empty
  /models                 - list available (alias for /model)
  /config                 - open config manager

Theme:
  /theme [name]          - switch or list if empty
  /theme preview <name>  - preview before switching

Safety:
  /trust                 - enable auto-approve
  /trust off             - disable auto-approve
  /undo                  - revert last write (alias: /u)

Debug:
  /debug                 - enable LLM trace
  /debug off             - disable
  /stats                 - show system stats

Help:
  /help                  - show command groups
  /help <group>          - detailed help
  /exit                  - quit (alias: /q)
```

---

### 5. Tool Confirmation Flow - CONFUSING

**Current:**
```go
// chat.go
readOnlyTUITools: fs_read, fs_list, web_fetch, test_run, delegate, code_search
// Auto-approved in TUI mode (user doesn't see prompt)

Destructive tools: shell, fs_write, git
// Prompt [y/n] in viewport

/trust mode: auto-approve ALL, but dangerous patterns double-confirm
// No indicator that /trust is enabled
```

**Problem:**
- User ne zna šta se auto-approve i šta prompt
- `/trust` je dangerous - no warning when enabled
- Confirmation inline in viewport (not popup)
- Dangerous patterns undocumented (34 patterns)

**Fix:**
- [ ] Show all tool calls in viewport (even auto-approved): `→ fs_read path=file.go [auto]`
- [ ] Add `/trust` warning: "⚠️ Trust mode ON - agent can modify files without confirmation"
- [ ] Add `/trust off` confirmation: "Disable trust mode? [y/n]"
- [ ] Add tool confirmation popup (modal, not inline)
- [ ] Document dangerous patterns in README
- [ ] Add custom dangerous patterns in agent config: `dangerous_patterns: ["custom-cmd"]`

**Effort:** High (confirmation flow refactor)
**Impact:** High (safety)

**Implementation Notes:**
```go
// Proposed confirmation popup:
type confirmPopup struct {
	tool    string
	input   string
	message string
	action  string // "Allow", "Deny", "Trust"
}

func (m tuiModel) renderConfirmPopup(p confirmPopup) string {
	return fmt.Sprintf(`
┌─────────────────────────────────────┐
│ Tool Confirmation                   │
│                                     │
│ Tool: %s                            │
│ Input: %s                           │
│                                     │
│ %s                                  │
│                                     │
│ [y] Allow  [n] Deny  [t] Trust all │
└─────────────────────────────────────┘
`, p.tool, p.input, p.message)
}
```

---

### 6. Error Handling - NEMA RECOVERY SUGGESTIONS

**Current:**
```go
// chat.go - makeReplErrorIntervention
fmt.Fprintf(stderr, "[%s failed] Press Enter to retry, or type a hint: ", toolName)
// No suggested fix, no docs link, no error code
```

**Problem:**
- Hint explanation missing (user doesn't know what to type)
- No suggested fix
- No docs link
- No error codes

**Fix:**
- [ ] Add error format: `[tool failed] Error: <msg>. Suggestion: <fix>. Docs: <url>`
- [ ] Add hint explanation: "Type a hint to guide the agent (e.g., 'try different path')"
- [ ] Add error codes: `E001: file not found`, `E002: permission denied`
- [ ] Add error reference page in docs
- [ ] Add common errors section in README

**Effort:** Medium (error handling refactor)
**Impact:** High (user recovery)

**Error Codes Proposal:**
```
E001: file not found - check path, use fs_list to explore
E002: permission denied - check file permissions, use sudo if needed
E003: command not found - check if tool is installed
E004: API rate limit - wait or switch model with /model
E005: context exceeded - use /compact to truncate history
E006: network timeout - check connection, retry
E007: invalid JSON - check tool input format
E008: MCP server not found - check MCP config, install server
```

---

## 🟡 MEDIUM PRIORITY UX ISSUES

### 7. Session Management - CONFUSING IDs

**Current:**
```go
// session.go
sessionID: timestamp format "20060102-150405"
// Example: 20260102-150405
```

**Problem:** Session IDs su timestamps - user ne zna šta je to. `/sessions` lista:
```
saved sessions (3):
  1) 20260102-150405
  2) 20260102-151203
  3) 20260102-152345
```
Koja je sesija koja? Nema label, nema description, nema task summary.

**Fix:**
- [ ] Add session labels: `/save label="fix dockerfile"`
- [ ] Add session summary: auto-generate from last user message
- [ ] Update `/sessions` to show: `<label> | <timestamp> | <message count>`
- [ ] Add session search: `/sessions search <keyword>`
- [ ] Add session delete: `/session delete <id>`

**Effort:** Medium
**Impact:** Medium

---

### 8. Model Discovery - FAILS GRACEFULLY

**Current:**
```go
// config.go - discoverDashScope
if resp.StatusCode == http.StatusNotFound {
	return probeDashScopeModels(ctx, key, baseURL)
}
// Probe 20 models in parallel, no progress indicator
```

**Problem:**
- User ne zna šta se probe
- Probe je slow (parallel HTTP calls, 15s timeout)
- No progress indicator
- No fallback model if all fail

**Fix:**
- [ ] Add progress indicator: "Scanning models... (probing 20 candidates)"
- [ ] Add fallback: "No models found. Use /model <id> to set manually. Default: qwen-plus"
- [ ] Cache discovery results (don't probe every time)
- [ ] Add timeout warning: "Discovery timeout, using defaults"
- [ ] Add manual model input if discovery fails

**Effort:** Medium
**Impact:** Medium

---

### 9. Theme System - NEMA PREVIEW

**Current:**
```go
// tui.go - /themes command
9 themes with text descriptions
// User must switch to see actual colors
```

**Problem:**
- No preview before switching
- Minimal theme je "no colors" - koji user želi that?
- No high-contrast option for accessibility

**Fix:**
- [ ] Add theme preview: `/theme preview <name>` show sample output
- [ ] Add high-contrast theme: "accessibility" theme
- [ ] Add theme recommendation: "Recommended: default (light) or dracula (dark)"
- [ ] Add theme screenshot in docs

**Effort:** Low
**Impact:** Medium

---

### 10. Documentation Structure - README PREVIŠE LONG

**Current:**
```
README.md: 200+ lines
Quick Start: 5 steps with confusing example
Known gaps: scary for new users
```

**Problem:**
- Quick Start nije quick (5 steps, confusing example)
- Nema visual diagram
- Nema troubleshooting section
- Known gaps je scary ("alpha, breaking changes")

**Fix:**
- [ ] Simplify Quick Start: 3 steps max (install -> run -> chat)
- [ ] Add visual diagram: ASCII flowchart of workflow
- [ ] Add troubleshooting section: "Common Issues"
- [ ] Rename "Known gaps" to "Roadmap" or "Limitations"
- [ ] Add FAQ section
- [ ] Add screenshots/GIFs

**Effort:** Medium
**Impact:** Medium

---

## 🟢 LOW PRIORITY UX ISSUES

### 11. Thinking Phrases - NEMA ETA

**Current:**
```go
// tui.go - thinkingPhrase
Spinner rotates phrases: "brewing ideas", "reading codebase", etc.
// No progress indicator, no ETA, no timeout warning
```

**Problem:**
- User ne zna šta agent radi (reading file? writing? waiting for API?)
- No progress indicator
- No ETA estimate
- No timeout warning

**Fix:**
- [ ] Add tool-specific phrases: "reading file..." (fs_read), "running test..." (test_run)
- [ ] Add progress: "Step 1/3: reading file"
- [ ] Add timeout warning: "Taking long... Press Esc to cancel"
- [ ] Add ETA estimate based on historical tool duration

**Effort:** Low
**Impact:** Low

---

### 12. Input Blocking - CONFUSING

**Current:**
```go
// tui.go - Update
if m.thinking {
	return m, nil // Block all input
}
// No indicator that input is blocked
```

**Problem:**
- User ne može type kad agent thinking
- No visual indicator (input box looks normal)
- User misli da je bug

**Fix:**
- [ ] Add input placeholder: "Agent thinking... (input blocked)"
- [ ] Add visual indicator: gray out input box
- [ ] Add unblock hint: "Press Esc to cancel agent"

**Effort:** Low
**Impact:** Low

---

### 13. Auto-Save - NEMA INDICATOR

**Current:**
```go
// tui.go - streamMsg done
go func() { autoSaveSnapshot(snap) }()
// Silent, no success/failure indicator
```

**Problem:**
- User ne zna da se save
- No success indicator
- No failure warning

**Fix:**
- [ ] Add auto-save indicator: "💾 Session saved" (flash in status bar)
- [ ] Add failure warning: "⚠️ Session save failed: <error>"
- [ ] Add save progress: "Saving..." (if slow)

**Effort:** Low
**Impact:** Low

---

### 14. Cost Estimates - ROUGH APPROXIMATION

**Current:**
```go
// tui.go - modelPricing
Cost estimates from hardcoded pricing table
// No disclaimer, no actual billing integration
```

**Problem:**
- User ne zna da je rough estimate
- No disclaimer
- No actual billing integration

**Fix:**
- [ ] Add disclaimer: "Cost: ~$0.05 (estimate, check provider billing)"
- [ ] Add actual billing integration (provider APIs)
- [ ] Add cost warning threshold: "Cost exceeds $1.00 - continue? [y/n]"

**Effort:** Low (disclaimer), High (billing integration)
**Impact:** Low

---

### 15. Accessibility - NEMA SCREEN READER SUPPORT

**Current:**
```
TUI is terminal-based:
- No screen reader support
- No high-contrast theme
- Color-only indicators (spinner, diff colors)
```

**Problem:**
- Not accessible for visually impaired users
- No keyboard shortcuts documentation
- No text labels for color indicators

**Fix:**
- [ ] Add screen reader mode: `/accessibility screen-reader`
- [ ] Add high-contrast theme (already in theme list, needs docs)
- [ ] Add keyboard shortcuts: `/help keys`
- [ ] Add text labels: "[thinking]" not just spinner
- [ ] Add audio feedback for tool confirmations

**Effort:** Medium
**Impact:** Low (but important for accessibility)

---

## 🔵 DevEx ISSUES (za developere)

### 16. Agent Definition Schema - UNDOCUMENTED

**Current:**
```yaml
---
name: devops
type: agent
model: anthropic/claude-sonnet-4-6
fallback: [...]
tools: [...]
temperature: 0.3
thinking_phrases: [...]  # Undocumented!
---
```

**Problem:**
- Schema defined in code (`internal/config/schema.go`)
- No docs page
- No validation error messages
- No example for each field
- `thinking_phrases` field undocumented

**Fix:**
- [ ] Add schema docs page: "Agent Definition Reference"
- [ ] Add validation errors: "Unknown field 'foo'. Valid fields: name, type, model, tools, ..."
- [ ] Add examples for each field
- [ ] Add schema JSON/YAML spec file

**Effort:** Medium
**Impact:** High (for agent developers)

---

### 17. MCP Integration - UNDOCUMENTED

**Current:**
```yaml
// examples/mcp/jira.md
---
name: jira
type: mcp
transport: stdio
command: ["mcp-jira"]
---
```

**Problem:**
- 5 MCP examples, no docs page
- No transport explanation (stdio vs http vs sse)
- No tool namespacing explanation
- No troubleshooting

**Fix:**
- [ ] Add MCP docs page: "MCP Integration Guide"
- [ ] Add transport explanation: "stdio = subprocess, http = POST, sse = streaming"
- [ ] Add tool namespacing: "MCP tools are namespaced: jira__get_issue"
- [ ] Add troubleshooting: "MCP server not found: install with npm install mcp-jira"
- [ ] Add MCP server list in docs

**Effort:** Medium
**Impact:** High (for MCP users)

---

### 18. Dangerous Patterns - UNDOCUMENTED

**Current:**
```go
// chat.go - dangerousPatterns
34 patterns: rm -rf, kubectl delete, terraform destroy, etc.
// No docs list, no custom pattern addition
```

**Problem:**
- User ne zna šta se double-confirm
- No docs list
- No custom pattern addition

**Fix:**
- [ ] Add dangerous patterns docs: "Tool Safety"
- [ ] Add list in README: "Dangerous commands always require double confirmation: rm -rf, kubectl delete, ..."
- [ ] Add custom patterns in agent config: `dangerous_patterns: ["custom-dangerous-cmd"]`

**Effort:** Low
**Impact:** Medium

---

### 19. Error Codes - NEMA

**Current:**
```
All errors: fmt.Errorf("error: %v", err)
// No error codes, no reference page
```

**Problem:**
- No error codes
- No error reference page
- No recovery suggestions

**Fix:**
- [ ] Add error codes: `E001: file not found`
- [ ] Add error reference page: "Error Codes Reference"
- [ ] Add recovery suggestions per error code

**Effort:** Medium
**Impact:** Medium

---

### 20. Testing Helpers - UNDOCUMENTED

**Current:**
```go
// cmd/m/testing_helpers_test.go
Test helpers exist, but no docs
```

**Problem:**
- No docs for testing agents
- No example test file
- No mock provider explanation

**Fix:**
- [ ] Add testing docs: "Testing Agents"
- [ ] Add example test: `examples/testing/agent_test.go`
- [ ] Add mock provider explanation

**Effort:** Low
**Impact:** Low

---

## 📊 Priority Matrix

| Issue | Effort | Impact | Priority | Status |
|-------|--------|--------|----------|--------|
| 1. Binary name confusion | Medium | High | 🔴 HIGH | ❌ TODO |
| 2. First-run wizard | High | High | 🔴 HIGH | ❌ TODO |
| 3. TUI header | Medium | High | 🔴 HIGH | ❌ TODO |
| 4. Slash commands | Medium | High | 🔴 HIGH | ❌ TODO |
| 5. Tool confirmation | High | High | 🔴 HIGH | ❌ TODO |
| 6. Error handling | Medium | High | 🔴 HIGH | ❌ TODO |
| 7. Session IDs | Medium | Medium | 🟡 MEDIUM | ❌ TODO |
| 8. Model discovery | Medium | Medium | 🟡 MEDIUM | ❌ TODO |
| 9. Theme preview | Low | Medium | 🟡 MEDIUM | ❌ TODO |
| 10. Documentation | Medium | Medium | 🟡 MEDIUM | ❌ TODO |
| 11. Thinking phrases | Low | Low | 🟢 LOW | ❌ TODO |
| 12. Input blocking | Low | Low | 🟢 LOW | ❌ TODO |
| 13. Auto-save indicator | Low | Low | 🟢 LOW | ❌ TODO |
| 14. Cost disclaimer | Low | Low | 🟢 LOW | ❌ TODO |
| 15. Accessibility | Medium | Low | 🟢 LOW | ❌ TODO |
| 16. Agent schema docs | Medium | High | 🔵 DevEx | ❌ TODO |
| 17. MCP docs | Medium | High | 🔵 DevEx | ❌ TODO |
| 18. Dangerous patterns | Low | Medium | 🔵 DevEx | ❌ TODO |
| 19. Error codes | Medium | Medium | 🔵 DevEx | ❌ TODO |
| 20. Testing docs | Low | Low | 🔵 DevEx | ❌ TODO |

---

## 🗓️ Implementation Timeline

### Sprint 1 (v0.1.0) - HIGH PRIORITY
- [ ] Issue 1: Binary name confusion
- [ ] Issue 2: First-run wizard (Quick Start mode)
- [ ] Issue 3: TUI header (compact mode)
- [ ] Issue 4: Slash commands (grouping + aliases)
- [ ] Issue 6: Error handling (error codes)

### Sprint 2 (v0.2.0) - HIGH + MEDIUM
- [ ] Issue 5: Tool confirmation (popup + visibility)
- [ ] Issue 7: Session IDs (labels)
- [ ] Issue 8: Model discovery (progress)
- [ ] Issue 10: Documentation (Quick Start refactor)

### Sprint 3 (v0.3.0) - MEDIUM + DevEx
- [ ] Issue 9: Theme preview
- [ ] Issue 16: Agent schema docs
- [ ] Issue 17: MCP docs
- [ ] Issue 18: Dangerous patterns docs

### Sprint 4 (v0.4.0+) - LOW + Polish
- [ ] Issue 11-15: UX polish
- [ ] Issue 19-20: DevEx polish
- [ ] Accessibility improvements

---

## 📝 Notes

- **Effort** estimates are rough (Low = 1-2 days, Medium = 3-5 days, High = 1-2 weeks)
- **Impact** is subjective (High = blocks users, Medium = annoys users, Low = polish)
- **Priority** follows standard product management: fix blockers first, then annoyances, then polish
- **Timeline** assumes solo developer working evenings (like current pace)

---

*Last updated: v0.0.25*
*Analysed by: Steva Đubre (UX expert, apparently)*