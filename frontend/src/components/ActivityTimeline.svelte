<script>
  export let events = [];
  export let collapsed = false;

  let expandedId = null;

  function fmtTime(ts) {
    if (!ts) return '';
    const d = new Date(ts);
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  }

  function kindIcon(k) {
    if (k === 'route') return '↗';
    if (k === 'tool') return '⚙';
    if (k === 'user') return '»';
    if (k === 'assistant') return '◆';
    if (k === 'error') return '✗';
    if (k === 'approval') return '⏸';
    if (k === 'continue') return '↻';
    if (k === 'status') return '·';
    return '·';
  }

  function kindClass(k) {
    if (k === 'error') return 'err';
    if (k === 'tool') return 'tool';
    if (k === 'route') return 'route';
    if (k === 'approval') return 'warn';
    return '';
  }

  function summary(ev) {
    if (ev.kind === 'route') return `route → ${ev.label} (${ev.detail?.model || ''})`;
    if (ev.kind === 'tool') {
      const auto = ev.detail?.auto ? 'auto' : (ev.detail?.ok === false ? 'denied' : 'approved');
      return `${ev.label} (${auto})`;
    }
    if (ev.kind === 'approval') return `awaiting approval: ${ev.label}`;
    if (ev.kind === 'continue') return `tool-step limit (${ev.detail?.turns || '?'})`;
    if (ev.kind === 'status') return ev.label || 'status';
    return ev.label || ev.kind;
  }

  function toggleExpand(id) {
    expandedId = expandedId === id ? null : id;
  }

  function detailText(ev) {
    if (!ev.detail) return ev.label || '';
    try {
      return JSON.stringify(ev.detail, null, 2);
    } catch (_) {
      return String(ev.detail);
    }
  }

  function hasDetail(ev) {
    return ev.detail && Object.keys(ev.detail).length > 0;
  }
</script>

<div class="timeline" class:collapsed>
  <button class="t-head" on:click={() => collapsed = !collapsed}>
    <span>Activity</span>
    <span class="cnt">{events.length}</span>
    <span class="chev">{collapsed ? '▸' : '▾'}</span>
  </button>
  {#if !collapsed}
    <div class="t-list">
      {#if events.length === 0}
        <div class="empty">Agent activity will appear here</div>
      {:else}
        {#each [...events].reverse() as ev (ev.id)}
          <div class="t-block">
            <button
              class="t-row {kindClass(ev.kind)}"
              class:expandable={hasDetail(ev)}
              class:expanded={expandedId === ev.id}
              on:click={() => hasDetail(ev) && toggleExpand(ev.id)}
            >
              <span class="t-time">{fmtTime(ev.ts)}</span>
              <span class="t-ico">{kindIcon(ev.kind)}</span>
              <span class="t-text">{summary(ev)}</span>
              {#if hasDetail(ev)}<span class="t-exp">{expandedId === ev.id ? '▾' : '▸'}</span>{/if}
            </button>
            {#if expandedId === ev.id && hasDetail(ev)}
              <pre class="t-detail">{detailText(ev)}</pre>
            {/if}
          </div>
        {/each}
      {/if}
    </div>
  {/if}
</div>

<style>
  .timeline{border-bottom:1px solid var(--border);flex-shrink:0;background:#080d18}
  .t-head{width:100%;display:flex;align-items:center;gap:8px;padding:8px 14px;background:none;border:none;color:var(--muted);font-size:11px;font-weight:700;text-transform:uppercase;letter-spacing:0.4px;cursor:pointer}
  .t-head:hover{color:var(--text)}
  .cnt{background:#1e293b;padding:1px 7px;border-radius:10px;font-size:10px}
  .chev{margin-left:auto;font-size:10px}
  .t-list{max-height:200px;overflow-y:auto;padding:0 10px 8px}
  .empty{padding:8px 4px;font-size:11px;color:var(--muted);text-align:center}
  .t-block{margin-bottom:2px}
  .t-row{display:flex;align-items:center;gap:8px;padding:5px 6px;font-size:11px;border-radius:6px;width:100%;border:none;background:none;text-align:left;color:inherit}
  .t-row.expandable{cursor:pointer}
  .t-row.expandable:hover,.t-row.expanded{background:#111c30}
  .t-time{color:var(--muted);font-family:'SF Mono',Menlo,monospace;font-size:10px;flex-shrink:0}
  .t-ico{flex-shrink:0;width:16px;text-align:center}
  .t-text{color:var(--text);white-space:nowrap;overflow:hidden;text-overflow:ellipsis;flex:1;min-width:0}
  .t-exp{color:var(--muted);font-size:9px;flex-shrink:0}
  .t-row.tool .t-text{color:#fbbf24}
  .t-row.route .t-text{color:#a78bfa}
  .t-row.err .t-text{color:#f87171}
  .t-row.warn .t-text{color:#fb923c}
  .t-detail{margin:0 6px 6px 30px;padding:8px 10px;background:#0c1322;border:1px solid var(--border);border-radius:6px;font-size:10px;color:#94a3b8;overflow-x:auto;max-height:120px;white-space:pre-wrap;word-break:break-all}
  .collapsed .t-list{display:none}
</style>
