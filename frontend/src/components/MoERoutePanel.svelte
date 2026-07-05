<script>
  export let routes = [];

  const colors = {
    reasoning: '#a78bfa',
    speed: '#4ade80',
    longctx: '#fb923c',
    coding: '#38bdf8',
    default: '#64748b',
  };
</script>

{#if routes.length > 0}
  <div class="moe">
    <div class="title">MoE routing history</div>
    <div class="list">
      {#each routes as r, i}
        <div class="row">
          <span class="idx">{routes.length - i}</span>
          <span class="cat" style="background:{colors[r.category] || colors.default}">{r.category}</span>
          <span class="model">{r.model}</span>
          {#if r.ts}<span class="ts">{new Date(r.ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</span>{/if}
        </div>
      {/each}
    </div>
  </div>
{/if}

<style>
  .moe{padding:10px 14px;background:#1e1b4b;border-bottom:1px solid #4338ca}
  .title{font-size:10px;font-weight:700;text-transform:uppercase;color:#c4b5fd;margin-bottom:8px;letter-spacing:0.4px}
  .list{display:flex;flex-direction:column;gap:4px;max-height:100px;overflow-y:auto}
  .row{display:flex;align-items:center;gap:8px;font-size:11px}
  .idx{color:var(--accent);font-weight:700;width:16px}
  .cat{font-size:9px;font-weight:700;padding:2px 8px;border-radius:10px;color:#0f172a;text-transform:uppercase}
  .model{color:#e2e8f0;flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .ts{color:#818cf8;font-size:10px;flex-shrink:0}
</style>
