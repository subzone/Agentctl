<script>
  import { onMount, createEventDispatcher } from 'svelte';
  const dispatch = createEventDispatcher();

  let provider = '';
  let model = '';
  let baseURL = '';
  let apiKey = '';
  let saving = false;
  let saved = false;
  let error = '';
  let themes = [];
  let activeTheme = localStorage.getItem('theme') || 'default';

  const providers = ['anthropic', 'openai', 'ollama', 'gemini', 'alibaba', 'litellm'];

  onMount(async () => {
    try {
      const cfg = await window.go.desktop.App.GetConfig();
      if (cfg) { provider = cfg.Provider || ''; model = cfg.Model || ''; baseURL = cfg.BaseURL || ''; }
      themes = await window.go.desktop.App.GetThemes();
    } catch (e) { error = String(e); }
  });

  async function save() {
    saving = true; error = '';
    try {
      await window.go.desktop.App.SaveConfig(provider, model, baseURL);
      if (apiKey.trim()) {
        await window.go.desktop.App.SaveAPIKey(provider, apiKey.trim());
        apiKey = '';
      }
      saved = true;
      setTimeout(() => saved = false, 2000);
    } catch (e) { error = String(e); }
    finally { saving = false; }
  }

  function applyTheme(theme) {
    activeTheme = theme.name;
    localStorage.setItem('theme', theme.name);
    dispatch('theme', theme);
  }
</script>

<div class="settings">
  <div class="header">
    <h2>Settings</h2>
    <button class="close" on:click={() => dispatch('close')}>✕</button>
  </div>

  <div class="body">
    <div class="section">
      <h3>Theme</h3>
      <div class="theme-grid">
        {#each themes as t}
          <button
            class="theme-btn"
            class:active={activeTheme === t.name}
            style="--bg:{t.bg};--accent:{t.accent};--text:{t.text}"
            on:click={() => applyTheme(t)}
          >
            <div class="theme-preview">
              <div class="preview-bar" style="background:{t.bgPanel}"></div>
              <div class="preview-msg user" style="background:{t.user}22;border-color:{t.user}44"></div>
              <div class="preview-msg asst" style="background:{t.bgInput}"></div>
            </div>
            <span class="theme-name">{t.name}</span>
          </button>
        {/each}
      </div>
    </div>

    <div class="section">
      <h3>Provider & Model</h3>
      <label>Provider
        <select bind:value={provider}>
          <option value="">Select provider...</option>
          {#each providers as p}<option value={p}>{p}</option>{/each}
        </select>
      </label>
      <label>Model
        <input type="text" bind:value={model} placeholder="e.g. qwen-plus, claude-sonnet-4-6" />
      </label>
      {#if provider === 'ollama' || provider === 'litellm' || provider === 'alibaba'}
        <label>Base URL
          <input type="text" bind:value={baseURL} placeholder="e.g. https://dashscope-intl.aliyuncs.com/compatible-mode" />
        </label>
      {/if}
    </div>

    <div class="section">
      <h3>API Key</h3>
      <p class="hint">Stored in OS keychain. Leave blank to keep existing.</p>
      <label>API Key
        <input type="password" bind:value={apiKey} placeholder="Paste new key to update..." />
      </label>
    </div>

    {#if error}<div class="err">⚠ {error}</div>{/if}

    <div class="actions">
      <button class="cancel" on:click={() => dispatch('close')}>Cancel</button>
      <button class="save-btn" on:click={save} disabled={saving}>
        {saving ? 'Saving...' : saved ? '✓ Saved' : 'Save'}
      </button>
    </div>
  </div>
</div>

<style>
  .settings{display:flex;flex-direction:column;height:100vh;background:var(--bg,#0f172a)}
  .header{
    display:flex;justify-content:space-between;align-items:center;
    padding:14px 24px;padding-top:42px;background:var(--bg-panel,#1e293b);
    border-bottom:1px solid var(--border,#334155);-webkit-app-region:drag;
  }
  .header h2{font-size:15px;font-weight:600;color:var(--text,#f1f5f9);-webkit-app-region:no-drag}
  .close{background:none;border:none;color:var(--muted,#64748b);font-size:18px;cursor:pointer;-webkit-app-region:no-drag;padding:4px 8px;border-radius:4px}
  .close:hover{background:var(--border,#334155);color:var(--text,#e2e8f0)}

  .body{flex:1;overflow-y:auto;padding:20px 24px;display:flex;flex-direction:column;gap:20px;max-width:600px}
  .section{display:flex;flex-direction:column;gap:10px}
  .section h3{font-size:11px;font-weight:600;color:var(--muted,#94a3b8);text-transform:uppercase;letter-spacing:0.5px}

  .theme-grid{display:grid;grid-template-columns:repeat(4,1fr);gap:8px}
  .theme-btn{
    background:var(--bg,#0f172a);border:2px solid var(--border,#334155);
    border-radius:8px;padding:8px;cursor:pointer;transition:all 0.15s;
    display:flex;flex-direction:column;align-items:center;gap:6px;
  }
  .theme-btn:hover{border-color:var(--accent,#3b82f6)}
  .theme-btn.active{border-color:var(--accent,#3b82f6);box-shadow:0 0 0 2px var(--accent,#3b82f6)44}
  .theme-preview{width:100%;height:40px;border-radius:4px;overflow:hidden;display:flex;flex-direction:column;gap:2px;padding:4px;background:var(--bg)}
  .preview-bar{height:8px;border-radius:2px;width:100%}
  .preview-msg{height:8px;border-radius:2px;border:1px solid transparent}
  .preview-msg.user{width:60%;align-self:flex-end}
  .preview-msg.asst{width:75%}
  .theme-name{font-size:10px;color:var(--text,#94a3b8);text-transform:capitalize}

  label{display:flex;flex-direction:column;gap:5px;font-size:12px;color:var(--muted,#94a3b8)}
  input,select{
    padding:9px 12px;background:var(--bg-input,#1e293b);border:1px solid var(--border,#334155);
    border-radius:7px;color:var(--text,#e2e8f0);font-size:13px;outline:none;
  }
  input:focus,select:focus{border-color:var(--accent,#3b82f6)}
  select option{background:var(--bg-panel,#1e293b)}
  .hint{font-size:11px;color:var(--muted,#475569)}
  .err{background:#2d1111;border:1px solid #ef444433;color:#fca5a5;padding:10px 14px;border-radius:7px;font-size:13px}

  .actions{display:flex;gap:10px;justify-content:flex-end;padding-top:4px}
  .cancel{padding:9px 20px;background:none;border:1px solid var(--border,#334155);border-radius:7px;color:var(--muted,#94a3b8);font-size:13px;cursor:pointer}
  .cancel:hover{background:var(--bg-input,#1e293b)}
  .save-btn{padding:9px 24px;background:var(--accent,#3b82f6);border:none;border-radius:7px;color:white;font-size:13px;font-weight:600;cursor:pointer}
  .save-btn:hover:not(:disabled){filter:brightness(1.1)}
  .save-btn:disabled{opacity:0.6;cursor:not-allowed}
</style>
