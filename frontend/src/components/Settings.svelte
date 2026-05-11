<script>
  import { onMount, createEventDispatcher } from 'svelte';
  
  const dispatch = createEventDispatcher();
  
  let config = null;
  let loading = true;
  let saving = false;
  
  onMount(async () => {
    try {
      const { GetConfig } = await import('../wailsjs/go/desktop/App');
      config = await GetConfig();
    } catch (err) {
      console.error('Failed to load config:', err);
      config = { model: '', apiKeys: {} };
    } finally {
      loading = false;
    }
  });
  
  async function saveSettings() {
    saving = true;
    try {
      const { SaveConfig } = await import('../wailsjs/go/desktop/App');
      await SaveConfig(config);
      alert('Settings saved successfully!');
    } catch (err) {
      console.error('Failed to save settings:', err);
      alert('Failed to save settings: ' + err);
    } finally {
      saving = false;
    }
  }
</script>

<div class="settings">
  <div class="settings-header">
    <h1>Settings</h1>
    <button class="close-btn" on:click={() => dispatch('close')}>✕</button>
  </div>
  
  {#if loading}
    <div class="loading">Loading settings...</div>
  {:else}
    <div class="settings-content">
      <div class="section">
        <h2>Model Configuration</h2>
        <div class="form-group">
          <label for="model">Default Model</label>
          <input
            id="model"
            type="text"
            bind:value={config.model}
            placeholder="e.g., ollama/qwen3-coder or anthropic/claude-3-5-sonnet"
          />
          <p class="help-text">
            Format: provider/model-name (e.g., ollama/qwen3-coder, anthropic/claude-3-5-sonnet)
          </p>
        </div>
      </div>
      
      <div class="section">
        <h2>API Keys</h2>
        <div class="form-group">
          <label for="anthropic-key">Anthropic API Key</label>
          <input
            id="anthropic-key"
            type="password"
            bind:value={config.apiKeys.anthropic}
            placeholder="sk-ant-..."
          />
        </div>
        
        <div class="form-group">
          <label for="openai-key">OpenAI API Key</label>
          <input
            id="openai-key"
            type="password"
            bind:value={config.apiKeys.openai}
            placeholder="sk-..."
          />
        </div>
        
        <div class="form-group">
          <label for="gemini-key">Google Gemini API Key</label>
          <input
            id="gemini-key"
            type="password"
            bind:value={config.apiKeys.gemini}
            placeholder="AIza..."
          />
        </div>
      </div>
      
      <div class="actions">
        <button class="save-btn" on:click={saveSettings} disabled={saving}>
          {saving ? 'Saving...' : 'Save Settings'}
        </button>
      </div>
    </div>
  {/if}
</div>

<style>
  .settings {
    display: flex;
    flex-direction: column;
    height: 100%;
    background-color: #1b2636;
  }
  
  .settings-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 24px;
    border-bottom: 1px solid #334155;
  }
  
  .settings-header h1 {
    font-size: 24px;
    font-weight: 700;
  }
  
  .close-btn {
    background: transparent;
    border: none;
    color: #94a3b8;
    font-size: 24px;
    cursor: pointer;
    padding: 4px 8px;
    border-radius: 4px;
    transition: all 0.2s;
  }
  
  .close-btn:hover {
    background-color: #1e293b;
    color: #e5e7eb;
  }
  
  .loading {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    color: #94a3b8;
  }
  
  .settings-content {
    flex: 1;
    overflow-y: auto;
    padding: 24px;
  }
  
  .section {
    margin-bottom: 32px;
  }
  
  .section h2 {
    font-size: 18px;
    margin-bottom: 16px;
    color: #e5e7eb;
  }
  
  .form-group {
    margin-bottom: 20px;
  }
  
  label {
    display: block;
    margin-bottom: 8px;
    font-size: 14px;
    font-weight: 500;
    color: #e5e7eb;
  }
  
  input {
    width: 100%;
    padding: 10px 12px;
    background-color: #0f172a;
    border: 1px solid #334155;
    border-radius: 8px;
    color: #e5e7eb;
    font-size: 14px;
    font-family: inherit;
  }
  
  input:focus {
    outline: none;
    border-color: #3b82f6;
  }
  
  .help-text {
    margin-top: 6px;
    font-size: 12px;
    color: #94a3b8;
  }
  
  .actions {
    padding-top: 24px;
    border-top: 1px solid #334155;
  }
  
  .save-btn {
    padding: 12px 24px;
    background-color: #3b82f6;
    border: none;
    border-radius: 8px;
    color: white;
    font-weight: 600;
    font-size: 14px;
    cursor: pointer;
    transition: background-color 0.2s;
  }
  
  .save-btn:hover:not(:disabled) {
    background-color: #2563eb;
  }
  
  .save-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
</style>
