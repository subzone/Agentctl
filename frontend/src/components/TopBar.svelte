<script>
  import { createEventDispatcher } from 'svelte';
  export let agent = null;
  export let cost = { inputTokens: 0, outputTokens: 0, cost: 0, model: '' };
  
  const dispatch = createEventDispatcher();
  
  function formatTokens(n) {
    if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M';
    if (n >= 1000) return (n / 1000).toFixed(1) + 'k';
    return n.toString();
  }
  
  function formatCost(c) {
    if (c < 0.01) return '$' + c.toFixed(4);
    return '$' + c.toFixed(2);
  }
</script>

<header class="topbar">
  <div class="left">
    <span class="logo">◆</span>
    <button class="agent-name" on:click={() => dispatch('switchAgent', agent)}>
      {agent ? agent.name : 'No agent'}
    </button>
    {#if agent}
      <span class="model">{agent.model || cost.model}</span>
    {/if}
  </div>
  
  <div class="stats">
    <div class="stat">
      <span class="stat-label">In</span>
      <span class="stat-value">{formatTokens(cost.inputTokens)}</span>
    </div>
    <div class="stat">
      <span class="stat-label">Out</span>
      <span class="stat-value">{formatTokens(cost.outputTokens)}</span>
    </div>
    <div class="stat cost">
      <span class="stat-label">Cost</span>
      <span class="stat-value">{formatCost(cost.cost)}</span>
    </div>
  </div>
  
  <div class="right">
    <button class="icon-btn" on:click={() => dispatch('settings')} title="Settings">
      ⚙️
    </button>
  </div>
</header>

<style>
  .topbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 16px;
    background-color: #1e293b;
    border-bottom: 1px solid #334155;
    height: 44px;
    -webkit-app-region: drag;
  }
  
  .left {
    display: flex;
    align-items: center;
    gap: 10px;
    -webkit-app-region: no-drag;
  }
  
  .logo {
    font-size: 20px;
    color: #3b82f6;
  }
  
  .agent-name {
    background: none;
    border: 1px solid #475569;
    border-radius: 6px;
    color: #f1f5f9;
    font-size: 14px;
    font-weight: 600;
    padding: 4px 12px;
    cursor: pointer;
    transition: all 0.2s;
  }
  
  .agent-name:hover {
    border-color: #3b82f6;
    background-color: #1e3a5f;
  }
  
  .model {
    font-size: 12px;
    color: #64748b;
  }
  
  .stats {
    display: flex;
    gap: 16px;
    -webkit-app-region: no-drag;
  }
  
  .stat {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1px;
  }
  
  .stat-label {
    font-size: 10px;
    color: #64748b;
    text-transform: uppercase;
  }
  
  .stat-value {
    font-size: 13px;
    font-weight: 500;
    color: #94a3b8;
    font-family: 'SF Mono', 'Fira Code', monospace;
  }
  
  .cost .stat-value {
    color: #4ade80;
  }
  
  .right {
    -webkit-app-region: no-drag;
  }
  
  .icon-btn {
    background: none;
    border: none;
    font-size: 18px;
    cursor: pointer;
    padding: 4px 8px;
    border-radius: 4px;
    transition: background-color 0.2s;
  }
  
  .icon-btn:hover {
    background-color: #334155;
  }
</style>
