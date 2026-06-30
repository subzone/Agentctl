<script>
  import { createEventDispatcher } from 'svelte';
  const dispatch = createEventDispatcher();

  export let preview = null; // AgentContextPreview
  export let loading = false;

  function personaSummary(p) {
    if (!p || (!p.tone && !p.verbosity && !p.instructions && p.temperature == null)) return 'Default';
    const parts = [];
    if (p.tone) parts.push(p.tone);
    if (p.verbosity) parts.push(p.verbosity);
    if (p.temperature != null) parts.push('temp ' + p.temperature);
    if (p.instructions) parts.push('custom');
    return parts.join(' · ') || 'Default';
  }
</script>

<aside class="inspector">
  <div class="ihead">
    <h3>Context</h3>
    <button class="x" on:click={() => dispatch('close')} title="Hide">✕</button>
  </div>

  {#if loading}
    <p class="muted">Loading preview…</p>
  {:else if preview}
    <div class="meta">
      <div class="row"><span class="k">Agent</span><span class="v">{preview.agent}</span></div>
      <div class="row"><span class="k">Model</span><span class="v mono">{preview.model}</span></div>
      <div class="row"><span class="k">Skills</span><span class="v">{(preview.skills || []).length ? preview.skills.join(', ') : '—'}</span></div>
      <div class="row"><span class="k">Persona</span><span class="v">{personaSummary(preview.persona)}</span></div>
      <div class="row"><span class="k">Tools</span><span class="v">{preview.toolCount}+ custom/km (+ builtins)</span></div>
      <div class="row"><span class="k">MCP</span><span class="v">{(preview.mcpServers || []).length ? preview.mcpServers.join(', ') : '—'}</span></div>
      <div class="row"><span class="k">Prompt size</span><span class="v">{preview.charCount.toLocaleString()} chars</span></div>
    </div>

    <div class="note">
      Edits to skills, persona, tools, or MCP apply on your next <strong>New Chat</strong>.
      <button class="apply" on:click={() => dispatch('apply')}>Apply now</button>
    </div>

    <div class="prompt-wrap">
      <div class="prompt-lbl">System prompt (preview)</div>
      <pre class="prompt">{preview.system}</pre>
    </div>
  {:else}
    <p class="muted">Select an agent to preview context.</p>
  {/if}
</aside>

<style>
  .inspector{width:320px;flex-shrink:0;display:flex;flex-direction:column;border-left:1px solid var(--border);background:#080d18;min-height:0;overflow:hidden}
  .ihead{display:flex;justify-content:space-between;align-items:center;padding:12px 14px;border-bottom:1px solid var(--border);flex-shrink:0}
  .ihead h3{margin:0;font-size:13px;font-weight:700;color:var(--text)}
  .x{background:none;border:none;color:var(--muted);cursor:pointer;font-size:14px;padding:2px 6px;border-radius:4px}
  .x:hover{background:var(--border);color:var(--text)}
  .muted{padding:14px;font-size:12px;color:var(--muted)}

  .meta{padding:12px 14px;display:flex;flex-direction:column;gap:8px;border-bottom:1px solid var(--border);flex-shrink:0}
  .row{display:flex;gap:8px;font-size:12px}
  .k{color:var(--muted);width:72px;flex-shrink:0;font-weight:600}
  .v{color:var(--text);flex:1;word-break:break-word}
  .mono{font-family:'SF Mono',Menlo,monospace;font-size:11px}

  .note{margin:10px 14px;padding:10px 12px;background:#1e1b4b;border:1px solid #4338ca;border-radius:8px;font-size:11px;color:#c4b5fd;line-height:1.5;flex-shrink:0}
  .apply{display:block;margin-top:8px;width:100%;padding:7px;border-radius:6px;border:none;background:var(--accent);color:#fff;font-size:12px;font-weight:600;cursor:pointer}
  .apply:hover{filter:brightness(1.1)}

  .prompt-wrap{flex:1;min-height:0;display:flex;flex-direction:column;padding:0 14px 14px;overflow:hidden}
  .prompt-lbl{font-size:10px;font-weight:700;color:var(--muted);text-transform:uppercase;letter-spacing:0.5px;padding:10px 0 6px;flex-shrink:0}
  .prompt{flex:1;overflow:auto;margin:0;padding:10px 12px;background:#0c1322;border:1px solid var(--border);border-radius:8px;font-size:11px;line-height:1.45;color:#cbd5e1;white-space:pre-wrap;word-break:break-word;font-family:'SF Mono',Menlo,monospace}
</style>
