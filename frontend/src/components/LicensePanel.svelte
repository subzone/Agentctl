<script>
  import { onMount } from 'svelte';

  let info = { plan: 'free', entitlements: ['free'], packages: [], isPro: false };
  let licenseKey = '';
  let activating = false;
  let error = '';
  let ok = '';

  let offers = [];
  let installing = '';

  onMount(async () => {
    await refresh();
  });

  async function refresh() {
    try {
      info = await window.go.desktop.App.GetEntitlement();
      offers = await window.go.desktop.App.ListPackageOffers() || [];
    } catch (e) {
      error = String(e);
    }
  }

  async function activate() {
    if (!licenseKey.trim()) return;
    activating = true;
    error = '';
    ok = '';
    try {
      info = await window.go.desktop.App.ActivateLicense(licenseKey.trim());
      licenseKey = '';
      ok = `Activated ${info.plan} plan`;
      offers = await window.go.desktop.App.ListPackageOffers() || [];
    } catch (e) {
      error = String(e);
    } finally {
      activating = false;
    }
  }

  async function install(name) {
    installing = name;
    error = '';
    ok = '';
    try {
      await window.go.desktop.App.InstallPackage(name);
      ok = `Installed ${name}`;
      offers = await window.go.desktop.App.ListPackageOffers() || [];
      info = await window.go.desktop.App.GetEntitlement();
    } catch (e) {
      error = String(e);
    } finally {
      installing = '';
    }
  }
</script>

<div class="license-panel">
  <div class="plan-row">
    <span class="plan-badge" class:pro={info.isPro}>{info.plan}</span>
    {#if info.licenseHint}<span class="hint-key">{info.licenseHint}</span>{/if}
  </div>
  <p class="hint">Entitlements: {info.entitlements?.join(', ') || 'free'}</p>

  {#if !info.isPro}
    <label class="field">
      <span>License key</span>
      <input type="text" bind:value={licenseKey} placeholder="AGENTCTL-PRO-DEV-2026" autocomplete="off" />
    </label>
    <button class="btn primary" disabled={activating || !licenseKey.trim()} on:click={activate}>
      {activating ? 'Activating…' : 'Activate license'}
    </button>
    <p class="dev-hint">Dev test keys: AGENTCTL-PRO-DEV-2026 · TEAM · ENTERPRISE</p>
  {/if}

  {#if offers.length > 0}
    <h4>Curated packages</h4>
    <div class="pkg-list">
      {#each offers as o}
        <div class="pkg-card" class:locked={o.locked}>
          <div class="pkg-top">
            <strong>{o.name}</strong>
            {#if o.installed}<span class="tag ok">installed</span>
            {:else if o.locked}<span class="tag lock">Pro</span>
            {:else}<span class="tag ok">available</span>{/if}
          </div>
          <p>{o.description}</p>
          {#if !o.installed}
            <button class="btn sm" disabled={o.locked || installing === o.name} on:click={() => install(o.name)}>
              {installing === o.name ? '…' : 'Install'}
            </button>
          {/if}
        </div>
      {/each}
    </div>
  {/if}

  {#if error}<div class="err">{error}</div>{/if}
  {#if ok}<div class="ok">{ok}</div>{/if}
</div>

<style>
  .license-panel{margin-bottom:20px;padding:14px 16px;background:var(--bg);border:1px solid var(--border);border-radius:10px}
  .plan-row{display:flex;align-items:center;gap:10px;margin-bottom:6px}
  .plan-badge{font-size:11px;font-weight:800;text-transform:uppercase;padding:4px 10px;border-radius:6px;background:var(--bg-input);color:var(--muted)}
  .plan-badge.pro{background:var(--accent-gradient);color:#fff}
  .hint-key{font-size:11px;color:var(--muted);font-family:'JetBrains Mono',monospace}
  .hint{font-size:12px;color:var(--muted);margin-bottom:12px}
  .field{display:flex;flex-direction:column;gap:4px;margin-bottom:10px}
  .field span{font-size:11px;font-weight:600;color:var(--muted);text-transform:uppercase}
  .field input{padding:8px 10px;background:var(--bg-input);border:1px solid var(--border);border-radius:6px;color:var(--text);font-size:13px}
  .btn{padding:8px 14px;border-radius:6px;border:1px solid var(--border);background:var(--bg-input);color:var(--text);font-size:12px;font-weight:600;cursor:pointer;transition:transform 0.15s,box-shadow 0.15s}
  .btn.primary{background:var(--accent-gradient);border-color:transparent;color:#fff;box-shadow:0 4px 14px rgba(79,70,229,0.35)}
  .btn.primary:hover:not(:disabled){transform:translateY(-1px);box-shadow:0 6px 18px rgba(79,70,229,0.45)}
  .btn:disabled{opacity:0.5;cursor:not-allowed}
  .btn.sm{padding:5px 10px;font-size:11px;margin-top:8px}
  .dev-hint{font-size:11px;color:var(--muted);margin-top:8px}
  h4{font-size:13px;font-weight:700;color:var(--text);margin:16px 0 8px}
  .pkg-list{display:flex;flex-direction:column;gap:8px}
  .pkg-card{padding:10px 12px;background:var(--bg-input);border:1px solid var(--border);border-radius:8px}
  .pkg-card.locked{opacity:0.85}
  .pkg-top{display:flex;align-items:center;gap:8px;margin-bottom:4px}
  .pkg-card p{font-size:12px;color:var(--muted);line-height:1.4}
  .tag{font-size:9px;font-weight:700;text-transform:uppercase;padding:2px 6px;border-radius:4px}
  .tag.ok{background:#14532d;color:#86efac}
  .tag.lock{background:#713f12;color:#fcd34d}
  .err{font-size:12px;color:#fca5a5;margin-top:10px}
  .ok{font-size:12px;color:#86efac;margin-top:10px}
</style>
