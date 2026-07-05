<script>
  import { createEventDispatcher } from 'svelte';
  const dispatch = createEventDispatcher();

  export let tools = [];
  export let activeName = null;
</script>

<aside class="tside">
  <div class="thead">
    <span class="group-label">Custom tools</span>
    <button class="mini" title="New tool" on:click={() => dispatch('new')}>＋</button>
  </div>

  <div class="tlist">
    {#each tools as t}
      <button class="titem" class:active={activeName === t.name} on:click={() => dispatch('select', t)}>
        <span class="ticon">🛠</span>
        <div class="tinfo">
          <div class="tname">
            {t.name}
            {#if t.error}<span class="terr" title={t.error}>!</span>{/if}
          </div>
          <div class="tdesc">{t.description || t.runtime || 'shell tool'}</div>
        </div>
      </button>
    {/each}
    {#if tools.length === 0}
      <div class="tempty">No custom tools yet.<br />Create one to extend the agent.</div>
    {/if}
  </div>

</aside>

<style>
  .tside{width:100%;flex:1;min-height:0;display:flex;flex-direction:column;overflow:hidden;padding:8px}
  .thead{display:flex;justify-content:space-between;align-items:center;padding:4px 6px}
  .group-label{font-size:10px;font-weight:700;color:#475569;text-transform:uppercase;letter-spacing:0.5px}
  .mini{background:#1e293b;border:1px solid #334155;border-radius:6px;color:#e2e8f0;cursor:pointer;font-size:14px;width:26px;height:26px;line-height:1}
  .mini:hover{background:#334155}

  .tlist{flex:1;overflow-y:auto;display:flex;flex-direction:column;gap:2px;margin-top:4px}
  .titem{width:100%;display:flex;align-items:center;gap:8px;padding:8px 10px;background:none;border:none;border-radius:6px;color:#e2e8f0;cursor:pointer;text-align:left}
  .titem:hover{background:#1e293b}
  .titem.active{background:#1e293b;border-left:2px solid var(--accent)}
  .ticon{font-size:15px}
  .tinfo{min-width:0}
  .tname{font-size:13px;font-weight:500;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;display:flex;align-items:center;gap:6px}
  .terr{display:inline-flex;align-items:center;justify-content:center;width:14px;height:14px;border-radius:50%;background:#7f1d1d;color:#fca5a5;font-size:10px;font-weight:700}
  .tdesc{font-size:11px;color:#64748b;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .tempty{padding:18px 12px;text-align:center;color:#475569;font-size:12px;line-height:1.5}

  .footer{padding:10px 6px 4px;border-top:1px solid #1e293b;flex-shrink:0}
  .settings-btn{width:100%;padding:8px;background:none;border:1px solid #334155;border-radius:6px;color:#64748b;font-size:13px;cursor:pointer}
  .settings-btn:hover{background:#1e293b;color:#e2e8f0}
</style>
