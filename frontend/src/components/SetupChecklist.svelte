<script>
  import { createEventDispatcher, onMount } from 'svelte';
  const dispatch = createEventDispatcher();

  export let health = { ok: true, checks: [] };
  export let proMode = true;
  export let hasChatted = false;

  let updateInfo = null;

  onMount(async () => {
    try {
      updateInfo = await window.go.desktop.App.CheckForUpdate();
    } catch (_) {}
  });

  $: steps = [
    { id: 'keys', label: 'API keys configured', done: health.checks?.some(c => c.id === 'free_moe' && c.status === 'ok') || health.checks?.some(c => c.id === 'api_key' && c.status === 'ok') },
    { id: 'chat', label: 'Start your first chat', done: hasChatted, action: 'agents' },
    { id: 'tool', label: 'Create a custom tool (optional)', done: false, action: 'ext-tools', pro: true },
  ];

  $: doneCount = steps.filter(s => s.done).length;
</script>

<div class="checklist">
  <div class="head">
    <h3>Getting started</h3>
    <span class="prog">{doneCount}/{steps.length}</span>
  </div>
  <div class="steps">
    {#each steps as s}
      {#if !s.pro || proMode}
        <button class="step" class:done={s.done} disabled={s.done} on:click={() => s.action && dispatch('nav', s.action)}>
          <span class="ico">{s.done ? '✓' : '○'}</span>
          <span class="lbl">{s.label}</span>
          {#if !s.done && s.action}<span class="go">→</span>{/if}
        </button>
      {/if}
    {/each}
  </div>
  {#if updateInfo?.updateAvailable}
    <div class="update">
      Update available: v{updateInfo.current} → v{updateInfo.latest}
      <a href={updateInfo.releaseUrl} target="_blank" rel="noreferrer">Download</a>
    </div>
  {/if}
</div>

<style>
  .checklist{background:#0c1322;border:1px solid var(--border);border-radius:12px;padding:14px 16px;margin-bottom:16px}
  .head{display:flex;justify-content:space-between;align-items:center;margin-bottom:10px}
  .head h3{margin:0;font-size:13px;font-weight:700;color:var(--text)}
  .prog{font-size:11px;color:var(--muted);background:#1e293b;padding:2px 8px;border-radius:10px}
  .steps{display:flex;flex-direction:column;gap:6px}
  .step{display:flex;align-items:center;gap:10px;padding:8px 10px;border-radius:8px;border:1px solid transparent;background:#080d18;width:100%;text-align:left;color:var(--text);cursor:pointer;font-size:12px}
  .step:not(.done):hover{border-color:var(--border)}
  .step.done{opacity:0.65;cursor:default}
  .ico{width:18px;text-align:center;color:#6366f1;font-weight:700}
  .step.done .ico{color:#86efac}
  .lbl{flex:1}
  .go{color:var(--accent);font-size:11px}
  .update{margin-top:12px;padding:8px 10px;background:#1e1b4b;border-radius:8px;font-size:11px;color:#c4b5fd}
  .update a{color:#818cf8;margin-left:8px}
</style>
