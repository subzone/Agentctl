<script>
  export let mode = 'tool'; // 'tool' | 'mcp'
  export let name = '';
  export let disabled = false;

  let input = '{\n  "text": "hello"\n}';
  let result = null;
  let running = false;
  let open = false;

  async function run() {
    if (!name || disabled) return;
    running = true;
    result = null;
    try {
      if (mode === 'tool') {
        result = await window.go.desktop.App.TestTool(name, input);
      } else {
        result = await window.go.desktop.App.TestMCP(name);
      }
    } catch (e) {
      result = { ok: false, error: String(e) };
    } finally {
      running = false;
      open = true;
    }
  }
</script>

<div class="bench">
  <button class="bench-toggle" on:click={() => open = !open}>
    <span>🧪 Test bench</span>
    <span class="chev">{open ? '▾' : '▸'}</span>
  </button>
  {#if open}
    <div class="bench-body">
      {#if mode === 'tool'}
        <label class="lbl">JSON input (stdin)</label>
        <textarea class="inp" bind:value={input} spellcheck="false" rows="4" placeholder={'{"key": "value"}'}></textarea>
      {:else}
        <p class="hint">Starts the MCP server briefly and lists exposed tools.</p>
      {/if}
      <button class="run" on:click={run} disabled={running || !name || disabled}>
        {running ? 'Running…' : mode === 'tool' ? 'Run tool' : 'Probe server'}
      </button>
      {#if result}
        <div class="out" class:ok={result.ok} class:fail={!result.ok}>
          {#if result.durationMs != null}
            <div class="meta">{result.durationMs}ms</div>
          {/if}
          {#if result.error}<div class="err">{result.error}</div>{/if}
          {#if result.output}<pre>{result.output}</pre>{/if}
          {#if result.tools?.length}
            <div class="tools-list"><strong>{result.tools.length} tool(s):</strong> {result.tools.join(', ')}</div>
          {/if}
          {#if result.log}<pre class="log">{result.log}</pre>{/if}
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .bench{border-top:1px solid var(--border);flex-shrink:0;background:#080d18}
  .bench-toggle{width:100%;display:flex;justify-content:space-between;align-items:center;padding:10px 16px;background:none;border:none;color:var(--muted);font-size:12px;font-weight:600;cursor:pointer}
  .bench-toggle:hover{color:var(--text)}
  .chev{font-size:10px}
  .bench-body{padding:0 16px 14px;display:flex;flex-direction:column;gap:8px}
  .lbl{font-size:10px;font-weight:700;color:var(--muted);text-transform:uppercase}
  .inp{width:100%;box-sizing:border-box;padding:8px 10px;background:#0c1322;border:1px solid var(--border);border-radius:6px;color:#e2e8f0;font-family:'SF Mono',Menlo,monospace;font-size:12px;resize:vertical}
  .hint{font-size:11px;color:var(--muted);margin:0}
  .run{align-self:flex-start;padding:7px 14px;border-radius:6px;border:none;background:var(--accent);color:#fff;font-size:12px;font-weight:600;cursor:pointer}
  .run:disabled{opacity:0.5;cursor:not-allowed}
  .out{padding:10px 12px;border-radius:8px;font-size:12px;background:#0c1322;border:1px solid var(--border)}
  .out.ok{border-color:#22c55e}
  .out.fail{border-color:#ef4444}
  .meta{font-size:10px;color:var(--muted);margin-bottom:6px}
  .err{color:#f87171;margin-bottom:6px}
  pre{margin:0;white-space:pre-wrap;word-break:break-word;font-family:'SF Mono',Menlo,monospace;font-size:11px;color:#cbd5e1;max-height:160px;overflow:auto}
  .log{margin-top:8px;opacity:0.8;font-size:10px}
  .tools-list{font-size:11px;color:#86efac;margin-top:4px}
</style>
