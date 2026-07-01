<script>
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  const dispatch = createEventDispatcher();

  export let sessionId = null;
  export let embedded = false;

  let entries = [];
  let loading = false;
  let probing = false;
  let pollTimer;

  async function refresh(probe = false) {
    loading = !probe;
    if (probe) probing = true;
    try {
      entries = await window.go.desktop.App.GetMCPDashboard(sessionId || '', probe) || [];
    } catch (_) {
      entries = [];
    } finally {
      loading = false;
      probing = false;
    }
  }

  onMount(() => {
    refresh(false);
    pollTimer = setInterval(() => refresh(false), 12000);
  });

  onDestroy(() => {
    if (pollTimer) clearInterval(pollTimer);
  });

  function statusLabel(e) {
    if (e.configError) return 'config error';
    if (e.inSession) return 'live';
    if (e.reachable) return 'reachable';
    if (e.error) return 'offline';
    return 'idle';
  }

  function statusClass(e) {
    const s = statusLabel(e);
    if (s === 'live') return 'live';
    if (s === 'reachable') return 'ok';
    if (s === 'config error' || s === 'offline') return 'err';
    return 'idle';
  }
</script>

{#if embedded}
  <div class="dash embedded">
    <div class="dash-head">
      <h3>🔌 MCP connections</h3>
      <button class="mini" disabled={probing} on:click={() => refresh(true)}>{probing ? '…' : '↻ Test all'}</button>
    </div>
    {#if loading && entries.length === 0}
      <div class="empty">Loading…</div>
    {:else if entries.length === 0}
      <div class="empty">No MCP servers configured. Add one under Extensions → MCP.</div>
    {:else}
      <div class="grid">
        {#each entries as e}
          <div class="card">
            <div class="card-top">
              <span class="name">{e.name}</span>
              <span class="badge {statusClass(e)}">{statusLabel(e)}</span>
            </div>
            <div class="meta">{e.transport || 'stdio'}{#if e.inSession} · in chat{/if}</div>
            {#if e.description}<div class="desc">{e.description}</div>{/if}
            {#if e.toolCount > 0}<div class="tools">{e.toolCount} tools</div>{/if}
            {#if e.configError}<div class="err">{e.configError}</div>{/if}
            {#if e.error && !e.configError}<div class="err">{e.error}</div>{/if}
          </div>
        {/each}
      </div>
    {/if}
  </div>
{:else}
  <div class="overlay" role="presentation" on:click={() => dispatch('close')} on:keydown={() => {}}>
    <div class="modal" role="dialog" on:click|stopPropagation on:keydown={() => {}}>
      <div class="dash-head">
        <h3>🔌 MCP live dashboard</h3>
        <div class="actions">
          <button class="mini" disabled={probing} on:click={() => refresh(true)}>{probing ? 'Testing…' : '↻ Test all'}</button>
          <button class="mini close" on:click={() => dispatch('close')}>✕</button>
        </div>
      </div>
      <p class="sub">Servers configured in <code>~/.config/m/mcp</code>. “Live” means connected in the current chat session.</p>
      {#if entries.length === 0}
        <div class="empty">No MCP servers configured.</div>
      {:else}
        <div class="grid">
          {#each entries as e}
            <div class="card">
              <div class="card-top">
                <span class="name">{e.name}</span>
                <span class="badge {statusClass(e)}">{statusLabel(e)}</span>
              </div>
              <div class="meta">{e.transport || 'stdio'}{#if e.inSession} · active in session{/if}</div>
              {#if e.description}<div class="desc">{e.description}</div>{/if}
              {#if e.tools?.length}
                <div class="tool-list">{e.tools.slice(0, 6).join(' · ')}{#if e.tools.length > 6}…{/if}</div>
              {/if}
              {#if e.configError}<div class="err">{e.configError}</div>{/if}
              {#if e.error && !e.configError}<div class="err">{e.error}</div>{/if}
            </div>
          {/each}
        </div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .overlay{position:fixed;inset:0;background:rgba(0,0,0,0.55);z-index:200;display:flex;align-items:center;justify-content:center;padding:24px}
  .modal{background:#0f172a;border:1px solid #334155;border-radius:12px;max-width:640px;width:100%;max-height:80vh;overflow-y:auto;padding:20px}
  .dash.embedded{padding:12px 16px;border-top:1px solid var(--border);background:#080d18}
  .dash-head{display:flex;align-items:center;justify-content:space-between;gap:12px;margin-bottom:8px}
  .dash-head h3{font-size:14px;font-weight:700;color:var(--text)}
  .actions{display:flex;gap:6px}
  .mini{background:#1e293b;border:1px solid #334155;border-radius:6px;color:#e2e8f0;font-size:11px;font-weight:600;padding:5px 10px;cursor:pointer}
  .mini:disabled{opacity:0.5;cursor:not-allowed}
  .mini.close{width:28px;padding:5px 0}
  .sub{font-size:12px;color:var(--muted);margin-bottom:12px;line-height:1.5}
  .sub code{background:#1e293b;padding:1px 5px;border-radius:4px;font-size:11px}
  .grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(240px,1fr));gap:10px}
  .card{background:#1e293b;border:1px solid #334155;border-radius:8px;padding:12px}
  .card-top{display:flex;align-items:center;justify-content:space-between;gap:8px;margin-bottom:4px}
  .name{font-size:13px;font-weight:600;color:var(--text)}
  .badge{font-size:9px;font-weight:700;text-transform:uppercase;padding:2px 7px;border-radius:4px}
  .badge.live{background:#14532d;color:#86efac}
  .badge.ok{background:#1e3a5f;color:#7dd3fc}
  .badge.err{background:#7f1d1d;color:#fca5a5}
  .badge.idle{background:#334155;color:#94a3b8}
  .meta{font-size:11px;color:var(--muted)}
  .desc{font-size:12px;color:#cbd5e1;margin-top:6px;line-height:1.4}
  .tools,.tool-list{font-size:11px;color:#93c5fd;margin-top:6px;font-family:'SF Mono',Menlo,monospace}
  .err{font-size:11px;color:#fca5a5;margin-top:6px;word-break:break-word}
  .empty{padding:20px;text-align:center;color:var(--muted);font-size:13px}
</style>
