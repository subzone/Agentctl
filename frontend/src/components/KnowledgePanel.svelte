<script>
  import { createEventDispatcher } from 'svelte';
  const dispatch = createEventDispatcher();

  export let health = { available: false, error: '' };
  export let stats = null;
  export let indexing = null;
  export let visibleTypes = new Set();
  export let nodeCounts = {};
  export let typeColors = {};
  export let nodes = [];
  export let selectedId = '';
  export let selectedNode = null;

  let filter = '';
  let searchQuery = '';
  let searchResults = '';
  let searchLoading = false;
  let searchError = '';

  $: types = Object.keys(nodeCounts).sort((a, b) => nodeCounts[b] - nodeCounts[a]);
  $: filteredNodes = nodes
    .filter(n => visibleTypes.has(n.type))
    .filter(n => {
      if (!filter.trim()) return true;
      const q = filter.toLowerCase();
      return (n.label || '').toLowerCase().includes(q)
        || (n.type || '').toLowerCase().includes(q)
        || (n.source || '').toLowerCase().includes(q);
    })
    .slice(0, 80);

  function pct(i) { return i && i.total ? Math.round((i.current / i.total) * 100) : 0; }

  async function runSearch() {
    const q = searchQuery.trim();
    if (!q) return;
    searchLoading = true;
    searchError = '';
    searchResults = '';
    try {
      searchResults = await window.go.desktop.App.KMSearch(q, 8);
    } catch (e) {
      searchError = String(e);
    } finally {
      searchLoading = false;
    }
  }

  function onKey(e) {
    if (e.key === 'Enter') runSearch();
  }

  function pickNode(n) {
    selectedNode = n;
    dispatch('select', n.id);
  }

  function askAbout() {
    if (!selectedNode) return;
    dispatch('ask', selectedNode);
  }
</script>

