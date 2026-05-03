package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/subzone/Agentctl/internal/userconfig"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Configure the default model backend (interactive wizard)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := runWizard(cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
			return err
		},
	}
}

// wiz bundles the IO triple and a single bufio.Scanner shared across all
// prompts. A fresh scanner per prompt would buffer extra stdin bytes
// internally and lose them on the next call, which broke piped/scripted
// input on the first iteration of this wizard.
type wiz struct {
	sc     *bufio.Scanner
	in     io.Reader
	out    io.Writer
	status io.Writer
}

func newWiz(in io.Reader, out, status io.Writer) *wiz {
	return &wiz{
		sc:     bufio.NewScanner(in),
		in:     in,
		out:    out,
		status: status,
	}
}

// runWizard walks the user through picking a provider, installs/pulls
// what's needed, and persists the result. Returns the config it just saved
// so callers (runDefaultChat on first run) can proceed without re-reading
// the file from disk.
func runWizard(in io.Reader, out, status io.Writer) (*userconfig.Config, error) {
	w := newWiz(in, out, status)

	fmt.Fprint(out, banner)
	fmt.Fprintln(out, "Welcome to m. Choose your model backend:")
	fmt.Fprintln(out, "  1) Ollama + Qwen3-Coder      — local, free, ~5–20 GB download")
	fmt.Fprintln(out, "  2) Anthropic (Claude)        — best quality, paid API")
	fmt.Fprintln(out, "  3) OpenAI (GPT)              — paid API")
	fmt.Fprintln(out, "  4) Google Gemini             — 1M context, paid API")
	fmt.Fprintln(out, "  5) Alibaba (Qwen Cloud)      — DashScope API")
	fmt.Fprintln(out, "  6) LiteLLM proxy             — self-hosted / custom endpoint")
	fmt.Fprintln(out)

	choice, err := w.prompt("Choice [1-6]: ")
	if err != nil {
		return nil, err
	}
	var cfg *userconfig.Config
	switch choice {
	case "1":
		cfg, err = setupOllama(w)
	case "2":
		cfg, err = setupAnthropic(w)
	case "3":
		cfg, err = setupHostedKey(w, userconfig.ProviderOpenAI, "OpenAI", "gpt-4o-mini", "sk-")
	case "4":
		cfg, err = setupGemini(w)
	case "5":
		cfg, err = setupAlibaba(w)
	case "6":
		cfg, err = setupLiteLLM(w)
	default:
		return nil, fmt.Errorf("invalid choice %q (expected 1-6)", choice)
	}
	if err != nil {
		return nil, err
	}
	if err := userconfig.Save(cfg); err != nil {
		return nil, err
	}
	p, _ := userconfig.Path()
	fmt.Fprintf(out, "\nSaved %s\n", p)
	return cfg, nil
}

func (w *wiz) prompt(label string) (string, error) {
	if label != "" {
		fmt.Fprint(w.out, label)
	}
	if !w.sc.Scan() {
		if err := w.sc.Err(); err != nil {
			return "", err
		}
		return "", errors.New("no input")
	}
	return strings.TrimSpace(w.sc.Text()), nil
}

// promptSecret reads without echo when stdin is a tty; for non-tty input
// (tests, scripts, pipes) it reads through the shared scanner so input
// stays in sync with sibling prompts.
func (w *wiz) promptSecret(label string) (string, error) {
	fmt.Fprint(w.out, label)
	if f, ok := w.in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(w.out)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
	return w.prompt("")
}

func (w *wiz) confirm(label string, defaultYes bool) (bool, error) {
	suffix := " [y/N]: "
	if defaultYes {
		suffix = " [Y/n]: "
	}
	s, err := w.prompt(label + suffix)
	if err != nil {
		return false, err
	}
	s = strings.ToLower(s)
	if s == "" {
		return defaultYes, nil
	}
	return s == "y" || s == "yes", nil
}

