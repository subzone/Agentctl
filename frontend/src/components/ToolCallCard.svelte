<script>
  export let call = {};
  export let expanded = false;

  $: status = call.status || 'running';
  $: statusLabel = status === 'running' ? 'running…' :
    status === 'success' ? 'done' :
    status === 'declined' ? 'denied' :
    status === 'policy_denied' ? 'policy blocked' :
    status === 'error' ? 'error' : status;

  function toggle() { expanded = !expanded; }

  function fmtDur(ms) {
    if (!ms && ms !== 0) return '';
    if (ms < 1000) return ms + 'ms';
    return (ms / 1000).toFixed(1) + 's';
  }
</script>

<div class="tc" class:running={status === 'running'} class:err={status === 'error' || status === 'policy_denied'}>
  <button class="tc-head" on:click={toggle}>
    <span class="ico">⚙</span>
    <span class="name">{call.name || 'tool'}</span>
    <span class="st {status}">{statusLabel}</span>
    {#if call.durationMs}<span class="dur">{fmtDur(call.durationMs)}</span>{/if}
    <span class="chev">{expanded ? '▾' : '▸'}</span>
  </button>
  {#if expanded}
    <div class="tc-body">
      {#if call.input}<div class="lbl">Input</div><pre>{call.input}</pre>{/if}
      {#if call.output}<div class="lbl">Output</div><pre>{call.output}</pre>{/if}
      {#if call.error}<div class="lbl">Error</div><pre class="err-txt">{call.error}</pre>{/if}
    </div>
  {/if}
</div>

<style>
  .tc{border:1px solid var(--border,#334155);border-radius:8px;background:#0c1322;margin:6px 0;overflow:hidden}
  .tc.running{border-color:var(--accent)}
  .tc.err{border-color:#ef4444}
  .tc-head{width:100%;display:flex;align-items:center;gap:8px;padding:8px 10px;background:none;border:none;color:var(--text,#e2e8f0);cursor:pointer;text-align:left;font-size:12px}
  .tc-head:hover{background:#111c30}
  .ico{opacity:0.8}
  .name{font-weight:600;flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .st{font-size:10px;font-weight:700;padding:2px 8px;border-radius:10px;text-transform:uppercase}
  .st.running{background:#312e81;color:#c4b5fd}
  .st.success{background:#14532d;color:#86efac}
  .st.declined,.st.policy_denied{background:#713f12;color:#fcd34d}
  .st.error{background:#7f1d1d;color:#fca5a5}
  .dur{font-size:10px;color:var(--muted,#64748b);font-family:monospace}
  .chev{color:var(--muted);font-size:10px}
  .tc-body{padding:0 10px 10px;font-size:11px}
  .lbl{color:var(--muted);font-size:10px;font-weight:700;text-transform:uppercase;margin:8px 0 4px}
  pre{margin:0;padding:8px;background:#080d18;border-radius:6px;overflow-x:auto;white-space:pre-wrap;word-break:break-all;color:#94a3b8;max-height:160px}
  .err-txt{color:#fca5a5}
</style>
