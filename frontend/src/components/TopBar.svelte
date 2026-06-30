<script>
  import { createEventDispatcher } from 'svelte';
  export let agent = null;
  export let cost = { inputTokens: 0, outputTokens: 0, cost: 0 };
  export let moeRoute = null;
  export let contextUsage = 0;
  const dispatch = createEventDispatcher();

  function fmt(n) { return (n||0) >= 1000 ? ((n||0)/1000).toFixed(1)+'k' : String(n||0); }
  function fmtCost(c) {
    if (!c) return '$0.00';
    return c < 0.01 ? '$'+c.toFixed(4) : '$'+c.toFixed(2);
  }

  function moeColor(cat) {
    if (cat === 'reasoning') return '#a78bfa';
    if (cat === 'speed') return '#4ade80';
    if (cat === 'longctx') return '#fb923c';
    return '#64748b';
  }
</script>

<header class="topbar">
  <div class="drag-region"></div>

  <div class="left">
    <svg class="logo" viewBox="0 0 32 32" width="22" height="22">
      <rect width="32" height="32" rx="6" fill="#4f46e5"/>
      <rect x="4" y="4" width="6" height="24" rx="1" fill="#fff"/>
      <rect x="22" y="4" width="6" height="24" rx="1" fill="#fff"/>
      <rect x="10" y="4" width="6" height="6" rx="1" fill="#fff"/>
      <rect x="12" y="10" width="4" height="4" rx="1" fill="#fff"/>
      <rect x="16" y="4" width="6" height="6" rx="1" fill="#fff"/>
      <rect x="16" y="10" width="4" height="4" rx="1" fill="#fff"/>
      <rect x="14" y="12" width="4" height="4" rx="1" fill="#fff"/>
    </svg>
    <span class="title">AgentCTL</span>
    {#if agent}
      <span class="sep">›</span>
      <span class="agent-name">{agent.name}</span>
      <span class="model">{agent.model}</span>
    {/if}
    {#if moeRoute}
      <span class="moe-tag" style="background:{moeColor(moeRoute.category)}">
        {moeRoute.category} → {moeRoute.model}
      </span>
    {/if}
  </div>

  <div class="right">
    {#if agent}
      {#if contextUsage > 0}
        <div class="ctx-bar" title="Context: {contextUsage}%">
          <div class="ctx-fill" style="width:{contextUsage}%;background:{contextUsage > 80 ? '#ef4444' : contextUsage > 50 ? '#f59e0b' : '#4ade80'}"></div>
          <span class="ctx-label">{contextUsage}%</span>
        </div>
      {/if}
      <div class="tokens">
        <span class="tok-label">In</span><span class="tok-val">{fmt(cost.inputTokens)}</span>
        <span class="tok-sep">·</span>
        <span class="tok-label">Out</span><span class="tok-val">{fmt(cost.outputTokens)}</span>
        <span class="tok-sep">·</span>
        <span class="tok-cost">{fmtCost(cost.cost)}</span>
      </div>
    {/if}
    <button class="settings-btn" on:click={() => dispatch('settings')} title="Settings">
      <svg viewBox="0 0 20 20" width="16" height="16" fill="currentColor">
        <path fill-rule="evenodd" d="M11.49 3.17c-.38-1.56-2.6-1.56-2.98 0a1.532 1.532 0 01-2.286.948c-1.372-.836-2.942.734-2.106 2.106.54.886.061 2.042-.947 2.287-1.561.379-1.561 2.6 0 2.978a1.532 1.532 0 01.947 2.287c-.836 1.372.734 2.942 2.106 2.106a1.532 1.532 0 012.287.947c.379 1.561 2.6 1.561 2.978 0a1.533 1.533 0 012.287-.947c1.372.836 2.942-.734 2.106-2.106a1.533 1.533 0 01.947-2.287c1.561-.379 1.561-2.6 0-2.978a1.532 1.532 0 01-.947-2.287c.836-1.372-.734-2.942-2.106-2.106a1.532 1.532 0 01-2.287-.947zM10 13a3 3 0 100-6 3 3 0 000 6z" clip-rule="evenodd"/>
      </svg>
    </button>
  </div>
</header>

<style>
  .topbar {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 52px;
    padding: 0 12px;
    padding-top: 28px;
    padding-left: 80px;
    background: #1e293b;
    border-bottom: 1px solid #334155;
    flex-shrink: 0;
    gap: 12px;
  }

  .drag-region {
    position: absolute;
    inset: 0;
    -webkit-app-region: drag;
  }

  .left, .right {
    position: relative;
    z-index: 1;
    display: flex;
    align-items: center;
    gap: 8px;
    -webkit-app-region: no-drag;
    min-width: 0;
  }

  .left { flex: 1; overflow: hidden; }
  .right { flex-shrink: 0; }

  .logo { flex-shrink: 0; }
  .title { font-size: 14px; font-weight: 700; color: #f1f5f9; flex-shrink: 0; }
  .sep { color: #475569; flex-shrink: 0; }
  .agent-name {
    font-size: 13px; font-weight: 600; color: #93c5fd;
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
    max-width: 160px;
  }
  .model {
    font-size: 11px; color: #64748b; background: #0f172a;
    padding: 2px 7px; border-radius: 4px; white-space: nowrap;
    flex-shrink: 0; max-width: 180px; overflow: hidden; text-overflow: ellipsis;
  }

  .moe-tag{font-size:10px;color:#fff;padding:2px 8px;border-radius:4px;font-weight:600;white-space:nowrap;flex-shrink:0}

  .ctx-bar{position:relative;width:60px;height:8px;background:#0f172a;border-radius:4px;border:1px solid #334155;overflow:hidden;flex-shrink:0}
  .ctx-fill{height:100%;border-radius:3px;transition:width 0.3s}
  .ctx-label{position:absolute;inset:0;display:flex;align-items:center;justify-content:center;font-size:7px;font-weight:700;color:#e2e8f0}

  .tokens {
    display: flex; align-items: center; gap: 5px;
    font-size: 12px; font-family: 'SF Mono', Menlo, monospace;
  }
  .tok-label { color: #475569; font-size: 10px; text-transform: uppercase; }
  .tok-val { color: #94a3b8; }
  .tok-sep { color: #334155; }
  .tok-cost { color: #4ade80; font-weight: 600; }

  .settings-btn {
    background: none; border: 1px solid #334155; border-radius: 6px;
    color: #64748b; padding: 5px 7px; cursor: pointer; display: flex;
    align-items: center; transition: all 0.15s;
  }
  .settings-btn:hover { background: #334155; color: #e2e8f0; }
</style>
