# AgentCTL Improvement Plan

**Version:** v0.0.25
**Last updated:** 2026-05-04
**Author:** Steva Đubre (jer niko drugi neće)

---

## Trenutno Stanje

| Metrika | Vrednost |
|---------|----------|
| Verzija | v0.0.25 |
| LOC | ~11,000 prod + ~5,500 test |
| Provideri | 6 (Anthropic, OpenAI, Ollama, Gemini, Alibaba, LiteLLM) |
| Built-in Tools | 9 (shell, fs_read, fs_write, fs_list, git, test_run, web_fetch, code_search, delegate) |
| MCP Transport | stdio + HTTP + SSE |
| Teme | 9 |
| Primer agenata | 32 |

---

## ✅ Završeno (od prethodnog plana)

| # | Stavka | Verzija |
|---|--------|---------|
| P1.1 | MCP HTTP/SSE Transport | v0.0.25 |
| P1.2 | `/trust` komanda (auto-approve + dangerous cmd protection) | v0.0.24 |
| P1.3 | `/debug` trace mode | v0.0.22 |
| P3.7 | `code_search` tool (grep + symbol index, 9 jezika) | v0.0.24 |
| P3.8 | Agent registry (`m install`, `m run <name>`) | v0.0.25 |
| — | Fallback modeli (auto-switch na 429) | v0.0.23 |
| — | Per-agent thinking phrases | v0.0.23 |
| — | Markdown rendering u TUI (bold, code, headers) | v0.0.23 |
| — | Session history rotation (last 10 backups) | v0.0.24 |
| — | Dangerous command double-confirmation (34 patterns) | v0.0.24 |
| — | web_fetch tool | v0.0.18 |
| — | Session persistence (AES-256-GCM) | v0.0.18 |
| — | Token-based context compaction | v0.0.18 |
| — | Homebrew tap + auto-update | v0.0.19 |

---

## ❌ Preostalo — po prioritetu

### P1: Kritično

#### 1. Test Coverage za cmd/m
**Problem:** ~16% coverage. Refaktoring bez testova je ruski rulet.

**Cilj:** 60%+ za cmd/m

**Pristup:**
- Integration tests sa `exec.Command()` za CLI komande
- Unit tests za slash command handling
- Mock provider za chat loop testove

**Vreme:** 1-2 dana

---

#### 2. Structured Logging (slog)
**Problem:** `fmt.Fprintln` everywhere. Kad nešto crkne u produkciji, nemaš pojma šta se desilo.

**Pristup:**
- `log/slog` sa JSON handler
- `--verbose` / `--debug` CLI flagovi
- Zameni sve `fmt.Fprintf(status, ...)` sa slog pozivima

**Vreme:** 1 dan

---

#### 3. Graceful Shutdown
**Problem:** Ctrl+C ne čuva session state. MCP serveri mogu da ostanu da vise.

**Pristup:**
- Signal handler koji čuva sesiju pre izlaska
- MCP manager cleanup na context cancel
- Autosave na SIGINT/SIGTERM

**Vreme:** 4 sata

---

### P2: Važno

#### 4. Benchmark Suite
**Problem:** Nema baseline. Ne znamo da li je nešto brže ili sporije posle promene.

**Pristup:**
- `BenchmarkStep` — single turn latency
- `BenchmarkTokenCompact` — compaction performance
- `BenchmarkCodeSearchIndex` — index build time

**Vreme:** 4 sata

---

#### 5. Model Context Window Discovery
**Problem:** `modelContextWindow` mapa je hardkodirana. Novi modeli nemaju context info.

**Pristup:**
- Query provider API za context window gde je dostupno
- Fallback na hardkodiranu mapu
- Cache rezultate

**Vreme:** 4 sata

---

#### 6. MCP Tool Output Streaming
**Problem:** Kad MCP tool vrati veliki output, vidimo samo kad se završi.

**Pristup:**
- Streaming za stdio transport (već radi liniju po liniju)
- Chunked response za HTTP transport
- Progress indicator za dugotrajne tool pozive

**Vreme:** 1 dan

---

### P3: Nice to Have

#### 7. `m search` — Fuzzy Agent Search
**Problem:** `m list` postoji ali nema fuzzy pretragu po imenu/opisu.

**Vreme:** 2 sata

---

#### 8. Plugin System (WASM)
**Problem:** Custom tools zahtevaju MCP server. WASM plugins bi bili lakši.

**Vreme:** 3-5 dana (veći posao)

---

#### 9. Embedding-based RAG
**Problem:** `code_search` radi grep + symbol index, ali nema semantičku pretragu.

**Pristup:** Lokalni embedding model (Ollama) + SQLite vector store

**Vreme:** 3-5 dana

---

#### 10. Team Features
**Problem:** Nema shared agent registry, audit log, RBAC.

**Status:** Out of scope za v1.0. Single-developer use only.

---

## Roadmap do v1.0

| Verzija | Šta |
|---------|-----|
| v0.0.26 | Test coverage 60%+, structured logging |
| v0.0.27 | Graceful shutdown, benchmarks |
| v0.1.0-beta | Stabilizacija, bug fixes, docs polish |
| v1.0.0 | Stable release |

---

*Ažurirano: v0.0.25 — Steva Đubre*
