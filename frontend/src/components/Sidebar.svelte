<script>
  import { createEventDispatcher } from 'svelte';
  
  export let agents = [];
  export let sessions = [];
  
  const dispatch = createEventDispatcher();
  
  let searchQuery = '';
  let activeTab = 'agents'; // 'agents' | 'sessions'
  
  $: filteredAgents = agents.filter(a => {
    if (!searchQuery) return a.category !== 'spoke'; // hide spokes by default
    const q = searchQuery.toLowerCase();
    return a.name.toLowerCase().includes(q) || 
           (a.description || '').toLowerCase().includes(q);
  });
  
  $: hubAgents = filteredAgents.filter(a => a.category === 'hub');
  $: standaloneAgents = filteredAgents.filter(a => a.category === 'standalone');
</script>

<aside class="sidebar">
  <div class="tabs">
    <button class:active={activeTab === 'agents'} on:click={() => activeTab = 'agents'}>
      Agents
    </button>
    <button class:active={activeTab === 'sessions'} on:click={() => activeTab = 'sessions'}>
      Sessions
    </button>
  </div>
  
  {#if activeTab === 'agents'}
    <div class="search">
      <input 
        type="text" 
        placeholder="Search agents..." 
        bind:value={searchQuery}
      />
    </div>
    
    <div class="list">
      {#if hubAgents.length > 0}
        <div class="section-label">Hub Agents</div>
        {#each hubAgents as agent}
          <button class="item" on:click={() => dispatch('newChat', agent)}>
            <span class="item-icon">🔀</span>
            <div class="item-info">
              <span class="item-name">{agent.name}</span>
              <span class="item-desc">{agent.model}</span>
            </div>
          </button>
        {/each}
      {/if}
      
      {#if standaloneAgents.length > 0}
        <div class="section-label">Agents</div>
        {#each standaloneAgents as agent}
          <button class="item" on:click={() => dispatch('newChat', agent)}>
            <span class="item-icon">{agent.builtin ? '📦' : '📄'}</span>
            <div class="item-info">
              <span class="item-name">{agent.name}</span>
              <span class="item-desc">{agent.model}</span>
            </div>
          </button>
        {/each}
      {/if}
    </div>
  {:else}
    <div class="list">
      {#if sessions.length === 0}
        <div class="empty">No sessions yet</div>
      {/if}
      {#each sessions as session}
        <button class="item" on:click={() => dispatch('resumeSession', session)}>
          <span class="item-icon">💬</span>
          <div class="item-info">
            <span class="item-name">{session.agent}</span>
            <span class="item-desc">{session.preview || `${session.messages} messages`}</span>
          </div>
        </button>
      {/each}
    </div>
  {/if}
</aside>

<style>
  .sidebar {
    width: 260px;
    background-color: #0f172a;
    border-right: 1px solid #1e293b;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  
  .tabs {
    display: flex;
    border-bottom: 1px solid #1e293b;
  }
  
  .tabs button {
    flex: 1;
    padding: 10px;
    background: none;
    border: none;
    color: #64748b;
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
    border-bottom: 2px solid transparent;
    transition: all 0.2s;
  }
  
  .tabs button.active {
    color: #e2e8f0;
    border-bottom-color: #3b82f6;
  }
  
  .search {
    padding: 8px 12px;
  }
  
  .search input {
    width: 100%;
    padding: 8px 12px;
    background-color: #1e293b;
    border: 1px solid #334155;
    border-radius: 6px;
    color: #e2e8f0;
    font-size: 13px;
    outline: none;
  }
  
  .search input:focus {
    border-color: #3b82f6;
  }
  
  .list {
    flex: 1;
    overflow-y: auto;
    padding: 4px 8px;
  }
  
  .section-label {
    font-size: 11px;
    text-transform: uppercase;
    color: #475569;
    padding: 12px 8px 4px;
    font-weight: 600;
    letter-spacing: 0.5px;
  }
  
  .item {
    width: 100%;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 10px;
    background: none;
    border: none;
    border-radius: 6px;
    color: #e2e8f0;
    cursor: pointer;
    transition: background-color 0.15s;
    text-align: left;
  }
  
  .item:hover {
    background-color: #1e293b;
  }
  
  .item-icon {
    font-size: 16px;
    flex-shrink: 0;
  }
  
  .item-info {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  
  .item-name {
    font-size: 13px;
    font-weight: 500;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  
  .item-desc {
    font-size: 11px;
    color: #64748b;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  
  .empty {
    padding: 20px;
    text-align: center;
    color: #475569;
    font-size: 13px;
  }
</style>
