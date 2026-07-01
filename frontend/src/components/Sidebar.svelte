<script>
  import { createEventDispatcher } from 'svelte';
  export let agents = [];
  export let active = null;
  export let savedSessions = [];
  export let sessionSearch = '';
  const dispatch = createEventDispatcher();

  let search = '';
  $: defaultAgents = agents.filter(a => a.category === 'default' && matchSearch(a));
  $: hubAgents = agents.filter(a => a.category === 'hub' && matchSearch(a));
  $: standaloneAgents = agents.filter(a => a.category !== 'hub' && a.category !== 'spoke' && a.category !== 'default' && matchSearch(a));

  function matchSearch(a) {
    return !search || a.name.toLowerCase().includes(search.toLowerCase());
  }

  function fmtSaved(ts) {
    if (!ts) return '';
    const d = new Date(ts * 1000);
    return d.toLocaleDateString([], { month: 'short', day: 'numeric' });
  }
</script>

<aside class="sidebar">
  <div class="search-box">
    <input type="text" placeholder="Search agents..." bind:value={search} />
  </div>

  <div class="agent-list">
    {#if defaultAgents.length > 0}
      <div class="group-label">Default</div>
      {#each defaultAgents as agent}
        <button class="agent-btn default-agent" class:active={active?.name === agent.name} on:click={() => dispatch('select', agent)}>
          <span class="icon">◆</span>
          <div class="info">
            <div class="name">{agent.name} <span class="moe-tag">MoE</span></div>
            <div class="desc">routes to the best free model</div>
          </div>
        </button>
      {/each}
    {/if}

    {#if hubAgents.length > 0}
      <div class="group-label">Hub Agents</div>
      {#each hubAgents as agent}
        <button class="agent-btn" class:active={active?.name === agent.name} on:click={() => dispatch('select', agent)}>
          <span class="icon">🔀</span>
          <div class="info">
            <div class="name">{agent.name}</div>
            <div class="desc">{agent.model}</div>
          </div>
        </button>
      {/each}
    {/if}

    {#if standaloneAgents.length > 0}
      <div class="group-label">Standalone</div>
      {#each standaloneAgents as agent}
        <button class="agent-btn" class:active={active?.name === agent.name} on:click={() => dispatch('select', agent)}>
          <span class="icon">📦</span>
          <div class="info">
            <div class="name">{agent.name}</div>
            <div class="desc">{agent.model}</div>
          </div>
        </button>
      {/each}
    {/if}

    {#if hubAgents.length === 0 && standaloneAgents.length === 0 && defaultAgents.length === 0}
      <div class="empty">No agents found</div>
    {/if}
  </div>

  {#if savedSessions.length > 0 || sessionSearch}
    <div class="history">
      <div class="group-label">Saved chats</div>
      <input class="sess-search" type="search" placeholder="Search sessions…" bind:value={sessionSearch}
        on:input={() => dispatch('search')} />
      {#each savedSessions.slice(0, 8) as s}
        <button class="history-item" on:click={() => dispatch('resume', s.id)} title={s.preview}>
          <span class="hist-preview">{s.label || s.preview || 'Saved chat'}</span>
          <span class="ts">{s.messages} · {fmtSaved(s.savedAt)}</span>
        </button>
      {/each}
    </div>
  {/if}
</aside>

<style>
  .sidebar {
    width: 100%; flex: 1; min-height: 0; background: #0c1322;
    display: flex; flex-direction: column; overflow: hidden;
  }
  .search-box { padding: 12px; }
  .search-box input {
    width: 100%; padding: 8px 12px; background: #1e293b; border: 1px solid #334155;
    border-radius: 6px; color: #e2e8f0; font-size: 13px; outline: none;
  }
  .search-box input:focus { border-color: #3b82f6; }

  .agent-list { flex: 1; overflow-y: auto; padding: 0 8px 8px; min-height: 0; }

  .group-label{font-size:10px;font-weight:700;color:#475569;text-transform:uppercase;letter-spacing:0.5px;padding:10px 10px 4px;margin-top:4px}

  .agent-btn {
    width: 100%; display: flex; align-items: center; gap: 8px;
    padding: 8px 10px; background: none; border: none; border-radius: 6px;
    color: #e2e8f0; cursor: pointer; text-align: left; margin-bottom: 2px;
  }
  .agent-btn:hover { background: #1e293b; }
  .agent-btn.active{background:#1e293b;border-left:2px solid #3b82f6}
  .default-agent{background:color-mix(in srgb,#6366f1 12%,transparent);border:1px solid color-mix(in srgb,#6366f1 30%,transparent)}
  .default-agent .icon{color:#818cf8}
  .moe-tag{font-size:9px;background:#6366f1;color:#fff;padding:1px 5px;border-radius:4px;vertical-align:middle;font-weight:600}
  .icon { font-size: 16px; }
  .info { min-width: 0; }
  .name { font-size: 13px; font-weight: 500; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .desc { font-size: 11px; color: #64748b; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .empty { padding: 20px; text-align: center; color: #475569; font-size: 13px; }

  .history{padding:0 8px 10px;border-top:1px solid #1e293b;max-height:180px;overflow-y:auto;flex-shrink:0}
  .sess-search{width:100%;margin:0 0 6px;padding:6px 8px;background:#1e293b;border:1px solid #334155;border-radius:6px;color:#e2e8f0;font-size:11px;outline:none;box-sizing:border-box}
  .sess-search:focus{border-color:#3b82f6}
  .history-item{
    display:flex;align-items:center;gap:6px;width:100%;padding:6px 10px;
    background:none;border:none;border-radius:6px;cursor:pointer;text-align:left;color:#94a3b8;
  }
  .history-item:hover{background:#1e293b;color:#e2e8f0}
  .hist-preview{flex:1;font-size:11px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .history-item .ts{flex-shrink:0;color:#334155;font-size:10px}
</style>
