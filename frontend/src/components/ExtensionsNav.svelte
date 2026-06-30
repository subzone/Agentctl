<script>
  import { createEventDispatcher } from 'svelte';
  const dispatch = createEventDispatcher();
  export let active = 'tools';
</script>

<div class="ext-nav">
  <button class:active={active === 'tools'} on:click={() => dispatch('select', 'tools')}>
    <span class="ico">🛠</span> Tools
  </button>
  <button class:active={active === 'skills'} on:click={() => dispatch('select', 'skills')}>
    <span class="ico">✦</span> Skills
  </button>
  <button class:active={active === 'mcp'} on:click={() => dispatch('select', 'mcp')}>
    <span class="ico">🔌</span> MCP
  </button>
  <button class:active={active === 'agents'} on:click={() => dispatch('select', 'agents')}>
    <span class="ico">⬡</span> Agents
  </button>
</div>

<style>
  .ext-nav{display:flex;gap:2px;padding:8px 8px 0;flex-shrink:0;border-bottom:1px solid #1e293b}
  .ext-nav button{flex:1;display:flex;align-items:center;justify-content:center;gap:4px;padding:8px 4px;background:none;border:none;border-bottom:2px solid transparent;color:#64748b;font-size:10px;font-weight:600;cursor:pointer;border-radius:6px 6px 0 0}
  .ext-nav button:hover{color:#cbd5e1;background:#111c30}
  .ext-nav button.active{color:#e2e8f0;border-bottom-color:#6366f1;background:#0c1322}
  .ico{font-size:12px}
</style>
