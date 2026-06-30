<script>
  import { createEventDispatcher } from 'svelte';
  const dispatch = createEventDispatcher();

  export let content = '';
  export let advanced = false;
  export let saving = false;
  export let saved = false;
  export let error = '';
  export let builtin = false;
  export let builtinTools = [];
  export let customToolNames = [];
  export let skillNames = [];
  export let mcpNames = [];

  let form = {
    name: '', description: '', model: '', fallbackLines: '',
    tools: [], skills: [], mcp: [], body: '',
    useTemp: false, temperature: 0.3, hasRouting: false,
  };
  let syncing = false;

  $: allToolOptions = [
    ...builtinTools.map(t => ({ id: t, group: 'Built-in' })),
    ...customToolNames.map(t => ({ id: t, group: 'Custom' })),
  ];

  $: if (content && !syncing) loadForm(content);

  async function loadForm(c) {
    try {
      const f = await window.go.desktop.App.ParseAgentForm(c);
      form = {
        name: f.name || '',
        description: f.description || '',
        model: f.model || '',
        fallbackLines: f.fallbackLines || '',
        tools: f.tools || [],
        skills: f.skills || [],
        mcp: f.mcp || [],
        body: f.body || '',
        useTemp: f.temperature != null,
        temperature: f.temperature ?? 0.3,
        hasRouting: !!f.hasRouting,
      };
      error = '';
    } catch (e) {
      error = String(e);
    }
  }

  function toggle(arr, id) {
    const i = arr.indexOf(id);
    if (i >= 0) arr.splice(i, 1);
    else arr.push(id);
    return arr;
  }

  async function applyForm() {
    if (builtin) return;
    syncing = true;
    try {
      const out = await window.go.desktop.App.ComposeAgentForm({
        name: form.name,
        description: form.description,
        model: form.model,
        fallbackLines: form.fallbackLines,
        tools: form.tools,
        skills: form.skills,
        mcp: form.mcp,
        temperature: form.useTemp ? Number(form.temperature) : null,
        body: form.body,
        hasRouting: form.hasRouting,
      });
      content = out;
      dispatch('change', out);
      error = '';
    } catch (e) {
      error = String(e);
    } finally {
      syncing = false;
    }
  }

  function onAdvancedInput(e) {
    content = e.target.value;
    dispatch('change', content);
  }

  async function duplicateBuiltin() {
    dispatch('duplicate');
  }
</script>

