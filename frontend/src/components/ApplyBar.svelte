<script>
  import { createEventDispatcher } from 'svelte';
  const dispatch = createEventDispatcher();

  export let visible = false;
  export let reason = '';
</script>

{#if visible}
  <div class="bar" role="status">
    <span class="ico">◷</span>
    <span class="msg">
      {#if reason}{reason}{:else}Configuration saved — not active in this chat yet.{/if}
    </span>
    <button class="apply" on:click={() => dispatch('apply')}>Apply now</button>
    <button class="dismiss" on:click={() => dispatch('dismiss')} title="Dismiss">✕</button>
  </div>
{/if}

<style>
  .bar{display:flex;align-items:center;gap:10px;padding:8px 16px;background:linear-gradient(90deg,#1e1b4b,#0f172a);border-bottom:1px solid #4338ca;color:#e2e8f0;font-size:12px;flex-shrink:0}
  .ico{font-size:14px;color:#a5b4fc}
  .msg{flex:1;color:#c4b5fd}
  .apply{padding:6px 14px;border-radius:6px;border:none;background:#6366f1;color:#fff;font-size:12px;font-weight:600;cursor:pointer}
  .apply:hover{filter:brightness(1.1)}
  .dismiss{background:none;border:none;color:#64748b;cursor:pointer;font-size:14px;padding:4px 8px}
  .dismiss:hover{color:#e2e8f0}
</style>