// setupOllama: detect/install ollama, ensure daemon is reachable, ask
// user to pick a Qwen size, and pull it. We shell out to `ollama pull`
// rather than reimplementing /api/pull so the user sees the official
// progress UI.
func setupOllama(w *wiz) (*userconfig.Config, error) {
	if _, err := exec.LookPath("ollama"); err != nil {
		if err := installOllama(w); err != nil {
			return nil, err
		}
	} else {
		fmt.Fprintln(w.out, "✓ ollama found on PATH")
	}
	if err := ensureOllamaRunning(w.out, w.status); err != nil {
		return nil, err
	}

	fmt.Fprintln(w.out)
	fmt.Fprintln(w.out, "Ollama tag for the coder model:")
	fmt.Fprintln(w.out, "  1) qwen3-coder       — latest tag (recommended, always exists)")
	fmt.Fprintln(w.out, "  2) qwen2.5-coder:7b  — small, well-tested fallback (~5 GB)")
	fmt.Fprintln(w.out, "  3) custom            — pick a specific tag from https://ollama.com/library/qwen3-coder")

	tagChoice, err := w.prompt("Choice [1-3, default 1]: ")
	if err != nil {
		return nil, err
	}
	var tag string
	switch tagChoice {
	case "1", "":
		tag = "qwen3-coder"
	case "2":
		tag = "qwen2.5-coder:7b"
	case "3":
		tag, err = w.prompt("Ollama tag: ")
		if err != nil {
			return nil, err
		}
		if tag == "" {
			return nil, errors.New("no tag provided")
		}
	default:
		return nil, fmt.Errorf("invalid choice %q", tagChoice)
	}

	fmt.Fprintf(w.out, "\nPulling %s — this may take a while.\n", tag)
	pull := exec.Command("ollama", "pull", tag)
	pull.Stdout = w.out
	pull.Stderr = w.status
	if err := pull.Run(); err != nil {
		return nil, fmt.Errorf("ollama pull %s: %w", tag, err)
	}

	return &userconfig.Config{
		Provider: userconfig.ProviderOllama,
		Model:    tag,
	}, nil
}

func installOllama(w *wiz) error {
	fmt.Fprintln(w.out, "Ollama is not installed.")
	var cmdline string
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("brew"); err != nil {
			return errors.New("homebrew not found; install ollama from https://ollama.com/download then re-run `m init`")
		}
		cmdline = "brew install ollama"
	case "linux":
		cmdline = "curl -fsSL https://ollama.com/install.sh | sh"
	default:
		return fmt.Errorf("auto-install not supported on %s; install ollama from https://ollama.com/download", runtime.GOOS)
	}
	fmt.Fprintf(w.out, "About to run: %s\n", cmdline)
	ok, err := w.confirm("Proceed?", true)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("aborted: install ollama manually then re-run `m init`")
	}
	cmd := exec.Command("sh", "-c", cmdline)
	cmd.Stdout = w.out
	cmd.Stderr = w.status
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ollama install failed: %w", err)
	}
	return nil
}

