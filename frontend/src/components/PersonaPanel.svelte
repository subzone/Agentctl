<script>
  import { createEventDispatcher } from 'svelte';
  const dispatch = createEventDispatcher();

  export let agentName = '';
  export let persona = { instructions: '', tone: '', verbosity: '', temperature: null };
  export let saved = false;

  // Local editable copy (remounted per agent via {#key} in the parent).
  let instructions = persona.instructions || '';
  let tone = persona.tone || '';
  let verbosity = persona.verbosity || '';
  let useTemp = persona.temperature !== null && persona.temperature !== undefined;
  let temperature = useTemp ? persona.temperature : 0.7;

  const tones = [
    { id: '', label: 'Default' },
    { id: 'concise', label: 'Concise' },
    { id: 'friendly', label: 'Friendly' },
    { id: 'formal', label: 'Formal' },
    { id: 'playful', label: 'Playful' },
    { id: 'direct', label: 'Direct' },
  ];
  const verbosities = [
    { id: '', label: 'Default' },
    { id: 'terse', label: 'Terse' },
    { id: 'balanced', label: 'Balanced' },
    { id: 'detailed', label: 'Detailed' },
  ];

  function save() {
    dispatch('save', {
      instructions: instructions.trim(),
      tone,
      verbosity,
      temperature: useTemp ? Number(temperature) : null,
    });
  }
  function reset() {
    instructions = ''; tone = ''; verbosity = ''; useTemp = false; temperature = 0.7;
  }
</script>

<div class="panel">
  <div class="phead">
    <div>
      <h2>Personality</h2>
      <p class="sub">Tweak how <strong>{agentName || 'this agent'}</strong> behaves. Applies on your next New Chat.</p>
    </div>
    <div class="actions">
      <button class="btn ghost" on:click={reset}>Reset</button>
      <button class="btn primary" on:click={save}>{saved ? '✓ Saved' : 'Save'}</button>
    </div>
  </div>

  <div class="pbody">
    <div class="field">
      <span class="lbl">Tone</span>
      <div class="chips">
        {#each tones as t}
          <button class="chip" class:on={tone === t.id} on:click={() => tone = t.id}>{t.label}</button>
        {/each}
      </div>
    </div>

    <div class="field">
      <span class="lbl">Verbosity</span>
      <div class="chips">
        {#each verbosities as v}
          <button class="chip" class:on={verbosity === v.id} on:click={() => verbosity = v.id}>{v.label}</button>
        {/each}
      </div>
    </div>

    <div class="field">
      <label class="temp-head">
        <input type="checkbox" bind:checked={useTemp} />
        <span class="lbl">Creativity (temperature)</span>
      </label>
      {#if useTemp}
        <div class="temp-row">
          <input type="range" min="0" max="1.5" step="0.05" bind:value={temperature} />
          <span class="temp-val">{Number(temperature).toFixed(2)}</span>
        </div>
        <p class="hint">Lower is more focused and deterministic; higher is more varied and creative.</p>
      {:else}
        <p class="hint">Using the agent's default creativity.</p>
      {/if}
    </div>

    <div class="field grow">
      <span class="lbl">Custom instructions</span>
      <textarea bind:value={instructions} spellcheck="false"
        placeholder="e.g. Always answer in British English. Prefer Go idioms. Ask before running destructive commands. Sign off as 'Captain'."></textarea>
      <p class="hint">Added verbatim to the agent's system prompt.</p>
    </div>
  </div>
</div>

<style>
  .panel{flex:1;display:flex;flex-direction:column;min-height:0;overflow:hidden}
  .phead{display:flex;justify-content:space-between;align-items:flex-start;gap:12px;padding:14px 20px;border-bottom:1px solid var(--border,#334155);flex-shrink:0}
  .phead h2{margin:0;font-size:16px;color:var(--text,#e2e8f0)}
  .sub{margin:4px 0 0;font-size:12px;color:var(--muted,#64748b)}
  .actions{display:flex;gap:8px;flex-shrink:0}

  .pbody{flex:1;overflow-y:auto;padding:18px 20px;display:flex;flex-direction:column;gap:20px}
  .field{display:flex;flex-direction:column;gap:8px}
  .field.grow{flex:1;min-height:160px}
  .lbl{font-size:11px;font-weight:700;color:var(--muted,#94a3b8);text-transform:uppercase;letter-spacing:0.5px}
  .chips{display:flex;flex-wrap:wrap;gap:6px}
  .chip{padding:6px 14px;border-radius:20px;border:1px solid var(--border,#334155);background:transparent;color:var(--text,#cbd5e1);font-size:12px;cursor:pointer}
  .chip:hover{border-color:var(--accent,#6366f1)}
  .chip.on{background:var(--accent,#6366f1);border-color:var(--accent,#6366f1);color:#fff}

  .temp-head{display:flex;align-items:center;gap:8px;cursor:pointer}
  .temp-row{display:flex;align-items:center;gap:12px;max-width:420px}
  .temp-row input[type=range]{flex:1;accent-color:var(--accent,#6366f1)}
  .temp-val{font-family:'SF Mono',Menlo,monospace;font-size:13px;color:var(--text,#e2e8f0);min-width:38px;text-align:right}
  .hint{margin:0;font-size:11px;color:var(--muted,#64748b)}

  textarea{flex:1;min-height:120px;width:100%;box-sizing:border-box;resize:vertical;padding:12px 14px;background:#0c1322;border:1px solid var(--border,#334155);border-radius:8px;color:#e2e8f0;font-size:13px;line-height:1.5;outline:none;font-family:inherit}
  textarea:focus{border-color:var(--accent,#6366f1)}

  .btn{padding:8px 18px;border-radius:8px;border:1px solid var(--border,#334155);background:var(--bg-input,#1e293b);color:var(--text,#e2e8f0);font-size:13px;font-weight:600;cursor:pointer}
  .btn:hover{filter:brightness(1.12)}
  .btn.primary{background:var(--accent,#6366f1);border-color:var(--accent,#6366f1);color:#fff}
  .btn.ghost{background:transparent;color:var(--muted,#94a3b8)}
</style>
