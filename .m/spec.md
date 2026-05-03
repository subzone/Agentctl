# UX Improvements for M Agent

## Problem Statement
User is frustrated with current UX:
1. Agent automatically continues for 15 iterations without asking permission - user must manually type "nastavi"
2. Agent asks for confirmation for every file edit, even when the change is obvious
3. Agent asks for confirmation for every tool call in REPL mode

## Goals
1. Implement smarter confirmation logic:
   - Auto-approve safe tools (fs_read, fs_list, git status, git log, git diff, test_run)
   - Auto-approve simple edits when change is obvious
   - Ask for confirmation only for destructive operations
2. Implement "Da nastavim?" prompt after each iteration or group of changes
3. Improve TUI experience to show progress and allow interruption

## Design

### Part 1: Smart Confirmation Logic
Modify `FSWriteTool` and `stdinToolConfirm` to:
- Auto-approve read-only tools
- Auto-approve simple edits (single line changes, small patches)
- Only ask for confirmation for:
  - Large file writes (> 100 lines)
  - Complex patches
  - Shell commands that could be destructive
  - Git operations that modify files

### Part 2: "Da nastavim?" Prompt
Add a new mechanism in `engine.go`:
- After each Step() completion, ask user "Da nastavim?" if more work is needed
- In TUI mode, show a prompt at bottom
- In REPL mode, print prompt and wait for y/n input
- Allow user to skip confirmation for small tasks

### Part 3: TUI Improvements
- Show progress indicator for multi-step operations
- Allow interrupting long operations
- Better visualization of what agent is doing

## Tasks

1. Modify `internal/tools/fswrite.go` to add smart confirmation logic
2. Modify `cmd/m/run.go` and `cmd/m/chat.go` to use smarter tool confirm
3. Add "Da nastavim?" prompt logic to `engine.go`
4. Update TUI to show continuation prompt
5. Test changes with existing agents