<script>
  import { createEventDispatcher } from 'svelte';
  const dispatch = createEventDispatcher();

  export let health = { available: false, error: '' };
  export let stats = null;            // {chunks, documents, repos}
  export let indexing = null;         // {active, current, total, file, message, error}
  export let visibleTypes = new Set();
  export let nodeCounts = {};         // type -> count
  export let typeColors = {};

  $: types = Object.keys(nodeCounts).sort((a, b) => nodeCounts[b] - nodeCounts[a]);
  function pct(i) { return i && i.total ? Math.round((i.current / i.total) * 100) : 0; }
</script>

<aside class="kside">
  {#if !health.available}
    <div class="down">
      <div class="down-title">knowledge-master is not running</div>
      <p>Start the backend, then refresh:</p>
      <pre>km start
km serve</pre>
      {#if health.error}<div class="down-err">{health.error}</div>{/if}
      <button class="btn" on:click={() => dispatch('refresh')}>↻ Retry</button>
    </div>
  {:else}
    <div class="section">
      <div class="stats">
        <div class="stat"><span class="num">{stats?.repos ?? '–'}</span><span class="lbl">repos</span></div>
        <div class="stat"><span class="num">{stats?.documents ?? '–'}</span><span class="lbl">docs</span></div>
        <div class="stat"><span class="num">{stats?.chunks ?? '–'}</span><span class="lbl">chunks</span></div>
      </div>
    </div>

    <div class="section">
      <button class="btn primary" on:click={() => dispatch('index')} disabled={indexing?.active}>
        {indexing?.active ? 'Indexing…' : '＋ Index folder / repo'}
      </button>
      {#if indexing?.active}
        <div class="prog">
          <div class="bar"><div class="fill" style="width:{pct(indexing)}%"></div></div>
          <div class="prog-meta">{indexing.current ?? 0}/{indexing.total ?? '?'} · {pct(indexing)}%</div>
          {#if indexing.file}<div class="prog-file" title={indexing.file}>{indexing.file}</div>{/if}
        </div>
      {:else if indexing?.message}
        <div class="ok">✓ {indexing.message}</div>
      {:else if indexing?.error}
        <div class="down-err">⚠ {indexing.error}</div>
      {/if}
    </div>

    <div class="section grow">
      <div class="section-head">
        <span class="group-label">Node types</span>
        <button class="mini" on:click={() => dispatch('refresh')} title="Reload graph">↻</button>
      </div>
      <div class="legend">
        {#each types as t}
          <label class="leg-row" class:off={!visibleTypes.has(t)}>
            <input type="checkbox" checked={visibleTypes.has(t)} on:change={() => dispatch('toggle', t)} />
            <span class="dot" style="background:{typeColors[t] || '#94a3b8'}"></span>
            <span class="leg-name">{t}</span>
            <span class="leg-count">{nodeCounts[t]}</span>
          </label>
        {/each}
      </div>
    </div>
  {/if}

</aside>

<style>
  .kside{width:100%;flex:1;min-height:0;display:flex;flex-direction:column;overflow:hidden;padding:8px}
  .section{padding:8px 6px;border-bottom:1px solid #1e293b}
  .section.grow{flex:1;min-height:0;display:flex;flex-direction:column;border-bottom:none}
  .section-head{display:flex;justify-content:space-between;align-items:center}
  .group-label{font-size:10px;font-weight:700;color:#475569;text-transform:uppercase;letter-spacing:0.5px}

  .stats{display:flex;gap:6px}
  .stat{flex:1;background:#0c1322;border:1px solid #1e293b;border-radius:8px;padding:8px 4px;display:flex;flex-direction:column;align-items:center}
  .num{font-size:16px;font-weight:700;color:#e2e8f0}
  .lbl{font-size:10px;color:#64748b;text-transform:uppercase;letter-spacing:0.3px}

  .btn{width:100%;padding:9px;border:1px solid #334155;border-radius:8px;background:#1e293b;color:#e2e8f0;font-size:13px;cursor:pointer}
  .btn:hover:not(:disabled){background:#334155}
  .btn:disabled{opacity:0.6;cursor:not-allowed}
  .btn.primary{background:var(--accent);border-color:var(--accent);color:#fff;font-weight:600}
  .btn.primary:hover:not(:disabled){filter:brightness(1.1)}

  .prog{margin-top:8px}
  .bar{height:6px;background:#1e293b;border-radius:4px;overflow:hidden}
  .fill{height:100%;background:var(--accent);transition:width 0.2s}
  .prog-meta{font-size:11px;color:#94a3b8;margin-top:4px}
  .prog-file{font-size:10px;color:#64748b;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;margin-top:2px}
  .ok{margin-top:8px;font-size:12px;color:#34d399}

  .legend{margin-top:6px;overflow-y:auto;display:flex;flex-direction:column;gap:1px}
  .leg-row{display:flex;align-items:center;gap:7px;padding:4px 6px;border-radius:6px;cursor:pointer;font-size:12px;color:#e2e8f0}
  .leg-row:hover{background:#1e293b}
  .leg-row.off{opacity:0.4}
  .leg-row input{margin:0}
  .dot{width:9px;height:9px;border-radius:50%;flex-shrink:0}
  .leg-name{flex:1}
  .leg-count{font-size:11px;color:#64748b}

  .down{padding:16px 10px;text-align:center;color:#94a3b8;font-size:12px}
  .down-title{color:#f1f5f9;font-weight:600;margin-bottom:6px}
  .down pre{text-align:left;background:#0c1322;border:1px solid #1e293b;border-radius:6px;padding:8px 10px;margin:8px 0;font-size:11px;color:#cbd5e1}
  .down-err{font-size:11px;color:#fca5a5;margin:8px 0;word-break:break-word}
  .mini{background:none;border:none;color:#64748b;cursor:pointer;font-size:13px}
  .mini:hover{color:#e2e8f0}

  .footer{padding:10px 6px 4px;border-top:1px solid #1e293b;flex-shrink:0}
  .settings-btn{width:100%;padding:8px;background:none;border:1px solid #334155;border-radius:6px;color:#64748b;font-size:13px;cursor:pointer}
  .settings-btn:hover{background:#1e293b;color:#e2e8f0}
</style>
