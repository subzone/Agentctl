<script>
  import { createEventDispatcher } from 'svelte';
  export let agents = [];
  const dispatch = createEventDispatcher();

  let search = '';
  $: filtered = agents.filter(a =>
    a.category !== 'spoke' &&
    (a.name.toLowerCase().includes(search.toLowerCase()) || !search)
  );
</script>

<aside class="sidebar">
  <div class="search-box">
    <input type="text" placeholder="Search agents..." bind:value={search} />
  </div>

  <div class="agent-list">
    {#each filtered as agent}
      <button class="agent-btn" on:click={() => dispatch('select', agent)}>
        <span class="icon">{agent.category === 'hub' ? '🔀' : '📦'}</span>
        <div class="info">
          <div class="name">{agent.name}</div>
          <div class="desc">{agent.model}</div>
        </div>
      </button>
    {/each}

    {#if filtered.length === 0}
      <div class="empty">No agents found</div>
    {/if}
  </div>

  <div class="footer">
    <button class="settings-btn" on:click={() => dispatch('settings')}>⚙ Settings</button>
  </div>
</aside>

<style>
  .sidebar {
    width: 240px; background: #0c1322; border-right: 1px solid #1e293b;
    display: flex; flex-direction: column; overflow: hidden;
  }
  .search-box { padding: 12px; }
  .search-box input {
    width: 100%; padding: 8px 12px; background: #1e293b; border: 1px solid #334155;
    border-radius: 6px; color: #e2e8f0; font-size: 13px; outline: none;
  }
  .search-box input:focus { border-color: #3b82f6; }

  .agent-list { flex: 1; overflow-y: auto; padding: 0 8px 8px; }

  .agent-btn {
    width: 100%; display: flex; align-items: center; gap: 8px;
    padding: 8px 10px; background: none; border: none; border-radius: 6px;
    color: #e2e8f0; cursor: pointer; text-align: left; margin-bottom: 2px;
  }
  .agent-btn:hover { background: #1e293b; }
  .icon { font-size: 16px; }
  .info { min-width: 0; }
  .name { font-size: 13px; font-weight: 500; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .desc { font-size: 11px; color: #64748b; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .empty { padding: 20px; text-align: center; color: #475569; font-size: 13px; }
  .footer{padding:12px;border-top:1px solid #1e293b;flex-shrink:0}
  .settings-btn{width:100%;padding:8px;background:none;border:1px solid #334155;border-radius:6px;color:#64748b;font-size:13px;cursor:pointer}
  .settings-btn:hover{background:#1e293b;color:#e2e8f0}
</style>
