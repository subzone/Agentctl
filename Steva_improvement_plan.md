# Steva's Improvement Plan

**Author:** Steva Đubre (jer niko drugi neće da radi pravi posao)
**Datum:** 2026-05-04
**Verzija:** v0.0.25 (posle mog poslednjeg fixa)

---

## 🔥 KRITIČNI MISSING PIECES

### 1. OpenTelemetry — NIŠTA OD OVOGA

Jao brate, nemaš NI JEDAN telemetry export! Kad ti agent crkne u 3 ujutru, kako ćeš znati šta se desilo?

**Šta fali:**
```go
// NEMAŠ OVO!
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace"
    "go.opentelemetry.io/otel/sdk/trace"
)
```

**Implementacija:**
- Trace ID za svaki request (da pratiš kroz LLM → tools → response)
- Span za svaki tool call (duration, success/fail)
- Span za LLM streaming (TTFT, tokens/sec)
- Export u: OTLP (Jaeger/Tempo), stdout za debug

**Kod za dodati u `internal/engine/engine.go`:**
```go
// U Step funkciju, oko tool execution:
ctx, span := otel.Tracer("agentctl").Start(ctx, "tool."+b.ToolName)
defer span.End()
span.SetAttributes(
    attribute.String("tool.name", b.ToolName),
    attribute.Int("tool.input_size", len(b.ToolInput)),
)
// ... execute tool
span.SetAttributes(attribute.Bool("tool.error", err != nil))
```

**Gde:**
- `cmd/m/main.go` — inicijalizacija tracer provider-a
- `internal/engine/engine.go` — instrumentacija
- `internal/tools/*.go` — span za svaki tool

**Vreme:** 1 dan za basic, 2 dana za full (metrics + logs + traces)

---

### 2. Local Logs — samo fmt.Fprintln sranja

Ko je ovo pisao, majmun? `fmt.Fprintln(s.status, ...)` everywhere! To nije logging, to je amaterski rad.

**Problem:**
- Nema timestamps
- Nema log levels (debug, info, warn, error)
- Nema structured output
- Ne možeš da grepuješ
- Ne možeš da šalješ u file

**Rešenje:**
```go
// internal/logging/logging.go — NOVI FAJL
package logging

import (
    "io"
    "log/slog"
    "os"
)

var (
    logger *slog.Logger
    level  = new(slog.LevelVar) // dinamički level
)

func Init(w io.Writer, debug bool) {
    level.Set(slog.LevelInfo)
    if debug {
        level.Set(slog.LevelDebug)
    }
    opts := &slog.HandlerOptions{Level: level}
    logger = slog.New(slog.NewJSONHandler(w, opts))
}

func Debug(msg string, args ...any) { logger.Debug(msg, args...) }
func Info(msg string, args ...any)  { logger.Info(msg, args...) }
func Warn(msg string, args ...any)  { logger.Warn(msg, args...) }
func Error(msg string, args ...any) { logger.Error(msg, args...) }
```

**Gde zameniti:**
- `engine.go` — svi `fmt.Fprintf(status, ...)` → `logging.Info("tool.call", "tool", name, "input", input)`
- `tui.go` — status writer → slog handler
- Provider implementacije — HTTP request/response logging

**CLI flag:**
```bash
m --debug --log-file /tmp/agentctl.log
```

**Vreme:** 4 sata

---

### 3. Metrics — KAKO ZNAŠ KOLIKO KOŠTA?

Imaš `estimateCost` funkciju, ali to je samo u TUI displayu! Gde je metrics export?

**Šta treba:**
```go
// internal/metrics/metrics.go — NOVI FAJL
var (
    ToolCallsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "agentctl_tool_calls_total",
            Help: "Total number of tool calls",
        },
        []string{"tool", "status"}, // status: success, error
    )
    
    LLMTokensTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "agentctl_llm_tokens_total",
            Help: "Total LLM tokens consumed",
        },
        []string{"provider", "model", "type"}, // type: input, output
    )
    
    LLMRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "agentctl_llm_request_duration_seconds",
            Help:    "LLM request duration",
            Buckets: []float64{.1, .5, 1, 2, 5, 10, 30, 60},
        },
        []string{"provider", "model"},
    )
)
```