<div class="studio">
  <div class="toolbar">
    <span class="title">Agent Studio</span>
    {#if builtin}
      <button class="dup" on:click={duplicateBuiltin}>Duplicate as custom</button>
    {:else}
      <button class="toggle" class:on={!advanced} on:click={() => advanced = false}>Form</button>
      <button class="toggle" class:on={advanced} on:click={() => advanced = true}>Advanced MD</button>
      <button class="save" on:click={() => dispatch('save')} disabled={saving}>
        {saving ? 'Saving…' : saved ? '✓ Saved' : 'Save'}
      </button>
    {/if}
  </div>

  {#if error}<div class="err">⚠ {error}</div>{/if}
  {#if form.hasRouting && !advanced}
    <div class="warn">MoE routing is configured — use <strong>Advanced MD</strong> to edit routing without losing it.</div>
  {/if}
  {#if builtin && !advanced}
    <div class="warn">Built-in agent — view only. Duplicate to create an editable copy in <code>~/.config/m/agents</code>.</div>
  {/if}

  {#if advanced || builtin}
    <textarea class="md" value={content} on:input={onAdvancedInput} readonly={builtin && !advanced}
      spellcheck="false" placeholder="Agent MD"></textarea>
  {:else}
    <div class="form">
      <label><span>Name</span><input bind:value={form.name} on:blur={applyForm} /></label>
      <label><span>Description</span><input bind:value={form.description} on:blur={applyForm} /></label>
      <label><span>Model</span><input bind:value={form.model} on:blur={applyForm} placeholder="groq/llama-3.3-70b-versatile" /></label>
      <label><span>Fallback models (one per line)</span>
        <textarea class="sm" bind:value={form.fallbackLines} on:blur={applyForm} rows="2"></textarea>
      </label>
      <div class="field">
        <span class="lbl">Tools</span>
        <div class="checks">
          {#each allToolOptions as t}
            <label class="chk">
              <input type="checkbox" checked={form.tools.includes(t.id)}
                on:change={() => { form.tools = toggle([...form.tools], t.id); applyForm(); }} />
              <span>{t.id}</span>
            </label>
          {/each}
        </div>
      </div>
      {#if skillNames.length}
        <div class="field">
          <span class="lbl">Skills</span>
          <div class="checks">
            {#each skillNames as s}
              <label class="chk">
                <input type="checkbox" checked={form.skills.includes(s)}
                  on:change={() => { form.skills = toggle([...form.skills], s); applyForm(); }} />
                <span>{s}</span>
              </label>
            {/each}
          </div>
        </div>
      {/if}
      {#if mcpNames.length}
        <div class="field">
          <span class="lbl">MCP servers</span>
          <div class="checks">
            {#each mcpNames as m}
              <label class="chk">
                <input type="checkbox" checked={form.mcp.includes(m)}
                  on:change={() => { form.mcp = toggle([...form.mcp], m); applyForm(); }} />
                <span>{m}</span>
              </label>
            {/each}
          </div>
        </div>
      {/if}
      <label class="row">
        <input type="checkbox" bind:checked={form.useTemp} on:change={applyForm} />
        <span>Temperature {form.useTemp ? form.temperature : '(default)'}</span>
        {#if form.useTemp}
          <input type="range" min="0" max="1.5" step="0.05" bind:value={form.temperature} on:change={applyForm} />
        {/if}
      </label>
      <label class="grow">
        <span>Instructions (system prompt body)</span>
        <textarea bind:value={form.body} on:blur={applyForm} rows="8"></textarea>
      </label>
    </div>
  {/if}
</div>

<style>
  .studio{flex:1;display:flex;flex-direction:column;min-height:0;overflow:hidden}
  .toolbar{display:flex;align-items:center;gap:8px;padding:10px 16px;border-bottom:1px solid var(--border);flex-shrink:0;background:#080d18}
  .title{flex:1;font-size:13px;font-weight:600}
  .toggle,.dup{padding:5px 10px;border-radius:6px;border:1px solid var(--border);background:transparent;color:var(--muted);font-size:11px;font-weight:600;cursor:pointer}
  .toggle.on{background:#1e293b;color:var(--text);border-color:#6366f1}
  .dup{border-color:#6366f1;color:#a5b4fc}
  .save{padding:6px 14px;border-radius:6px;border:none;background:var(--accent);color:#fff;font-size:12px;font-weight:600;cursor:pointer}
  .err,.warn{margin:8px 16px 0;padding:8px 10px;border-radius:6px;font-size:12px}
  .err{background:#450a0a;color:#fca5a5}
  .warn{background:#1e1b4b;color:#c4b5fd}
  .warn code{font-size:11px}
  .md,.form{flex:1;min-height:0}
  .md{width:100%;box-sizing:border-box;padding:14px 16px;background:#0c1322;border:none;color:#e2e8f0;font-family:'SF Mono',Menlo,monospace;font-size:12px;resize:none;outline:none}
  .form{overflow-y:auto;padding:14px 16px;display:flex;flex-direction:column;gap:12px}
  label{display:flex;flex-direction:column;gap:4px;font-size:11px;color:var(--muted);font-weight:600;text-transform:uppercase}
  label.grow{flex:1;min-height:120px}
  label.row{flex-direction:row;align-items:center;gap:8px;text-transform:none;font-size:12px}
  input,textarea,textarea.sm{box-sizing:border-box;padding:8px 10px;background:#0c1322;border:1px solid var(--border);border-radius:6px;color:#e2e8f0;font-size:13px;font-family:inherit}
  .field .lbl{font-size:11px;font-weight:700;color:var(--muted);text-transform:uppercase;margin-bottom:4px;display:block}
  .checks{display:flex;flex-wrap:wrap;gap:6px}
  .chk{display:flex;align-items:center;gap:4px;font-size:11px;color:#cbd5e1;text-transform:none;font-weight:400;cursor:pointer}
  .chk input{margin:0}
</style>
