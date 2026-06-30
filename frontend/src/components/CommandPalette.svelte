<script>
  import { createEventDispatcher, onMount } from 'svelte';
  const dispatch = createEventDispatcher();

  export let open = false;
  export let items = [];

  let filter = '';
  let active = 0;

  $: visible = filter.trim()
    ? items.filter(i => i.label.toLowerCase().includes(filter.toLowerCase()))
    : items;

  function pick(i) {
    dispatch('select', i.action);
    open = false;
    filter = '';
    active = 0;
  }

  function onKey(e) {
    if (!open) return;
    if (e.key === 'Escape') { open = false; e.preventDefault(); }
    if (e.key === 'ArrowDown') { active = Math.min(active + 1, visible.length - 1); e.preventDefault(); }
    if (e.key === 'ArrowUp') { active = Math.max(active - 1, 0); e.preventDefault(); }
    if (e.key === 'Enter' && visible[active]) { pick(visible[active]); e.preventDefault(); }
  }

  onMount(() => {
    const h = (e) => onKey(e);
    window.addEventListener('keydown', h);
    return () => window.removeEventListener('keydown', h);
  });

  $: if (open) active = 0;
</script>

{#if open}
  <div class="overlay" on:click|self={() => open = false} role="presentation">
    <div class="palette">
      <input class="search" bind:value={filter} placeholder="Type a command…" autofocus />
      <div class="list">
        {#each visible as item, i}
          <button class="item" class:active={i === active} on:click={() => pick(item)}>
            <span class="ico">{item.icon || '›'}</span>
            <span class="lbl">{item.label}</span>
            {#if item.hint}<span class="hint">{item.hint}</span>{/if}
          </button>
        {/each}
        {#if visible.length === 0}
          <div class="empty">No matches</div>
        {/if}
      </div>
      <div class="foot">↑↓ navigate · Enter select · Esc close</div>
    </div>
  </div>
{/if}

<style>
  .overlay{position:fixed;inset:0;background:rgba(2,6,23,0.55);backdrop-filter:blur(3px);display:flex;align-items:flex-start;justify-content:center;padding-top:12vh;z-index:100}
  .palette{width:480px;max-width:92vw;background:#0f172a;border:1px solid var(--border);border-radius:12px;overflow:hidden;box-shadow:0 24px 80px rgba(0,0,0,0.5)}
  .search{width:100%;box-sizing:border-box;padding:14px 16px;background:#080d18;border:none;border-bottom:1px solid var(--border);color:var(--text);font-size:15px;outline:none}
  .list{max-height:320px;overflow-y:auto;padding:6px}
  .item{width:100%;display:flex;align-items:center;gap:10px;padding:10px 12px;background:none;border:none;border-radius:8px;color:var(--text);cursor:pointer;text-align:left}
  .item:hover,.item.active{background:#1e293b}
  .ico{width:20px;text-align:center;flex-shrink:0}
  .lbl{flex:1;font-size:14px}
  .hint{font-size:11px;color:var(--muted)}
  .empty{padding:16px;text-align:center;color:var(--muted);font-size:13px}
  .foot{padding:8px 14px;font-size:10px;color:var(--muted);border-top:1px solid var(--border)}
</style>