// ensureOllamaRunning probes localhost:11434, then tries the
// persistent-daemon path (brew services on macOS), then falls back to
// launching `ollama serve` as a background child for the duration of the
// `m` process. The earlier 2-second sleep + single brew-services attempt
// was racy on first install and silently swallowed brew errors; this
// version polls and surfaces failures so users can see what went wrong.
func ensureOllamaRunning(out, status io.Writer) error {
	if ollamaReachable() {
		fmt.Fprintln(out, "✓ ollama daemon reachable at http://localhost:11434")
		return nil
	}

	// macOS: try the persistent-service path first — it survives reboots
	// and is what most desktop users want long-term.
	if runtime.GOOS == "darwin" {
		var stderr bytes.Buffer
		brew := exec.Command("brew", "services", "start", "ollama")
		brew.Stderr = &stderr
		if err := brew.Run(); err == nil {
			if waitForOllama(10 * time.Second) {
				fmt.Fprintln(out, "✓ started ollama via brew services")
				return nil
			}
		} else if msg := strings.TrimSpace(stderr.String()); msg != "" {
			fmt.Fprintf(status, "brew services start ollama: %s\n", msg)
		}
	}

	// Fall back to launching the daemon ourselves. The child outlives this
	// wizard step but typically dies with the m process; the closing
	// message tells the user how to make it persistent afterward.
	fmt.Fprintln(out, "Starting ollama serve in the background for this session…")
	serve := exec.Command("ollama", "serve")
	serve.Stdout = io.Discard
	serve.Stderr = io.Discard
	if err := serve.Start(); err != nil {
		return fmt.Errorf("failed to start ollama serve: %w. Start it manually with `ollama serve` in another terminal and re-run `m init`", err)
	}
	if waitForOllama(15 * time.Second) {
		fmt.Fprintln(out, "✓ ollama daemon reachable at http://localhost:11434")
		hint := "  (this instance ends with `m`. To keep it running between sessions, run `brew services start ollama`.)"
		if runtime.GOOS == "linux" {
			hint = "  (this instance ends with `m`. The Linux installer normally registers a systemd unit; if it didn't, see https://ollama.com/download for setup.)"
		}
		fmt.Fprintln(out, hint)
		return nil
	}
	return errors.New("ollama daemon did not become reachable within 15s — start it manually with `ollama serve` and re-run `m init`")
}

// waitForOllama polls reachability with exponential backoff until the
// total duration elapses. Returns true the moment a probe succeeds.
func waitForOllama(total time.Duration) bool {
	deadline := time.Now().Add(total)
	interval := 500 * time.Millisecond
	for time.Now().Before(deadline) {
		if ollamaReachable() {
			return true
		}
		time.Sleep(interval)
		if interval < 2*time.Second {
			interval *= 2
		}
	}
	return false
}

// saveKeyWithRetry wraps userconfig.SaveAPIKey with a one-shot install
// recovery path: on Linux, if the call fails because secret-tool is
// missing, offer to install libsecret via the system package manager and
// retry once. On macOS the keychain is built-in so this just delegates.
func saveKeyWithRetry(w *wiz, provider userconfig.Provider, key string) error {
	err := userconfig.SaveAPIKey(provider, key)
	if err == nil {
		return nil
	}
	if !errors.Is(err, userconfig.ErrSecretToolMissing) {
		return err
	}
	if err := installSecretTool(w); err != nil {
		return err
	}
	return userconfig.SaveAPIKey(provider, key)
}

// installSecretTool is the Linux counterpart to installOllama: detect the
// distro's package manager, ask the user to confirm the exact command,
// then run it under sudo. The package is named differently across
// distros so we map per-package-manager. Skipped on non-Linux (the
// caller never invokes us there in practice).
func installSecretTool(w *wiz) error {
	if runtime.GOOS != "linux" {
		return errors.New("secret-tool auto-install is only supported on Linux")
	}
	type pm struct {
		bin     string
		pkg     string
		install []string
	}
	candidates := []pm{
		{bin: "apt-get", pkg: "libsecret-tools", install: []string{"sudo", "apt-get", "install", "-y", "libsecret-tools"}},
		{bin: "dnf", pkg: "libsecret", install: []string{"sudo", "dnf", "install", "-y", "libsecret"}},
		{bin: "pacman", pkg: "libsecret", install: []string{"sudo", "pacman", "-S", "--noconfirm", "libsecret"}},
		{bin: "apk", pkg: "libsecret", install: []string{"sudo", "apk", "add", "libsecret"}},
	}
	var chosen *pm
	for i := range candidates {
		if _, err := exec.LookPath(candidates[i].bin); err == nil {
			chosen = &candidates[i]
			break
		}
	}
	if chosen == nil {
		return errors.New("no supported package manager found (looked for apt-get, dnf, pacman, apk). Install libsecret-tools manually then re-run `m init`")
	}
	fmt.Fprintln(w.out, "secret-tool (libsecret) is required to store API keys in your keyring.")
	fmt.Fprintf(w.out, "About to run: %s\n", strings.Join(chosen.install, " "))
	ok, err := w.confirm("Proceed (you may be prompted for your sudo password)?", true)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("aborted: install libsecret-tools/libsecret manually then re-run `m init`")
	}
	cmd := exec.Command(chosen.install[0], chosen.install[1:]...)
	cmd.Stdin = w.in // sudo needs to read the password
	cmd.Stdout = w.out
	cmd.Stderr = w.status
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("install %s failed: %w", chosen.pkg, err)
	}
	return nil
}

