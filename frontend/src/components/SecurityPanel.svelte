<script>
  import { onMount, createEventDispatcher } from 'svelte';
  const dispatch = createEventDispatcher();

  let rules = [];
  let auditCfg = { backend: 'none', path: '', active: false };
  let events = [];
  let loading = true;
  let tab = 'audit';

  onMount(refresh);

  async function refresh() {
    loading = true;
    try {
      rules = await window.go.desktop.App.GetPolicyRules() || [];
      auditCfg = await window.go.desktop.App.GetAuditConfig() || auditCfg;
      events = await window.go.desktop.App.ListAuditEvents(40) || [];
    } catch (_) {}
    loading = false;
  }

  async function openAuditPath() {
    if (auditCfg.path) {
      try { await window.go.desktop.App.OpenPath(auditCfg.path); } catch (_) {}
    }
  }
</script>

<div class="sec">
  <header class="head">
    <h2>Security</h2>
    <button class="link" on:click={refresh} disabled={loading}>{loading ? '…' : '↻ Refresh'}</button>
  </header>

  <div class="tabs">
    <button class:act={tab === 'audit'} on:click={() => tab = 'audit'}>Audit log</button>
    <button class:act={tab === 'policy'} on:click={() => tab = 'policy'}>Policy rules</button>
  </div>

  {#if tab === 'audit'}
    <div class="meta">
      Backend: <strong>{auditCfg.backend}</strong>
      {#if auditCfg.path}
        · <code>{auditCfg.path}</code>
        <button class="link" on:click={openAuditPath}>Open folder</button>
      {/if}
    </div>
    {#if events.length === 0}
      <p class="empty">No audit events yet. Enable <code>audit:</code> in <code>~/.config/m/config.yaml</code>.</p>
    {:else}
      <div class="elist">
        {#each [...events].reverse() as ev}
          <div class="erow">
            <span class="etype">{ev.type}</span>
            {#if ev.tool}<span class="etool">{ev.tool}</span>{/if}
            {#if ev.outcome}<span class="eout {ev.outcome}">{ev.outcome}</span>{/if}
            {#if ev.durationMs}<span class="edur">{ev.durationMs}ms</span>{/if}
            {#if ev.error}<span class="eerr">{ev.error}</span>{/if}
          </div>
        {/each}
      </div>
    {/if}
  {:else}
    {#if rules.length === 0}
      <p class="empty">No policy rules loaded. Add <code>policy.rules</code> to your config YAML.</p>
    {:else}
      <div class="rules">
        {#each rules as r}
          <div class="rule">
            <div class="rname">{r.name || 'unnamed'}</div>
            {#if r.tool}<div class="rdetail">tool: {r.tool}</div>{/if}
            {#if r.denyPattern}<div class="rdetail">deny: <code>{r.denyPattern}</code></div>{/if}
            {#if r.message}<div class="rdetail">{r.message}</div>{/if}
          </div>
        {/each}
      </div>
    {/if}
  {/if}
</div>

<style>
  .sec{flex:1;overflow-y:auto;padding:20px 24px}
  .head{display:flex;justify-content:space-between;align-items:center;margin-bottom:16px}
  .head h2{margin:0;font-size:18px;font-weight:700}
  .link{background:none;border:none;color:var(--accent);cursor:pointer;font-size:12px}
  .tabs{display:flex;gap:6px;margin-bottom:14px}
  .tabs button{padding:8px 14px;border-radius:8px;border:1px solid var(--border);background:var(--bg-input);color:var(--muted);cursor:pointer;font-size:12px;font-weight:600}
  .tabs button.act{border-color:var(--accent);color:var(--text)}
  .meta{font-size:12px;color:var(--muted);margin-bottom:12px}
  .meta code{font-size:11px;background:#080d18;padding:2px 6px;border-radius:4px}
  .empty{font-size:13px;color:var(--muted);line-height:1.6}
  .empty code{background:#080d18;padding:1px 5px;border-radius:4px;font-size:11px}
  .elist{display:flex;flex-direction:column;gap:4px;max-height:480px;overflow-y:auto}
  .erow{display:flex;flex-wrap:wrap;gap:8px;align-items:center;padding:8px 10px;background:#0c1322;border-radius:8px;font-size:11px}
  .etype{font-weight:700;color:#a78bfa;text-transform:uppercase;font-size:10px}
  .etool{font-weight:600;color:var(--text)}
  .eout{font-size:10px;padding:2px 6px;border-radius:4px;background:#1e293b}
  .eout.success{color:#86efac}
  .eout.error,.eout.policy_denied{color:#fca5a5}
  .edur{font-family:monospace;color:var(--muted)}
  .eerr{color:#fca5a5;flex:1;min-width:100%}
  .rules{display:flex;flex-direction:column;gap:8px}
  .rule{padding:12px 14px;background:#0c1322;border:1px solid var(--border);border-radius:10px}
  .rname{font-weight:700;font-size:13px;margin-bottom:4px}
  .rdetail{font-size:12px;color:var(--muted);margin-top:2px}
  .rdetail code{font-size:11px}
</style>
