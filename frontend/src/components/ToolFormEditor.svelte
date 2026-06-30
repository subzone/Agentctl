<script>
  import { createEventDispatcher } from 'svelte';
  const dispatch = createEventDispatcher();

  export let content = '';
  export let advanced = false;
  export let saving = false;
  export let saved = false;
  export let error = '';

  let form = { name: '', description: '', commandText: '', timeoutSec: 30, parametersJson: '{\n  "type": "object",\n  "properties": {}\n}', body: '' };
  let syncing = false;

  $: if (content && !syncing) loadForm(content);

  async function loadForm(c) {
    try {
      const f = await window.go.desktop.App.ParseToolForm(c);
      form = {
        name: f.name || '',
        description: f.description || '',
        commandText: (f.command || []).join('\n'),
        timeoutSec: f.timeoutSec || 30,
        parametersJson: f.parametersJson || '{\n  "type": "object",\n  "properties": {}\n}',
        body: f.body || '',
      };
      error = '';
    } catch (e) {
      error = String(e);
    }
  }

  async function applyForm() {
    syncing = true;
    try {
      const cmd = form.commandText.split('\n').map(s => s.trim()).filter(Boolean);
      const out = await window.go.desktop.App.ComposeToolForm({
        name: form.name,
        description: form.description,
        command: cmd,
        timeoutSec: Number(form.timeoutSec) || 30,
        parametersJson: form.parametersJson,
        body: form.body,
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
</script>

<div class="studio">
  <div class="toolbar">
    <span class="title">Tool builder</span>
    <button class="toggle" class:on={!advanced} on:click={() => advanced = false}>Form</button>
    <button class="toggle" class:on={advanced} on:click={() => advanced = true}>Advanced MD</button>
    <button class="save" on:click={() => dispatch('save')} disabled={saving}>
      {saving ? 'Saving…' : saved ? '✓ Saved' : 'Save'}
    </button>
  </div>

  {#if error}<div class="err">⚠ {error}</div>{/if}

  {#if advanced}
    <textarea class="md" value={content} on:input={onAdvancedInput} spellcheck="false"
      placeholder="Tool MD (frontmatter + body)"></textarea>
  {:else}
    <div class="form">
      <label>
        <span>Name</span>
        <input bind:value={form.name} on:blur={applyForm} placeholder="my_tool" spellcheck="false" />
      </label>
      <label>
        <span>Description</span>
        <input bind:value={form.description} on:blur={applyForm} placeholder="One-line summary for the model" />
      </label>
      <label>
        <span>Command (one argument per line)</span>
        <textarea class="cmd" bind:value={form.commandText} on:blur={applyForm} rows="4" spellcheck="false"
          placeholder="bash&#10;-c&#10;cat"></textarea>
      </label>
      <label class="row">
        <span>Timeout (seconds)</span>
        <input type="number" min="1" max="600" bind:value={form.timeoutSec} on:blur={applyForm} />
      </label>
      <label>
        <span>Parameters (JSON Schema)</span>
        <textarea class="json" bind:value={form.parametersJson} on:blur={applyForm} rows="6" spellcheck="false"></textarea>
      </label>
      <label class="grow">
        <span>Instructions for the model</span>
        <textarea bind:value={form.body} on:blur={applyForm} rows="5" spellcheck="false"
          placeholder="Optional longer description of inputs, outputs, and when to use this tool."></textarea>
      </label>
    </div>
  {/if}
</div>

<style>
  .studio{flex:1;display:flex;flex-direction:column;min-height:0;overflow:hidden}
  .toolbar{display:flex;align-items:center;gap:8px;padding:10px 16px;border-bottom:1px solid var(--border);flex-shrink:0;background:#080d18}
  .title{flex:1;font-size:13px;font-weight:600;color:var(--text)}
  .toggle{padding:5px 10px;border-radius:6px;border:1px solid var(--border);background:transparent;color:var(--muted);font-size:11px;font-weight:600;cursor:pointer}
  .toggle.on{background:#1e293b;color:var(--text);border-color:#6366f1}
  .save{padding:6px 14px;border-radius:6px;border:none;background:var(--accent);color:#fff;font-size:12px;font-weight:600;cursor:pointer}
  .save:disabled{opacity:0.6}
  .err{margin:8px 16px 0;padding:8px 10px;border-radius:6px;background:#450a0a;color:#fca5a5;font-size:12px}
  .md,.form{flex:1;min-height:0}
  .md{width:100%;box-sizing:border-box;padding:14px 16px;background:#0c1322;border:none;color:#e2e8f0;font-family:'SF Mono',Menlo,monospace;font-size:12px;line-height:1.5;resize:none;outline:none}
  .form{overflow-y:auto;padding:14px 16px;display:flex;flex-direction:column;gap:12px}
  label{display:flex;flex-direction:column;gap:4px;font-size:11px;color:var(--muted);font-weight:600;text-transform:uppercase}
  label.grow{flex:1;min-height:100px}
  label.row{flex-direction:row;align-items:center;justify-content:space-between;max-width:280px}
  input,textarea{box-sizing:border-box;padding:8px 10px;background:#0c1322;border:1px solid var(--border);border-radius:6px;color:#e2e8f0;font-size:13px;font-family:inherit;resize:vertical}
  input[type=number]{width:80px}
  .cmd,.json{font-family:'SF Mono',Menlo,monospace;font-size:12px}
  textarea{min-height:72px}
</style>