func ollamaReachable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:11434/api/tags", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 500
}

// setupHostedKey covers Anthropic and OpenAI: collect a key, save it to
// the OS keychain, and produce a config pointing at a sensible default
// model. expectedPrefix is a low-effort sanity check on the pasted key
// (warning only — we don't reject in case formats change).
func setupHostedKey(w *wiz, provider userconfig.Provider, label, defaultModel, expectedPrefix string) (*userconfig.Config, error) {
	fmt.Fprintf(w.out, "\n%s API key:\n", label)
	key, err := w.promptSecret("Paste key (input hidden): ")
	if err != nil {
		return nil, err
	}
	if key == "" {
		return nil, errors.New("empty api key")
	}
	if expectedPrefix != "" && !strings.HasPrefix(key, expectedPrefix) {
		fmt.Fprintf(w.out, "  warning: key does not start with %q — proceeding anyway\n", expectedPrefix)
	}
	if err := saveKeyWithRetry(w, provider, key); err != nil {
		return nil, fmt.Errorf("save key to keychain: %w", err)
	}
	fmt.Fprintln(w.out, "✓ key saved to OS keychain")
	return &userconfig.Config{
		Provider: provider,
		Model:    defaultModel,
	}, nil
}

func setupAnthropic(w *wiz) (*userconfig.Config, error) {
	fmt.Fprintln(w.out, "\nAnthropic API key:")
	key, err := w.promptSecret("Paste key (input hidden): ")
	if err != nil {
		return nil, err
	}
	if key == "" {
		return nil, errors.New("empty api key")
	}
	if !strings.HasPrefix(key, "sk-ant-") {
		fmt.Fprintln(w.out, "  warning: key does not start with \"sk-ant-\" \u2014 proceeding anyway")
	}

	if err := saveKeyWithRetry(w, userconfig.ProviderAnthropic, key); err != nil {
		return nil, fmt.Errorf("save key to keychain: %w", err)
	}
	fmt.Fprintln(w.out, "\u2713 key saved to OS keychain")

	os.Setenv("ANTHROPIC_API_KEY", key)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	models, scanErr := discoverAnthropic(ctx)

	var model string
	if scanErr == nil && len(models) > 0 {
		fmt.Fprintf(w.out, "\nFound %d models on Anthropic:\n", len(models))
		for i, m := range models {
			fmt.Fprintf(w.out, "  %d) %s\n", i+1, m)
		}
		fmt.Fprintln(w.out)
		pick, err := w.prompt(fmt.Sprintf("Choice [1-%d, or type a model id]: ", len(models)))
		if err != nil {
			return nil, err
		}
		var idx int
		if _, serr := fmt.Sscanf(pick, "%d", &idx); serr == nil && idx >= 1 && idx <= len(models) {
			model = models[idx-1]
		} else {
			model = strings.TrimSpace(pick)
		}
	} else {
		if scanErr != nil {
			fmt.Fprintf(w.out, "\nCould not scan models: %v\n", scanErr)
		}
		fmt.Fprintln(w.out, "\nSelect a model manually:")
		fmt.Fprintln(w.out, "  1) claude-sonnet-4-6    \u2014 best balance (recommended)")
		fmt.Fprintln(w.out, "  2) claude-haiku-4-5-20251001 \u2014 fast, cheap (Haiku 4.5)")
		fmt.Fprintln(w.out, "  3) claude-opus-4        \u2014 highest quality")
		fmt.Fprintln(w.out, "  4) custom               \u2014 enter a model id")
		choice, err := w.prompt("Choice [1-4, default 1]: ")
		if err != nil {
			return nil, err
		}
		switch choice {
		case "1", "":
			model = "claude-sonnet-4-6"
		case "2":
			model = "claude-haiku-4-5-20251001"
		case "3":
			model = "claude-opus-4"
		case "4":
			model, err = w.prompt("Model id: ")
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("invalid choice %q", choice)
		}
	}

	if model == "" {
		return nil, errors.New("no model selected")
	}

	fmt.Fprintf(w.out, "\u2713 using model %s\n", model)
	return &userconfig.Config{
		Provider: userconfig.ProviderAnthropic,
		Model:    model,
	}, nil
}

