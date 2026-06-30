<script>
  export let events = [];
  export let collapsed = false;

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
    return ev.label || ev.kind;
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
          <div class="t-row {kindClass(ev.kind)}">
            <span class="t-time">{fmtTime(ev.ts)}</span>
            <span class="t-ico">{kindIcon(ev.kind)}</span>
            <span class="t-text">{summary(ev)}</span>
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
  .t-list{max-height:140px;overflow-y:auto;padding:0 10px 8px}
  .empty{padding:8px 4px;font-size:11px;color:var(--muted);text-align:center}
  .t-row{display:flex;align-items:center;gap:8px;padding:5px 6px;font-size:11px;border-radius:6px}
  .t-row:hover{background:#111c30}
  .t-time{color:var(--muted);font-family:'SF Mono',Menlo,monospace;font-size:10px;flex-shrink:0}
  .t-ico{flex-shrink:0;width:16px;text-align:center}
  .t-text{color:var(--text);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .t-row.tool .t-text{color:#fbbf24}
  .t-row.route .t-text{color:#a78bfa}
  .t-row.err .t-text{color:#f87171}
  .t-row.warn .t-text{color:#fb923c}
  .collapsed .t-list{display:none}
</style>
