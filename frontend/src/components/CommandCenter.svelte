<script>
  import { createEventDispatcher } from 'svelte';
  const dispatch = createEventDispatcher();

  export let agent = null;
  export let health = { ok: true, checks: [] };
  export let stats = { tools: 0, skills: 0, mcp: 0, km: false };
  export let moeRoute = null;
  export let sessions = [];

  function fmtTime(ts) {
    if (!ts) return '';
    const d = new Date(ts * 1000);
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  }

  function statusIcon(s) {
    if (s === 'ok') return '✓';
    if (s === 'warn') return '!';
    return '✗';
  }
  function statusClass(s) {
    if (s === 'ok') return 'ok';
    if (s === 'warn') return 'warn';
    return 'err';
  }
</script>

<div class="center">
  <header class="head">
    <div>
      <h1>Command Center</h1>
      <p class="sub">
        {#if agent}
          Active agent: <strong>{agent.name}</strong>
          <span class="model">{agent.model}</span>
        {:else}
          Select an agent to get started
        {/if}
      </p>
    </div>
    <div class="head-actions">
      <button class="act primary" on:click={() => dispatch('newChat')} disabled={!agent}>New chat</button>
      <button class="act" on:click={() => dispatch('refresh')}>Refresh health</button>
    </div>
  </header>

  <div class="grid">
    <button class="card" on:click={() => dispatch('nav', 'agents')}>
      <span class="card-ico">⬡</span>
      <span class="card-lbl">Chat</span>
      <span class="card-sub">Talk to {agent?.name || 'an agent'}</span>
    </button>
    <button class="card" on:click={() => dispatch('nav', 'knowledge')}>
      <span class="card-ico">◆</span>
      <span class="card-lbl">Knowledge</span>
      <span class="card-sub">{stats.km ? 'Graph online' : 'Start km serve'}</span>
    </button>
    <button class="card" on:click={() => dispatch('nav', 'ext-tools')}>
      <span class="card-ico">🛠</span>
      <span class="card-lbl">Tools</span>
      <span class="card-sub">{stats.tools} custom</span>
    </button>
    <button class="card" on:click={() => dispatch('nav', 'ext-skills')}>
      <span class="card-ico">✦</span>
      <span class="card-lbl">Skills</span>
      <span class="card-sub">{stats.skills} active</span>
    </button>
    <button class="card" on:click={() => dispatch('nav', 'ext-mcp')}>
      <span class="card-ico">🔌</span>
      <span class="card-lbl">MCP</span>
      <span class="card-sub">{stats.mcp} server(s)</span>
    </button>
    <button class="card" on:click={() => dispatch('nav', 'personality')}>
      <span class="card-ico">🎭</span>
      <span class="card-lbl">Persona</span>
      <span class="card-sub">Per-agent behaviour</span>
    </button>
  </div>

  {#if moeRoute}
    <div class="moe-banner">
      Last route: <strong>{moeRoute.category}</strong> → {moeRoute.model}
    </div>
  {/if}

  {#if sessions.length > 0}
    <section class="sessions">
      <h2>Active sessions</h2>
      <div class="sess-list">
        {#each sessions.slice(0, 5) as s}
          <button class="sess-row" on:click={() => dispatch('nav', 'agents')}>
            <span class="sess-agent">{s.agent}</span>
            <span class="sess-preview">{s.preview || 'New session'}</span>
            <span class="sess-meta">{s.messages} msg · {fmtTime(s.createdAt)}</span>
          </button>
        {/each}
      </div>
    </section>
  {/if}

  <section class="health">
    <div class="health-head">
      <h2>Setup health</h2>
      <span class="badge" class:ok={health.ok} class:bad={!health.ok}>{health.ok ? 'All good' : 'Needs attention'}</span>
    </div>
    <div class="checks">
      {#each health.checks as c}
        <button class="check" class:clickable={!!c.fixTab} on:click={() => c.fixTab && dispatch('nav', c.fixTab)}>
          <span class="ico {statusClass(c.status)}">{statusIcon(c.status)}</span>
          <div class="check-body">
            <div class="check-lbl">{c.label}</div>
            <div class="check-detail">{c.detail}</div>
          </div>
          {#if c.fixTab}<span class="fix">Fix →</span>{/if}
        </button>
      {/each}
    </div>
  </section>
</div>

<style>
  .center{flex:1;overflow-y:auto;padding:24px 28px;min-height:0}
  .head{display:flex;justify-content:space-between;align-items:flex-start;gap:16px;margin-bottom:24px}
  .head h1{margin:0;font-size:22px;font-weight:700;color:var(--text)}
  .sub{margin:6px 0 0;font-size:13px;color:var(--muted)}
  .model{margin-left:8px;font-size:11px;background:#0f172a;padding:2px 8px;border-radius:4px;color:#94a3b8}
  .head-actions{display:flex;gap:8px;flex-shrink:0}
  .act{padding:9px 16px;border-radius:8px;border:1px solid var(--border);background:var(--bg-input);color:var(--text);font-size:13px;font-weight:600;cursor:pointer}
  .act:hover{filter:brightness(1.1)}
  .act.primary{background:var(--accent);border-color:var(--accent);color:#fff}
  .act:disabled{opacity:0.5;cursor:not-allowed}

  .grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(140px,1fr));gap:10px;margin-bottom:20px}
  .card{display:flex;flex-direction:column;align-items:flex-start;gap:4px;padding:14px 16px;background:#0c1322;border:1px solid var(--border);border-radius:10px;cursor:pointer;text-align:left;color:var(--text)}
  .card:hover{border-color:var(--accent);background:#111c30}
  .card-ico{font-size:20px}
  .card-lbl{font-size:13px;font-weight:600}
  .card-sub{font-size:11px;color:var(--muted)}

  .moe-banner{padding:10px 14px;background:#1e1b4b;border:1px solid #4338ca;border-radius:8px;font-size:12px;color:#c4b5fd;margin-bottom:20px}

  .sessions{margin-bottom:20px}
  .sessions h2{margin:0 0 10px;font-size:14px;font-weight:700;color:var(--text)}
  .sess-list{display:flex;flex-direction:column;gap:6px}
  .sess-row{display:flex;flex-wrap:wrap;align-items:baseline;gap:8px;padding:10px 12px;background:#0c1322;border:1px solid var(--border);border-radius:8px;cursor:pointer;text-align:left;color:inherit;width:100%}
  .sess-row:hover{border-color:var(--accent)}
  .sess-agent{font-size:12px;font-weight:700;color:#c4b5fd}
  .sess-preview{flex:1;font-size:12px;color:var(--text);min-width:120px}
  .sess-meta{font-size:10px;color:var(--muted)}

  .health{background:#0c1322;border:1px solid var(--border);border-radius:12px;padding:16px 18px}
  .health-head{display:flex;align-items:center;gap:12px;margin-bottom:14px}
  .health-head h2{margin:0;font-size:14px;font-weight:700}
  .badge{font-size:11px;font-weight:700;padding:3px 10px;border-radius:20px;background:#7f1d1d;color:#fca5a5}
  .badge.ok{background:#14532d;color:#86efac}
  .checks{display:flex;flex-direction:column;gap:6px}
  .check{display:flex;align-items:center;gap:12px;padding:10px 12px;border-radius:8px;background:#080d18;border:1px solid transparent;width:100%;text-align:left;color:inherit}
  .check.clickable{cursor:pointer}
  .check.clickable:hover{border-color:var(--border)}
  .ico{width:22px;height:22px;border-radius:50%;display:flex;align-items:center;justify-content:center;font-size:11px;font-weight:700;flex-shrink:0}
  .ico.ok{background:#14532d;color:#86efac}
  .ico.warn{background:#713f12;color:#fcd34d}
  .ico.err{background:#7f1d1d;color:#fca5a5}
  .check-body{flex:1;min-width:0}
  .check-lbl{font-size:13px;font-weight:600}
  .check-detail{font-size:11px;color:var(--muted);margin-top:2px}
  .fix{font-size:11px;color:var(--accent);flex-shrink:0}
</style>