func setupGemini(w *wiz) (*userconfig.Config, error) {
	fmt.Fprintln(w.out, "\nGoogle Gemini API key:")
	key, err := w.promptSecret("Paste key (input hidden): ")
	if err != nil {
		return nil, err
	}
	if key == "" {
		return nil, errors.New("empty api key")
	}

	if err := saveKeyWithRetry(w, userconfig.ProviderGemini, key); err != nil {
		return nil, fmt.Errorf("save key to keychain: %w", err)
	}
	fmt.Fprintln(w.out, "\u2713 key saved to OS keychain")

	os.Setenv("GEMINI_API_KEY", key)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	models, scanErr := discoverGemini(ctx)

	var model string
	if scanErr == nil && len(models) > 0 {
		fmt.Fprintf(w.out, "\nFound %d Gemini models:\n", len(models))
		for i, m := range models {
			fmt.Fprintf(w.out, "  %d) %s\n", i+1, m)
		}
		fmt.Fprintln(w.out)
		pick, err := w.prompt(fmt.Sprintf("Choice [1-%d, or type a model id]: ", len(models)))
		if err != nil {
			return nil, err
		}
		var idx int
		if _, serr := fmt.Sscanf(pick, "%d", &idx); serr == nil && idx >= 1 && idx <= len(models) {
			model = models[idx-1]
		} else {
			model = strings.TrimSpace(pick)
		}
	} else {
		if scanErr != nil {
			fmt.Fprintf(w.out, "\nCould not scan models: %v\n", scanErr)
		}
		fmt.Fprintln(w.out, "\nSelect a model manually:")
		fmt.Fprintln(w.out, "  1) gemini-2.5-flash    \u2014 fast, cheap, 1M context (recommended)")
		fmt.Fprintln(w.out, "  2) gemini-2.5-pro      \u2014 highest quality, 1M context")
		fmt.Fprintln(w.out, "  3) gemini-2.0-flash    \u2014 previous gen, fast")
		fmt.Fprintln(w.out, "  4) custom              \u2014 enter a model id")
		choice, err := w.prompt("Choice [1-4, default 1]: ")
		if err != nil {
			return nil, err
		}
		switch choice {
		case "1", "":
			model = "gemini-2.5-flash"
		case "2":
			model = "gemini-2.5-pro"
		case "3":
			model = "gemini-2.0-flash"
		case "4":
			model, err = w.prompt("Model id: ")
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("invalid choice %q", choice)
		}
	}

	if model == "" {
		return nil, errors.New("no model selected")
	}

	fmt.Fprintf(w.out, "\u2713 using model %s\n", model)
	return &userconfig.Config{
		Provider: userconfig.ProviderGemini,
		Model:    model,
	}, nil
}

