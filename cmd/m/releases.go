package main

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/subzone/m/internal/userconfig"
)

// release captures the user-visible highlights for one tagged version.
// Entries are kept newest-first so display logic can walk the slice in
// order and stop once it hits a version the user has already seen.
type release struct {
	Version    string
	Date       string
	Highlights []string
}

// releases is the source of truth for what gets shown in the changelog
// and "what's new" banner. When you cut a new tag, prepend an entry.
var releases = []release{
	{
		Version: "0.0.3",
		Date:    "2026-05-01",
		Highlights: []string{
			"New chat TUI: persistent header with the M banner on the left and a live system-stats table (CPU, RAM, GPU, Disk) on the right, scrolling chat viewport in the middle, input pinned at the bottom. Auto-falls-back to the line-oriented REPL when stdin/stdout/stderr aren't all terminals (so scripts and tests stay clean).",
			"Animated `thinking…` indicator while waiting for the model's first streamed token; disappears the moment text starts flowing.",
			"GPU stat shows `n/a` for now — Apple Silicon has no clean public API; Linux NVIDIA via `nvidia-smi` is planned.",
			"Stats refresh once per second via `gopsutil`.",
		},
	},
	{
		Version: "0.0.2",
		Date:    "2026-05-01",
		Highlights: []string{
			"Ollama daemon detection: polls localhost:11434 for up to 15 s and falls back to starting `ollama serve` as a background child when `brew services` doesn't bring it up.",
			"`brew services start ollama` failures now print their stderr instead of being silently swallowed.",
			"Qwen tag picker no longer hardcodes invalid `:Nb` sizes. Defaults to the always-valid `qwen3-coder` tag, with a `qwen2.5-coder:7b` fallback and a custom-tag option.",
		},
	},
	{
		Version: "0.0.1",
		Date:    "2026-05-01",
		Highlights: []string{
			"Default chat: type `m` to talk to an embedded agent — no arguments required.",
			"First-run setup wizard with four backends: Ollama+Qwen3-Coder, Anthropic, OpenAI, LiteLLM.",
			"Auto-installs Ollama (brew/curl) and libsecret (apt/dnf/pacman/apk) on demand, with explicit confirmation.",
			"API keys stored in the OS keychain (macOS Keychain, Linux libsecret) — never in plain config.",
			"GitHub Actions release pipeline: macOS .pkg installer and Linux .deb on every `vX.Y.Z` tag.",
			"`m init` re-runs the wizard; `M_MODEL=provider/model m` overrides the configured model for one session.",
			"`m changelog` prints the full version history.",
		},
	},
}

func newChangelogCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "changelog",
		Short: "Print the m release history",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			printReleases(cmd.OutOrStdout(), releases, "")
			return nil
		},
	}
}

// showReleaseNotesIfNeeded compares the current build's Version against
// the last version whose notes we showed the user, prints anything new,
// and persists the bump. Failures here are non-fatal — release notes are
// nice-to-have and we never want them blocking the user from chatting.
//
// Skips entirely when Version is "dev" (an unreleased build), so source
// builds don't pollute the state file.
func showReleaseNotesIfNeeded(out io.Writer, current string) {
	if current == "" || current == "dev" {
		return
	}
	st, err := userconfig.LoadState()
	if err != nil && !userconfig.IsNotExist(err) {
		// Treat read failures as "first install" to avoid swallowing notes;
		// SaveState below will overwrite the bad file on the way out.
		st = nil
	}

	var unseen []release
	header := ""
	if st == nil || st.LastSeenVersion == "" {
		// First install on this machine: show everything.
		unseen = releases
		header = fmt.Sprintf("Welcome to m %s. Highlights:\n", current)
	} else {
		for _, r := range releases {
			if versionLess(st.LastSeenVersion, r.Version) {
				unseen = append(unseen, r)
			}
		}
		if len(unseen) > 0 {
			header = fmt.Sprintf("What's new in m since %s:\n", st.LastSeenVersion)
		}
	}

	if len(unseen) == 0 {
		// Even with nothing to show, advance the marker so a future
		// release notices the bump correctly.
		_ = userconfig.SaveState(&userconfig.State{LastSeenVersion: current})
		return
	}

	printReleases(out, unseen, header)
	_ = userconfig.SaveState(&userconfig.State{LastSeenVersion: current})
}

func printReleases(out io.Writer, rs []release, header string) {
	if header != "" {
		fmt.Fprintln(out, header)
	}
	for i, r := range rs {
		if i > 0 {
			fmt.Fprintln(out)
		}
		fmt.Fprintf(out, "v%s — %s\n", r.Version, r.Date)
		for _, h := range r.Highlights {
			fmt.Fprintf(out, "  • %s\n", h)
		}
	}
	fmt.Fprintln(out)
}

// versionLess reports a < b for SemVer-shaped strings. Pre-release tags
// (-rc1, +meta) are stripped before comparison; we don't model them
// because the project tags releases as plain MAJOR.MINOR.PATCH.
func versionLess(a, b string) bool {
	pa, errA := parseVersion(a)
	pb, errB := parseVersion(b)
	if errA != nil || errB != nil {
		// Fall back to string compare so a malformed entry doesn't crash
		// the wizard — worst case the user sees notes once more than
		// they should.
		return a < b
	}
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return false
}

func parseVersion(v string) ([3]int, error) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 3 {
		return [3]int{}, errors.New("version must be MAJOR.MINOR.PATCH")
	}
	var out [3]int
	for i, p := range parts {
		// Trim pre-release / build metadata: "1.0.0-rc1" → "1.0.0".
		if cut := strings.IndexAny(p, "-+"); cut >= 0 {
			p = p[:cut]
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return [3]int{}, fmt.Errorf("part %d %q: %w", i, p, err)
		}
		out[i] = n
	}
	return out, nil
}
