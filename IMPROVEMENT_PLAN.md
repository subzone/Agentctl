# AgentCTL Improvement Plan

**Version:** 2026.01-alpha  
**Date:** 2026-01-09  
**Author:** Steva Đubre (jer niko drugi neće da radi sranja oko kodiranja)

---

## Trenutno Stanje (Summary)

| Metrika | Vrednost | Komentar |
|---------|----------|----------|
| Verzija | v0.0.23 | Alpha, još uvek ne sme u produkciju |
| LOC | ~10,600 | Solidno za jedan CLI alat |
| Test Coverage | 15.9% - 100% | cmd/m je katastrofa (15.9%), internal/llm je super (93.8%) |
| LLM Provideri | 6 | Anthropic, OpenAI, Ollama, Gemini, Alibaba, LiteLLM |
| Built-in Tools | 8 | shell, fs_read, fs_write, fs_list, git, test_run, web_fetch, delegate |
| MCP Transport | stdio only | HTTP/SSE nije implementiran |
| TUI | 9 tema | matrix, nord, dracula, gruvbox, tokyonight, catppuccin, solarized, default, minimal |

### Šta radi dobro
- Arhitektura je čista - hexagonal, ports/adapters
- Provideri su stdlib-only, bez SDK zavisnosti
- Session persistence sa AES-256-GCM enkripcijom
- Fallback modeli na 429 rate limit
- Token-based context compaction

### Šta je sranje
- **cmd/m test coverage 15.9%** - Ko je pisao ovo? Ah da, ja. Sramota.
- **MCP HTTP/SSE transport** - Nema ga. Mnogi MCP serveri koriste HTTP.
- **/trust komanda** - Nema. Svaki put kad radiš autonomous session moraš da klikćeš y/n.
- **Debug/trace mode** - Nema. Kad LLM halucinira, ne vidiš šta šalješ.
- **Codebase RAG** - Nema. Agent vidi samo šta eksplicitno pročita.

---

## Improvement Plan po Prioritetu

### P1: Kritično (Mora odmah)

#### 1. MCP HTTP/SSE Transport
**Problem:** Mnogi MCP serveri (npr. `mcp-server-datadog`, `mcp-server-slack`) koriste HTTP/SSE transport. AgentCTL podržava samo stdio.

**Resenje:**
```go
// internal/mcp/transport.go
type Transport interface {
    Connect(ctx context.Context) error
    Send(ctx context.Context, req Request) (Response, error)
    Close() error
}

type StdioTransport struct { /* postojeci kod */ }
type HTTPTransport struct { client *http.Client; url string }
type SSETransport struct { client *http.Client; url string }
```

**Fajlovi za izmenu:**
- `internal/mcp/client.go` - dodaj transport selection
- `internal/mcp/transport.go` - novi fajl
- `internal/config/schema.go` - dodaj `transport: http/sse` u MCPServerSpec

**Test coverage:** Cilj 80%+ za novi kod

**Vreme:** 2-3 dana

---

#### 2. `/trust` Command za Auto-Approval
**Problem:** Dugotrajne autonomous sesije su nemoguće jer moraš da klikćeš y/n za svaki shell/fs_write.

**Resenje:**
```go
// cmd/m/chat.go
type chatState struct {
    session    *engine.Session
    autoApprove bool  // <-- dodaj ovo
}

// U slash commands:
case "/trust":
    cs.autoApprove = !cs.autoApprove
    fmt.Fprintln(w, "Auto-approve:", cs.autoApprove)
```

**Fajlovi za izmenu:**
- `cmd/m/chat.go` - dodaj `/trust` toggle
- `cmd/m/tui.go` - dodaj indikator u status bar
- `internal/engine/engine.go` - ToolConfirm callback već postoji

**Test coverage:** Dodaj test za auto-approve flow

**Vreme:** 4-6 sati

---

#### 3. Debug/Trace Mode (`/debug`)
**Problem:** Kad LLM halucinira tool call ili šalje loš JSON, ne vidiš šta tačno šalješ primaš.

**Resenje:**
```go
// internal/engine/engine.go
type Config struct {
    // ... postojeća polja
    TraceWriter io.Writer  // <-- dodaj ovo
}

// U Stream loop-u:
if cfg.TraceWriter != nil {
    json.NewEncoder(cfg.TraceWriter).Encode(req)
}
```

**Fajlovi za izmenu:**
- `internal/engine/engine.go` - dodaj TraceWriter
- `cmd/m/chat.go` - dodaj `/debug` toggle
- `cmd/m/tui.go` - prikaži trace output

**Test coverage:** Dodaj test za trace output

**Vreme:** 4-6 sati

---

### P2: Važno (U sledećih 2 nedelje)

#### 4. Test Coverage za cmd/m
**Problem:** 15.9% coverage je ispod svakog nivoa. Ako refaktorišeš nešto, nećeš znati da li si slomio.