func setupAlibaba(w *wiz) (*userconfig.Config, error) {
	fmt.Fprintln(w.out, "\nAlibaba Cloud (DashScope) API key:")
	key, err := w.promptSecret("Paste key (input hidden): ")
	if err != nil {
		return nil, err
	}
	if key == "" {
		return nil, errors.New("empty api key")
	}

	if err := saveKeyWithRetry(w, userconfig.ProviderAlibaba, key); err != nil {
		return nil, fmt.Errorf("save key to keychain: %w", err)
	}
	fmt.Fprintln(w.out, "\u2713 key saved to OS keychain")

	baseURL, err := w.prompt("Base URL (enter for default, or paste token-plan URL): ")
	if err != nil {
		return nil, err
	}
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = "https://dashscope-intl.aliyuncs.com/compatible-mode"
	}
	// Strip trailing /v1 if user pasted the full OpenAI-compat URL.
	baseURL = strings.TrimSuffix(baseURL, "/v1")
	baseURL = strings.TrimSuffix(baseURL, "/")

	os.Setenv("DASHSCOPE_API_KEY", key)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	models, scanErr := discoverDashScope(ctx, key, baseURL)

	var model string
	if scanErr == nil && len(models) > 0 {
		fmt.Fprintf(w.out, "\nFound %d models on DashScope:\n", len(models))
		for i, m := range models {
			fmt.Fprintf(w.out, "  %d) %s\n", i+1, m)
		}
		fmt.Fprintln(w.out)
		pick, err := w.prompt(fmt.Sprintf("Choice [1-%d, or type a model id]: ", len(models)))
		if err != nil {
			return nil, err
		}
		var idx int
		if _, serr := fmt.Sscanf(pick, "%d", &idx); serr == nil && idx >= 1 && idx <= len(models) {
			model = models[idx-1]
		} else {
			model = strings.TrimSpace(pick)
		}
	} else {
		if scanErr != nil {
			fmt.Fprintf(w.out, "\nCould not scan models: %v\n", scanErr)
		}
		fmt.Fprintln(w.out, "\nSelect a model manually:")
		fmt.Fprintln(w.out, "  1) qwen-plus           \u2014 good balance (recommended)")
		fmt.Fprintln(w.out, "  2) qwen-turbo          \u2014 fastest, cheapest")
		fmt.Fprintln(w.out, "  3) qwen-max            \u2014 highest quality")
		fmt.Fprintln(w.out, "  4) custom              \u2014 enter a model id")
		choice, err := w.prompt("Choice [1-4, default 1]: ")
		if err != nil {
			return nil, err
		}
		switch choice {
		case "1", "":
			model = "qwen-plus"
		case "2":
			model = "qwen-turbo"
		case "3":
			model = "qwen-max"
		case "4":
			model, err = w.prompt("Model id: ")
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("invalid choice %q", choice)
		}
	}

	if model == "" {
		return nil, errors.New("no model selected")
	}

	fmt.Fprintf(w.out, "\u2713 using model %s\n", model)
	return &userconfig.Config{
		Provider: userconfig.ProviderAlibaba,
		Model:    model,
		BaseURL:  baseURL,
	}, nil
}

func setupLiteLLM(w *wiz) (*userconfig.Config, error) {
	baseURL, err := w.prompt("LiteLLM base URL (e.g. http://localhost:4000): ")
	if err != nil {
		return nil, err
	}
	if baseURL == "" {
		return nil, errors.New("empty base url")
	}
	model, err := w.prompt("Model id (whatever your LiteLLM router exposes): ")
	if err != nil {
		return nil, err
	}
	if model == "" {
		return nil, errors.New("empty model id")
	}
	key, err := w.promptSecret("API key (input hidden, leave blank if proxy is unauthenticated): ")
	if err != nil {
		return nil, err
	}
	// Persist whatever was entered (or a placeholder) so the runtime can
	// always populate LITELLM_API_KEY: the openai client requires a non-
	// empty Bearer header even when the proxy ignores it.
	if key == "" {
		key = "no-auth"
	}
	if err := saveKeyWithRetry(w, userconfig.ProviderLiteLLM, key); err != nil {
		return nil, fmt.Errorf("save key to keychain: %w", err)
	}
	fmt.Fprintln(w.out, "✓ key saved to OS keychain")
	return &userconfig.Config{
		Provider: userconfig.ProviderLiteLLM,
		Model:    model,
		BaseURL:  baseURL,
	}, nil
}
