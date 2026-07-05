<script>
  import { createEventDispatcher } from 'svelte';
  const dispatch = createEventDispatcher();

  export let step = 1;
  const freeProviders = [
    { id: 'groq', label: 'Groq', signup: 'https://console.groq.com/keys' },
    { id: 'gemini', label: 'Gemini', signup: 'https://aistudio.google.com/apikey' },
    { id: 'cerebras', label: 'Cerebras', signup: 'https://cloud.cerebras.ai' },
    { id: 'mistral', label: 'Mistral', signup: 'https://console.mistral.ai/api-keys' },
  ];
  let moeKeys = { groq: '', gemini: '', cerebras: '', mistral: '' };
  let saving = false;
  let error = '';

  async function enableMoE() {
    saving = true; error = '';
    try {
      const entered = freeProviders.filter(p => moeKeys[p.id]?.trim());
      if (entered.length === 0) { error = 'Paste at least one API key, or skip for now.'; saving = false; return; }
      for (const p of entered) {
        await window.go.desktop.App.SaveAPIKey(p.id, moeKeys[p.id].trim());
      }
      const primary = entered[0];
      const models = { groq: 'llama-3.3-70b-versatile', gemini: 'gemini-2.5-flash', cerebras: 'gpt-oss-120b', mistral: 'mistral-large-latest' };
      await window.go.desktop.App.SaveConfig(primary.id, models[primary.id] || '', '');
      step = 3;
    } catch (e) { error = String(e); }
    finally { saving = false; }
  }

  function finish() {
    localStorage.setItem('agentctl_onboarded_v1', '1');
    dispatch('done');
  }
</script>

<div class="overlay" role="presentation">
  <div class="card">
    <div class="steps">
      <span class:done={step > 1}>1</span>
      <span class="line"></span>
      <span class:done={step > 2}>2</span>
      <span class="line"></span>
      <span class:done={step > 2}>3</span>
    </div>

    {#if step === 1}
      <h2>Welcome to AgentCTL</h2>
      <p>Your local agent control plane — chat, memory, custom tools, and MCP servers. Takes about a minute to set up.</p>
      <ul>
        <li><strong>Home</strong> — health & quick navigation</li>
        <li><strong>Chat</strong> — talk to agents with activity timeline</li>
        <li><strong>Extensions</strong> — tools, skills, MCP as files</li>
      </ul>
      <div class="actions">
        <button class="ghost" on:click={finish}>Skip</button>
        <button class="primary" on:click={() => step = 2}>Get started →</button>
      </div>
    {:else if step === 2}
      <h2>Free MoE setup</h2>
      <p>Paste any free API keys below. The default <strong>m</strong> agent routes across them automatically.</p>
      <div class="keys">
        {#each freeProviders as p}
          <label>
            <span>{p.label}</span>
            <input type="password" bind:value={moeKeys[p.id]} placeholder="API key (optional)" autocomplete="off" />
            <a href={p.signup} target="_blank" rel="noopener">Get key</a>
          </label>
        {/each}
      </div>
      {#if error}<p class="err">{error}</p>{/if}
      <div class="actions">
        <button class="ghost" on:click={() => step = 3}>Skip</button>
        <button class="primary" on:click={enableMoE} disabled={saving}>{saving ? 'Saving…' : 'Save & continue'}</button>
      </div>
    {:else}
      <h2>You're ready</h2>
      <p>Try these next:</p>
      <ul>
        <li>Open <strong>Home</strong> to check setup health and resume saved chats</li>
        <li>Start a <strong>Chat</strong> with the default MoE agent</li>
        <li>Press <strong>⌘K</strong> for the command palette · <strong>⌘N</strong> for new chat</li>
        <li>Use <strong>Settings → General → Simple</strong> for a minimal chat-first UI</li>
      </ul>
      <div class="actions">
        <button class="primary" on:click={finish}>Open AgentCTL</button>
      </div>
    {/if}
  </div>
</div>

<style>
  .overlay{position:fixed;inset:0;background:rgba(2,6,23,0.75);backdrop-filter:blur(4px);display:flex;align-items:center;justify-content:center;z-index:200;padding:20px}
  .card{width:480px;max-width:100%;background:#0f172a;border:1px solid #334155;border-radius:14px;padding:28px;box-shadow:0 24px 80px rgba(0,0,0,0.5)}
  .steps{display:flex;align-items:center;justify-content:center;gap:8px;margin-bottom:24px}
  .steps span{width:28px;height:28px;border-radius:50%;display:flex;align-items:center;justify-content:center;font-size:12px;font-weight:700;background:#1e293b;color:#64748b}
  .steps span.done{background:var(--accent);color:#fff}
  .line{width:32px;height:2px;background:#334155}
  h2{margin:0 0 10px;font-size:20px;color:#f1f5f9}
  p{margin:0 0 16px;font-size:14px;color:#94a3b8;line-height:1.5}
  ul{margin:0 0 20px;padding-left:20px;font-size:13px;color:#cbd5e1;line-height:1.7}
  .actions{display:flex;justify-content:flex-end;gap:10px}
  .primary,.ghost{padding:10px 20px;border-radius:8px;font-size:14px;font-weight:600;cursor:pointer;border:1px solid #334155}
  .primary{background:var(--accent);border-color:var(--accent);color:#fff}
  .ghost{background:transparent;color:#94a3b8}
  .keys{display:flex;flex-direction:column;gap:12px;margin-bottom:16px}
  .keys label{display:flex;flex-direction:column;gap:4px;font-size:12px;color:#94a3b8}
  .keys input{padding:8px 10px;background:#080d18;border:1px solid #334155;border-radius:6px;color:#e2e8f0;font-size:13px}
  .keys a{font-size:11px;color:#818cf8}
  .err{color:#f87171;font-size:12px;margin-bottom:12px}
</style>