**Resenje:**
- Dodaj integration tests za CLI komande
- Koristi `exec.Command()` da pokreneš binary i testiraš output
- Cilj: 60%+ coverage za cmd/m

**Fajlovi za dodavanje:**
- `cmd/m/cli_test.go` - integration tests
- `cmd/m/chat_integration_test.go` - REPL tests

**Vreme:** 1-2 dana

---

#### 5. Structured Logging sa slog
**Problem:** Nema proper logging-a. Kad nešto crkne, ne znaš šta se desilo.

**Resenje:**
```go
// internal/log/log.go
import "log/slog"

func Init(level string) {
    var lvl slog.Level
    switch level {
    case "debug": lvl = slog.LevelDebug
    case "info":  lvl = slog.LevelInfo
    default:      lvl = slog.LevelWarn
    }
    slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
        Level: lvl,
    })))
}
```

**Fajlovi za izmenu:**
- `internal/log/log.go` - novi fajl
- `cmd/m/main.go` - init logging
- Svi ostali fajlovi - zameni fmt.Println sa slog

**Vreme:** 1 dan

---

#### 6. Graceful Shutdown
**Problem:** Ako prekineš sesiju sa Ctrl+C, ne čuva se state. MCP serveri ostaju da vise.

**Resenje:**
```go
// cmd/m/chat.go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
go func() {
    <-sigCh
    fmt.Fprintln(w, "\nSaving session...")
    cs.Save()
    cancel()
}()
```

**Fajlovi za izmenu:**
- `cmd/m/chat.go` - dodaj signal handling
- `cmd/m/tui.go` - dodaj graceful exit

**Vreme:** 4 sata

---

### P3: Nice to Have (Kad stigne vreme)

#### 7. Code Search Tool (`code_search`)
**Problem:** Nema RAG, nema embeddings. Agent mora ručno da čita fajlove.

**Resenje:** Wrapper oko `ripgrep` sa fuzzy matching
```go
// internal/tools/codesearch.go
type CodeSearchTool struct{}

func (t *CodeSearchTool) Run(ctx context.Context, query string) ([]Match, error) {
    // ripgrep -i --json query
    // parsiraj output
}
```

**Vreme:** 1 dan

---

#### 8. Agent Registry (`~/.agent/agents/`)
**Problem:** Moras da znaš putanju do agent.md fajla. Nema globalni registry.

**Resenje:**
```bash
m install ./my-agent.md    # kopira u ~/.agent/agents/
m list                     # prikazuje sve instalirane
m run my-agent "task"      # pokreće po imenu
```

**Fajlovi za dodavanje:**
- `cmd/m/install.go` - nova komanda
- `internal/registry/registry.go` - registry logic

**Vreme:** 1 dan

---

#### 9. Benchmark Suite
**Problem:** Ne znamo da li je brzo ili sporo. Nema baseline.

**Resenje:**
```go
// internal/engine/engine_bench_test.go
func BenchmarkStep(b *testing.B) {
    // benchmark single turn
}
```

**Vreme:** 4 sata

---

#### 10. Plugin System (WASM)
**Problem:** Samo built-in tools + MCP. Ako hoćeš custom tool, moraš da pišeš MCP server.

**Resenje:** WASM-based plugins
```yaml
---
name: my-plugin
type: tool
runtime: wasm
wasm: ./my-plugin.wasm
---
```

**Vreme:** 3-5 dana (veci posao)

---

## Verzionisanje

Format: `v0.0.24-YYYY.MM.PATCH`

- `v0.0.24` - Sledeći release sa P1 itemima
- `v0.1.0` - Prvi beta (nakon P1 + P2)
- `v1.0.0` - Prvi stable (nakon P1 + P2 + P3 items 1-8)

---

## Roadmap

| Datum | Verzija | Šta |
|-------|---------|-----|
| 2026-01-15 | v0.0.24 | MCP HTTP/SSE, /trust, /debug |
| 2026-01-30 | v0.0.25 | Test coverage 60%+, structured logging |
| 2026-02-15 | v0.1.0-beta | Graceful shutdown, code search |
| 2026-03-01 | v1.0.0-rc1 | Agent registry, benchmarks |
| 2026-03-15 | v1.0.0 | Stable release |

---

## Known Gaps (iz README)

Ovo su stvari koje NISAM dodao u plan jer su vec dokumentovane:

1. **Codebase RAG** - Preporuka: koristi MCP sa Qdrant/Chroma
2. **No /trust** - Dodato u P1
3. **No team features** - Out of scope za v1.0
4. **No IDE integration** - Namerno, CLI tool

---

## Zakljucak

Projekat je u dobrom stanju za alpha, ali ima rupa koje moraju da se pokriju pre beta. Prioritet je MCP HTTP/SSE i /trust komanda - bez toga je alat neupotrebljiv za prave autonomous sesije.

Ako ti treba pomoc sa implementacijom, javi. Ali molim te, ne pitaj glupa pitanja.

---

*Generated by Steva Đubre, 2026-01-09*