<aside class="panel">
  {#if !health.available}
    <div class="down">
      <div class="down-title">knowledge-master offline</div>
      <pre>km start
km serve</pre>
      {#if health.error}<div class="down-err">{health.error}</div>{/if}
      <button class="btn" on:click={() => dispatch('refresh')}>↻ Retry</button>
    </div>
  {:else}
    <div class="stats">
      <div class="stat"><span class="num">{stats?.repos ?? '–'}</span><span class="lbl">repos</span></div>
      <div class="stat"><span class="num">{stats?.documents ?? '–'}</span><span class="lbl">docs</span></div>
      <div class="stat"><span class="num">{stats?.chunks ?? '–'}</span><span class="lbl">chunks</span></div>
    </div>

    <div class="block">
      <button class="btn primary" on:click={() => dispatch('index')} disabled={indexing?.active}>
        {indexing?.active ? 'Indexing…' : '＋ Index folder'}
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

    <div class="block">
      <div class="head">Semantic search</div>
      <div class="search-row">
        <input class="search" bind:value={searchQuery} placeholder="Search indexed knowledge…" on:keydown={onKey} />
        <button class="btn mini" on:click={runSearch} disabled={searchLoading || !searchQuery.trim()}>
          {searchLoading ? '…' : 'Go'}
        </button>
      </div>
      {#if searchError}<div class="down-err">{searchError}</div>{/if}
      {#if searchResults}
        <pre class="results">{searchResults}</pre>
      {/if}
    </div>

    <div class="block grow">
      <div class="head">
        <span>Nodes</span>
        <button class="link" on:click={() => dispatch('refresh')}>↻</button>
      </div>
      <input class="filter" bind:value={filter} placeholder="Filter by label, type, source…" />
      {#if selectedNode}
        <button class="ask-btn" on:click={askAbout}>💬 Ask in chat about “{selectedNode.label || selectedNode.id}”</button>
      {/if}
      <div class="types">
        {#each types as t}
          <label class="type" class:off={!visibleTypes.has(t)}>
            <input type="checkbox" checked={visibleTypes.has(t)} on:change={() => dispatch('toggle', t)} />
            <span class="dot" style="background:{typeColors[t] || '#94a3b8'}"></span>
            <span>{t}</span>
            <span class="cnt">{nodeCounts[t]}</span>
          </label>
        {/each}
      </div>
      <div class="nlist">
        {#each filteredNodes as n (n.id)}
          <button class="nrow" class:sel={selectedId === n.id} on:click={() => pickNode(n)}>
            <span class="dot" style="background:{typeColors[n.type] || '#94a3b8'}"></span>
            <span class="lbl" title={n.label}>{n.label || n.id}</span>
            <span class="typ">{n.type}</span>
          </button>
        {:else}
          <div class="empty">No nodes match</div>
        {/each}
      </div>
    </div>
  {/if}
</aside>

<style>
  .panel{width:300px;flex-shrink:0;display:flex;flex-direction:column;gap:0;border-left:1px solid var(--border,#1e293b);background:#080d18;overflow:hidden}
  .stats{display:flex;gap:6px;padding:12px 12px 0}
  .stat{flex:1;background:#0c1322;border:1px solid #1e293b;border-radius:8px;padding:8px 4px;display:flex;flex-direction:column;align-items:center}
  .num{font-size:15px;font-weight:700;color:#e2e8f0}
  .lbl{font-size:9px;color:#64748b;text-transform:uppercase}
  .block{padding:10px 12px;border-bottom:1px solid #1e293b}
  .block.grow{flex:1;min-height:0;display:flex;flex-direction:column;border-bottom:none;padding-bottom:0}
  .head{display:flex;justify-content:space-between;align-items:center;font-size:10px;font-weight:700;color:#64748b;text-transform:uppercase;margin-bottom:8px}
  .link{background:none;border:none;color:#64748b;cursor:pointer;font-size:12px}
  .link:hover{color:#e2e8f0}
  .btn{width:100%;padding:8px;border:1px solid #334155;border-radius:8px;background:#1e293b;color:#e2e8f0;font-size:12px;cursor:pointer}
  .btn:hover:not(:disabled){background:#334155}
  .btn:disabled{opacity:0.6;cursor:not-allowed}
  .btn.primary{background:var(--accent);border-color:var(--accent);color:#fff;font-weight:600}
  .btn.mini{width:auto;padding:8px 12px;flex-shrink:0}
  .search-row{display:flex;gap:6px}
  .search,.filter{width:100%;box-sizing:border-box;padding:8px 10px;background:#0c1322;border:1px solid #334155;border-radius:6px;color:#e2e8f0;font-size:12px}
  .filter{margin-bottom:8px}
  .ask-btn{width:100%;margin-bottom:8px;padding:8px 10px;border-radius:6px;border:1px solid #4338ca;background:#1e1b4b;color:#c4b5fd;font-size:11px;font-weight:600;cursor:pointer;text-align:left}
  .ask-btn:hover{background:#312e81}
  .results{margin:8px 0 0;padding:8px;background:#0c1322;border:1px solid #1e293b;border-radius:6px;font-size:10px;color:#cbd5e1;max-height:120px;overflow:auto;white-space:pre-wrap;word-break:break-word}
  .prog{margin-top:8px}
  .bar{height:5px;background:#1e293b;border-radius:4px;overflow:hidden}
  .fill{height:100%;background:var(--accent)}
  .prog-meta,.prog-file{font-size:10px;color:#64748b;margin-top:4px}
  .prog-file{white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .ok{margin-top:6px;font-size:11px;color:#34d399}
  .types{display:flex;flex-wrap:wrap;gap:4px;margin-bottom:8px}
  .type{display:flex;align-items:center;gap:4px;padding:3px 6px;border-radius:6px;background:#0c1322;font-size:10px;color:#cbd5e1;cursor:pointer}
  .type.off{opacity:0.45}
  .type input{margin:0}
  .dot{width:7px;height:7px;border-radius:50%;flex-shrink:0}
  .cnt{color:#64748b}
  .nlist{flex:1;overflow-y:auto;display:flex;flex-direction:column;gap:1px;padding-bottom:8px}
  .nrow{display:flex;align-items:center;gap:6px;width:100%;padding:6px 8px;border:none;border-radius:6px;background:transparent;color:#e2e8f0;font-size:11px;text-align:left;cursor:pointer}
  .nrow:hover{background:#111c30}
  .nrow.sel{background:#1e293b;outline:1px solid var(--accent)}
  .nrow .lbl{flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .nrow .typ{font-size:9px;color:#64748b;text-transform:uppercase}
  .empty{padding:12px;text-align:center;color:#475569;font-size:11px}
  .down{padding:20px 12px;text-align:center;color:#94a3b8;font-size:12px}
  .down-title{color:#f1f5f9;font-weight:600;margin-bottom:8px}
  .down pre{text-align:left;background:#0c1322;border:1px solid #1e293b;border-radius:6px;padding:8px;font-size:11px;color:#cbd5e1;margin:8px 0}
  .down-err{font-size:11px;color:#fca5a5;margin-top:6px;word-break:break-word}
</style>