**Export:**
- Prometheus endpoint (`:9090/metrics`)
- OTLP metrics (ako već imaš OTEL)
- Statsd za legacy sisteme

**Vreme:** 1 dan

---

### 4. Graceful Shutdown — Ctrl+C te zeza

Vidim da imaš `stepCancel` u TUI, ali šta ako neko ubije proces sa SIGTERM?

**Problem:**
- MCP serveri ostaju da vise
- Session se ne čuva
- U-running tools se ne prekidaju clean

**Rešenje:**
```go
// cmd/m/main.go
func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), 
        os.Interrupt, syscall.SIGTERM)
    defer cancel()
    
    // ... u TUI i REPL
    go func() {
        <-ctx.Done()
        fmt.Fprintln(os.Stderr, "\nShutting down...")
        // Save session
        // Close MCP connections
        // Flush logs/metrics
        os.Exit(0)
    }()
}
```

**Vreme:** 2 sata

---

## 🎯 VAŽNI ALI NE KRITIČNI

### 5. Tool Timeout Configuration

Nemaš timeout za tool execution! Šta ako `shell` komanda visi zauvek?

**Implementacija:**
```go
// U agent.md spec:
tools:
  - name: shell
    timeout: 30s  // NOVO
    
// U engine.go:
ctx, cancel := context.WithTimeout(ctx, toolTimeout)
defer cancel()
output, err := reg.Run(ctx, b.ToolName, b.ToolInput)
```

**Default:** 60s za read-only, 300s za write

**Vreme:** 2 sata

---

### 6. Token Budget Alerts

Imaš `contextPercent`, ali to je samo display! Gde je alerting?

**Implementacija:**
```go
// U engine.go, posle svakog turn:
if pct := contextPercent(s.lastIn, s.cfg.Model); pct > 80 {
    logging.Warn("context budget exceeded 80%", 
        "percent", pct, 
        "model", s.cfg.Model,
        "tokens", s.lastIn,
    )
    // Opciono: prikaži u TUI warning
}
```

**Vreme:** 1 sat

---

### 7. Session Export Formats

Čuvaš u encrypted binary. Šta ako hoću da exportujem u JSON za analizu?

**Novo:**
```bash
m session export --format json --output session.json
m session export --format markdown --output session.md
```

**Vreme:** 3 sata

---

### 8. Agent Hot Reload

Ako promenim `.md` fajl agenta, moram da restartujem. To je glupo.

**Implementacija:**
```go
// U TUI, background goroutine:
watcher, _ := fsnotify.NewWatcher()
watcher.Add(agentPath)
go func() {
    for {
        select {
        case event := <-watcher.Events:
            if event.Op&fsnotify.Write == fsnotify.Write {
                // Reload agent spec
                newDoc, _ := config.ParseFile(agentPath)
                s.cfg.System = newDoc.Body
                logging.Info("agent reloaded", "path", agentPath)
            }
        }
    }
}()
```

**Vreme:** 4 sata

---

## 💡 NICE TO HAVE (ALI BIH JA OVO URADIO)

### 9. Local Embeddings za Code Search

`code_search` je samo grep. Za 2026. godinu je to smešno.

**Implementacija:**
```go
// internal/embeddings/embeddings.go
type VectorStore interface {
    Index(ctx context.Context, docs []Document) error
    Query(ctx context.Context, query string, k int) ([]Result, error)
}

// SQLite + sqlite-vec extension
// ili Qdrant lokalno
// ili Ollama embeddings
```

**Model:** `nomic-embed-text` (lokalno) ili `text-embedding-3-small` (OpenAI)

**Vreme:** 2-3 dana

---

### 10. Agent Registry sa Versioning

