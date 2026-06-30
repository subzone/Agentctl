<script>
  import { createEventDispatcher } from 'svelte';
  const dispatch = createEventDispatcher();

  export let agents = [];
  export let activeName = null;
</script>

<aside class="aside">
  <div class="head">
    <span class="lbl">Agents</span>
    <button class="mini" title="New agent" on:click={() => dispatch('new')}>＋</button>
  </div>
  <div class="list">
    {#each agents as a}
      <button class="row" class:active={activeName === a.name} on:click={() => dispatch('select', a)}>
        <span class="ico">{a.builtin ? '◆' : '⬡'}</span>
        <div class="info">
          <div class="name">
            {a.name}
            {#if a.builtin}<span class="tag">built-in</span>{/if}
            {#if a.error}<span class="err">!</span>{/if}
          </div>
          <div class="desc">{a.description || a.model || 'agent'}</div>
        </div>
      </button>
    {/each}
  </div>
</aside>

<style>
  .aside{width:100%;flex:1;min-height:0;display:flex;flex-direction:column;overflow:hidden;padding:8px}
  .head{display:flex;justify-content:space-between;align-items:center;padding:4px 6px}
  .lbl{font-size:10px;font-weight:700;color:#475569;text-transform:uppercase}
  .mini{background:#1e293b;border:1px solid #334155;border-radius:6px;color:#e2e8f0;cursor:pointer;font-size:14px;width:26px;height:26px}
  .list{flex:1;overflow-y:auto;display:flex;flex-direction:column;gap:2px;margin-top:4px}
  .row{width:100%;display:flex;align-items:center;gap:8px;padding:8px 10px;background:none;border:none;border-radius:6px;color:#e2e8f0;cursor:pointer;text-align:left}
  .row:hover{background:#1e293b}
  .row.active{background:#1e293b;border-left:2px solid #6366f1}
  .ico{font-size:14px;opacity:0.8}
  .info{min-width:0;flex:1}
  .name{font-size:13px;font-weight:600;display:flex;align-items:center;gap:6px}
  .tag{font-size:9px;font-weight:700;color:#64748b;text-transform:uppercase}
  .err{color:#f87171}
  .desc{font-size:11px;color:#64748b;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
</style>
