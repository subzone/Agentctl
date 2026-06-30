<script>
  import { createEventDispatcher } from 'svelte';
  export let agents = [];
  export let active = null;
  const dispatch = createEventDispatcher();

  let search = '';
  $: defaultAgents = agents.filter(a => a.category === 'default' && matchSearch(a));
  $: hubAgents = agents.filter(a => a.category === 'hub' && matchSearch(a));
  $: standaloneAgents = agents.filter(a => a.category !== 'hub' && a.category !== 'spoke' && a.category !== 'default' && matchSearch(a));

  function matchSearch(a) {
    return !search || a.name.toLowerCase().includes(search.toLowerCase());
  }

  // Session history (stored in localStorage)
  let sessionHistory = JSON.parse(localStorage.getItem('agentctl-sessions') || '[]');

  export function addSession(name) {
    const entry = { name, ts: Date.now() };
    sessionHistory = [entry, ...sessionHistory.slice(0, 9)];
    localStorage.setItem('agentctl-sessions', JSON.stringify(sessionHistory));
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

    {#if hubAgents.length === 0 && standaloneAgents.length === 0}
      <div class="empty">No agents found</div>
    {/if}
  </div>

  {#if sessionHistory.length > 0}
    <div class="history">
      <div class="group-label">Recent Sessions</div>
      {#each sessionHistory as s}
        <div class="history-item">{s.name} <span class="ts">{new Date(s.ts).toLocaleDateString()}</span></div>
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

  .agent-list { flex: 1; overflow-y: auto; padding: 0 8px 8px; }

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

  .history{padding:0 8px 8px;border-top:1px solid #1e293b;max-height:120px;overflow-y:auto}
  .history-item{font-size:11px;color:#64748b;padding:4px 10px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .history-item .ts{float:right;color:#334155;font-size:10px}

  .footer{padding:12px;border-top:1px solid #1e293b;flex-shrink:0}
  .settings-btn{width:100%;padding:8px;background:none;border:1px solid #334155;border-radius:6px;color:#64748b;font-size:13px;cursor:pointer}
  .settings-btn:hover{background:#1e293b;color:#e2e8f0}
</style>