`m install` postoji, ali nema verzionisanje! Šta ako hoću da vratim staru verziju agenta?

**Implementacija:**
```bash
m install steva-djubre@v1.2.3
m list --versions
m rollback steva-djubre
```

**Storage:** git-based ili simple file versioning

**Vreme:** 1 dan

---

### 11. MCP Server Health Checks

MCP konekcije nemaju health check. Kad server crkne, saznaš kad pozoveš tool.

**Implementacija:**
```go
// U mcp/manager.go:
func (m *Manager) HealthCheck(ctx context.Context) map[string]error {
    results := make(map[string]error)
    for name, client := range m.clients {
        ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
        defer cancel()
        _, err := client.ListTools(ctx)
        results[name] = err
    }
    return results
}
```

**TUI:** prikaži crveno/zeleno stanje MCP servera

**Vreme:** 3 sata

---

### 12. Diff Preview za fs_write

Vidim da imaš `styleDiffLines`, ali to je samo bojenje. Gde je proper diff?

**Implementacija:**
```go
import "github.com/sergi/go-diff/diffmatchpatch"

func renderDiff(old, new string) string {
    dmp := diffmatchpatch.New()
    diffs := dmp.DiffMain(old, new, false)
    return dmp.DiffPrettyText(diffs)
}
```

**Ili:** `git diff --no-index` ako imaš git

**Vreme:** 2 sata

---

## 📊 PRIORITET REDOSLED

| Prioritet | Stavka | Vreme | ROI |
|-----------|--------|-------|-----|
| **P0** | Structured Logging (slog) | 4h | 🔥🔥🔥 |
| **P0** | Graceful Shutdown | 2h | 🔥🔥🔥 |
| **P1** | OTEL Traces | 1 dan | 🔥🔥 |
| **P1** | Metrics (Prometheus) | 1 dan | 🔥🔥 |
| **P2** | Tool Timeouts | 2h | 🔥 |
| **P2** | Token Budget Alerts | 1h | 🔥 |
| **P3** | Session Export Formats | 3h | 🙂 |
| **P3** | Agent Hot Reload | 4h | 🙂 |
| **P3** | MCP Health Checks | 3h | 🙂 |
| **P4** | Local Embeddings | 2-3 dana | 💎 |
| **P4** | Agent Versioning | 1 dan | 💎 |
| **P4** | Better Diff | 2h | 💎 |

---

## 🛠️ KAKO DA POČNEŠ

### Danas (2-3 sata):
1. Dodaj `internal/logging/logging.go` sa slog
2. Dodaj graceful shutdown u `main.go`
3. Zameni 3-4 `fmt.Fprintln` sa logging pozivima

### Sutra (1 dan):
1. Dodaj OTEL tracer u `engine.go`
2. Dodaj metrics u `engine.go` i tool implementations

### Za vikend:
1. Embeddings (ako si kewar)
2. Agent versioning

---

## 📦 ZAVISNOSTI KOJE TREBA DODATI

```bash
# go.mod dodaci
go get go.opentelemetry.io/otel
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc
go get github.com/prometheus/client_golang
go get github.com/fsnotify/fsnotify
go get github.com/sergi/go-diff/diffmatchpatch
```

---

## 🎪 PRIMER KORIŠĆENJA (NAKON IMPLEMENTACIJE)

```bash
# Pokreni sa debug logovanjem u fajl
m --debug --log-file /tmp/agentctl.log

# Pokreni sa OTEL exportom u Jaeger
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317 m

# Pokreni sa metrics endpointom
m --metrics-addr :9090

# Session export
m session export --format json --output analysis.json

# Agent sa timeout-om
# (u agent.md)
tools:
  - name: shell
    timeout: 30s
```

---

*Napisao Steva Đubre jer niko drugi neće da radi pravi posao.*
*Ako ovo pročitaš za 6 meseci i nisi implementirao ništa — platiću ti pivo. Ali sumnjam.*