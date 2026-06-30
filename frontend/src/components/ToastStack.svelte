<script>
  export let toasts = [];
</script>

<div class="toasts">
  {#each toasts as t (t.id)}
    <div class="toast" class:info={t.type === 'info'} class:ok={t.type === 'ok'} class:warn={t.type === 'warn'}>
      {t.message}
    </div>
  {/each}
</div>

<style>
  .toasts{position:fixed;bottom:20px;right:20px;z-index:90;display:flex;flex-direction:column;gap:8px;pointer-events:none}
  .toast{padding:10px 16px;border-radius:8px;font-size:13px;font-weight:500;background:#1e293b;border:1px solid var(--border);color:var(--text);box-shadow:0 8px 24px rgba(0,0,0,0.35);max-width:360px}
  .toast.ok{border-color:#22c55e;background:#14532d;color:#86efac}
  .toast.warn{border-color:#f59e0b;background:#78350f;color:#fcd34d}
  .toast.info{border-color:var(--accent)}
</style>
