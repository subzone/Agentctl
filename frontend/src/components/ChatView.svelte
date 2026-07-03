<script>
  import { tick, createEventDispatcher, onMount } from 'svelte';
  import ToolCallCard from './ToolCallCard.svelte';
  import MoERoutePanel from './MoERoutePanel.svelte';
  import ActivityTimeline from './ActivityTimeline.svelte';
  import ContextInspector from './ContextInspector.svelte';
  import MCPDashboard from './MCPDashboard.svelte';

  const dispatch = createEventDispatcher();

  export let agent = null;
  export let sessionId = null;
  export let messages = [];
  export let streamBuffer = '';
  export let streaming = false;
  export let cost = { inputTokens: 0, outputTokens: 0, cost: 0 };
  export let moeRoute = null;
  export let moeRouteHistory = [];
  export let sessionMCP = [];
  export let proMode = true;
  export let showInspector = false;
  export let contextPreview = null;
  export let contextLoading = false;
  export let persona = {};
  export let pendingApproval = null;
  export let pendingContinue = null;
  export let attachedFiles = [];
  export let inputValue = '';
  export let activityEvents = [];

  let messagesEl;
  let showMCPDashboard = false;
  let fileSuggestions = [];
  let fileSuggestOpen = false;
  let fileSuggestIdx = 0;
  let atQuery = '';
  let atStart = -1;

  export function scrollToBottom() {
    tick().then(() => { if (messagesEl) messagesEl.scrollTop = messagesEl.scrollHeight; });
  }

  function personaLabel() {
    const parts = [];
    if (persona.tone) parts.push(persona.tone);
    if (persona.verbosity) parts.push(persona.verbosity);
    if (persona.instructions?.trim()) parts.push('custom');
    return parts.length ? parts.join(' · ') : 'default';
  }

  function fmt(n) { return (n || 0) >= 1000 ? ((n || 0) / 1000).toFixed(1) + 'k' : String(n || 0); }
  function fmtCost(c) { if (!c) return '$0.00'; return c < 0.01 ? '$' + c.toFixed(4) : '$' + c.toFixed(2); }

  function escHtml(s) {
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }

  function inlineMd(text) {
    let s = escHtml(text);
    s = s.replace(/`([^`]+)`/g, '<code class="inline-code">$1</code>');
    s = s.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
    s = s.replace(/\*([^*]+)\*/g, '<em>$1</em>');
    s = s.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" class="md-link">$1</a>');
    return s;
  }

  function renderMarkdown(text) {
    if (!text) return '';
    let html = '';
    const lines = text.split('\n');
    let i = 0;
    while (i < lines.length) {
      const line = lines[i];
      const fence = line.match(/^```(\w*)/);
      if (fence) {
        const lang = fence[1] || '';
        let code = '';
        i++;
        while (i < lines.length && !lines[i].startsWith('```')) {
          code += (code ? '\n' : '') + lines[i];
          i++;
        }
        i++;
        const langClass = lang ? ' lang-' + lang : '';
        html += `<pre class="code-block${langClass}"><code>${escHtml(code)}</code></pre>`;
        continue;
      }
      const hMatch = line.match(/^(#{1,3})\s+(.+)/);
      if (hMatch) {
        const level = hMatch[1].length;
        html += `<h${level} class="md-h">${inlineMd(hMatch[2])}</h${level}>`;
        i++; continue;
      }
      if (line.match(/^\s*[-*]\s+/)) {
        html += '<ul class="md-list">';
        while (i < lines.length && lines[i].match(/^\s*[-*]\s+/)) {
          html += `<li>${inlineMd(lines[i].replace(/^\s*[-*]\s+/, ''))}</li>`;
          i++;
        }
        html += '</ul>';
        continue;
      }
      if (line.trim() === '') { html += '<br/>'; i++; continue; }
      html += `<p class="md-p">${inlineMd(line)}</p>`;
      i++;
    }
    return html;
  }

  async function attachFile() {
    try {
      const file = await window.go.desktop.App.OpenFile();
      if (!file || !file.name) return;
      if (file.kind === 'image' && file.dataB64) {
        attachedFiles = [...attachedFiles, {
          name: file.name,
          kind: 'image',
          mimeType: file.mimeType,
          dataB64: file.dataB64,
        }];
      } else if (file.content !== undefined) {
        attachedFiles = [...attachedFiles, {
          name: file.name,
          kind: 'text',
          content: file.content,
        }];
      }
    } catch (e) {
      dispatch('error', String(e));
    }
  }

  function removeFile(name) {
    attachedFiles = attachedFiles.filter(f => f.name !== name);
  }

  async function handleSlashCommand(cmd) {
    const parts = cmd.split(/\s+/);
    const name = parts[0].toLowerCase();
    try {
      if (name === '/help') {
        messages = [...messages, { role: 'system', content: 'Commands: /reset · /retry · /model provider/model · /export · @file/path' }];
        return;
      }
      if (name === '/reset') {
        await window.go.desktop.App.ResetSession(sessionId);
        messages = [];
        streamBuffer = '';
        dispatch('cleared');
        dispatch('toast', { message: 'Chat cleared', type: 'ok' });
        return;
      }
      if (name === '/retry') {
        streaming = true;
        await window.go.desktop.App.RetryLastMessage(sessionId);
        dispatch('toast', { message: 'Retrying last message', type: 'ok' });
        return;
      }
      if (name === '/model' && parts[1]) {
        await window.go.desktop.App.SwitchSessionModel(sessionId, parts[1]);
        messages = [...messages, { role: 'system', content: '◆ Model → ' + parts[1] }];
        dispatch('toast', { message: 'Model updated', type: 'ok' });
        return;
      }
      if (name === '/export') {
        dispatch('export');
        return;
      }
      messages = [...messages, { role: 'system', content: 'Unknown command. Try /help' }];
    } catch (e) {
      dispatch('toast', { message: String(e), type: 'error' });
    }
    scrollToBottom();
  }

  async function sendMessage() {
    let msg = inputValue.trim();
    if (!msg && attachedFiles.length === 0) return;
    if (!sessionId || streaming) return;

    if (msg.startsWith('/')) {
      inputValue = '';
      closeFileSuggest();
      await handleSlashCommand(msg);
      return;
    }

    let fullMsg = msg;
    const textFiles = attachedFiles.filter(f => f.kind !== 'image');
    const imageFiles = attachedFiles.filter(f => f.kind === 'image');
    for (const f of textFiles) {
      fullMsg += `\n\n[File: ${f.name}]\n\`\`\`\n${f.content}\n\`\`\``;
    }
    const images = imageFiles.map(f => ({
      name: f.name,
      mimeType: f.mimeType || 'image/png',
      dataB64: f.dataB64,
    }));
    const attachCount = textFiles.length + imageFiles.length;
    const displayMsg = msg + (attachCount > 0 ? ` [+${attachCount} file(s)]` : '');
    inputValue = '';
    attachedFiles = [];
    closeFileSuggest();
    messages = [...messages, { role: 'user', content: displayMsg }];
    dispatch('activity', { kind: 'user', label: displayMsg.substring(0, 80) });
    streaming = true;
    streamBuffer = '';
    await tick();
    scrollToBottom();
    try {
      await window.go.desktop.App.SendMessage(sessionId, fullMsg, images);
    } catch (e) {
      streaming = false;
      messages = [...messages, { role: 'error', content: String(e) }];
    }
  }

  async function stopGen() {
    try { await window.go.desktop.App.StopGeneration(sessionId); } catch (_) {}
    pendingApproval = null;
    pendingContinue = null;
    streaming = false;
    if (streamBuffer) {
      messages = [...messages, { role: 'assistant', content: streamBuffer + '\n[stopped]' }];
      streamBuffer = '';
    }
  }

  async function respondApproval(approved) {
    if (!pendingApproval) return;
    const { requestId, tool, input } = pendingApproval;
    pendingApproval = null;
    messages = [...messages, {
      role: 'tool',
      content: (approved ? '✓ approved ' : '✕ denied ') + tool + ' ' + input.substring(0, 80),
    }];
    try { await window.go.desktop.App.RespondToolApproval(requestId, approved); } catch (e) { dispatch('error', String(e)); }
    dispatch('activity', { kind: 'tool', label: tool, detail: { ok: approved, approval: true } });
    scrollToBottom();
  }

  async function respondContinue(keepGoing) {
    if (!pendingContinue) return;
    const { requestId } = pendingContinue;
    pendingContinue = null;
    if (!keepGoing) {
      messages = [...messages, { role: 'system', content: '■ stopped after tool-step limit' }];
    }
    try { await window.go.desktop.App.RespondToolApproval(requestId, keepGoing); } catch (e) { dispatch('error', String(e)); }
    scrollToBottom();
  }

  async function retryLastChat() {
    if (!sessionId || streaming) return;
    streaming = true;
    try {
      await window.go.desktop.App.RetryLastMessage(sessionId);
      dispatch('toast', { message: 'Retrying last message', type: 'ok' });
    } catch (e) {
      dispatch('toast', { message: String(e), type: 'error' });
      streaming = false;
    }
  }

  let suggestTimer;
  async function refreshFileSuggestions(query) {
    try {
      fileSuggestions = await window.go.desktop.App.SearchAtFiles(query) || [];
      fileSuggestOpen = fileSuggestions.length > 0;
      fileSuggestIdx = 0;
    } catch (_) {
      fileSuggestions = [];
      fileSuggestOpen = false;
    }
  }

  function closeFileSuggest() {
    fileSuggestOpen = false;
    fileSuggestions = [];
    atQuery = '';
    atStart = -1;
  }

  function detectAtMention(val, cursor) {
    const before = val.slice(0, cursor);
    const at = before.lastIndexOf('@');
    if (at < 0) { closeFileSuggest(); return; }
    const fragment = before.slice(at + 1);
    if (fragment.includes(' ') || fragment.includes('\n')) { closeFileSuggest(); return; }
    atStart = at;
    atQuery = fragment;
    clearTimeout(suggestTimer);
    suggestTimer = setTimeout(() => refreshFileSuggestions(atQuery), 120);
  }

  function insertFileMention(path) {
    if (atStart < 0) return;
    const before = inputValue.slice(0, atStart);
    const after = inputValue.slice(atStart + 1 + atQuery.length);
    inputValue = before + '@' + path + ' ' + after;
    closeFileSuggest();
  }

  function onInput(e) {
    detectAtMention(e.target.value, e.target.selectionStart);
  }

  function handleKey(e) {
    if (fileSuggestOpen && fileSuggestions.length > 0) {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        fileSuggestIdx = (fileSuggestIdx + 1) % fileSuggestions.length;
        return;
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        fileSuggestIdx = (fileSuggestIdx - 1 + fileSuggestions.length) % fileSuggestions.length;
        return;
      }
      if (e.key === 'Tab' || (e.key === 'Enter' && !e.shiftKey)) {
        e.preventDefault();
        insertFileMention(fileSuggestions[fileSuggestIdx].path);
        return;
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        closeFileSuggest();
        return;
      }
    }
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  }
</script>

<div class="chat-layout">
  <div class="chat-col">
    <div class="chat-toolbar">
      <span class="chat-title">◆ {agent?.name}</span>
      {#if moeRoute}<span class="moe-pill">{moeRoute.category}</span>{/if}
      {#if sessionMCP.length > 0}
        <button class="mcp-pill mcp-btn" title={sessionMCP.join(', ')} on:click={() => showMCPDashboard = true}>
          🔌 {sessionMCP.length} MCP
        </button>
      {/if}
      {#if proMode}
        <button class="tb-btn persona-pill" on:click={() => dispatch('personality')} title="Edit personality">
          🎭 {personaLabel()}
        </button>
        <button class="tb-btn" class:on={showInspector}
          on:click={() => dispatch('toggleInspector')} title="Context inspector">◧ Context</button>
      {/if}
      <button class="tb-btn" on:click={retryLastChat} disabled={streaming || messages.length === 0} title="Retry last message">↻ Retry</button>
      <button class="tb-btn" on:click={() => dispatch('palette')} title="Command palette (⌘K)">⌘K</button>
      <button class="tb-btn" on:click={() => dispatch('export')} title="Export chat as Markdown">⤓ Export</button>
    </div>

    <MoERoutePanel routes={moeRouteHistory} />
    <ActivityTimeline events={activityEvents} />

    <div class="messages" bind:this={messagesEl}>
      {#each messages as msg (msg)}
        <div class="msg {msg.role}">
          <div class="lbl">
            {#if msg.role === 'user'}» You
            {:else if msg.role === 'assistant'}◆ {agent?.name || 'Agent'}
            {:else if msg.role === 'tool'}⚙ Tool
            {:else if msg.role === 'error'}⚠ Error
            {:else}ℹ{/if}
          </div>
          {#if msg.role === 'assistant'}
            <div class="txt md-content">{@html renderMarkdown(msg.content)}</div>
          {:else if msg.role === 'toolcard'}
            <ToolCallCard call={msg.toolCall} />
          {:else}
            <div class="txt">{msg.content}</div>
          {/if}
        </div>
      {/each}
      {#if streaming}
        <div class="msg assistant">
          <div class="lbl">◆ {agent?.name || 'Agent'}</div>
          <div class="txt md-content">
            {#if streamBuffer}{@html renderMarkdown(streamBuffer)}<span class="cursor">▊</span>
            {:else}<span class="thinking">Thinking...</span>{/if}
          </div>
        </div>
      {/if}
    </div>

    {#if pendingApproval}
      <div class="approval">
        <div class="approval-info">
          <span class="approval-tool">⚙ {pendingApproval.tool}</span>
          <code class="approval-input">{pendingApproval.input.substring(0, 160)}</code>
        </div>
        <div class="approval-actions">
          <button class="appr deny" on:click={() => respondApproval(false)}>Deny</button>
          <button class="appr allow" on:click={() => respondApproval(true)}>Allow</button>
        </div>
      </div>
    {/if}

    {#if pendingContinue}
      <div class="approval continue">
        <div class="approval-info">
          <span class="approval-tool">↻ Tool-step limit reached</span>
          <span class="continue-sub">M ran {pendingContinue.turns} tool steps without finishing. Keep going?</span>
        </div>
        <div class="approval-actions">
          <button class="appr deny" on:click={() => respondContinue(false)}>Stop</button>
          <button class="appr allow" on:click={() => respondContinue(true)}>Continue</button>
        </div>
      </div>
    {/if}

    {#if attachedFiles.length > 0}
      <div class="attachments">
        {#each attachedFiles as f}
          <div class="chip">
            {#if f.kind === 'image'}
              🖼️
            {:else}
              📄
            {/if}
            <span>{f.name}</span>
            <button on:click={() => removeFile(f.name)}>✕</button>
          </div>
        {/each}
      </div>
    {/if}

    <div class="input-area">
      <div class="input-row">
        <button class="attach-btn" on:click={attachFile} disabled={streaming} title="Attach file">📎</button>
        <div class="input-wrap">
          {#if fileSuggestOpen}
            <div class="file-suggest">
              {#each fileSuggestions as s, i}
                <button type="button" class="file-opt" class:sel={i === fileSuggestIdx}
                  on:click={() => insertFileMention(s.path)}>
                  <span class="file-name">@{s.path}</span>
                  <span class="file-dir">{s.dir}</span>
                </button>
              {/each}
            </div>
          {/if}
          <textarea bind:value={inputValue} on:keydown={handleKey} on:input={onInput}
            placeholder="Type a message… @file.go · 📎 images · Enter to send"
            disabled={streaming} rows="3"></textarea>
        </div>
        <div class="btn-col">
          <button class="btn new-chat" on:click={() => dispatch('newChat')} title="New Chat">＋</button>
          {#if streaming}
            <button class="btn stop" on:click={stopGen}>■ Stop</button>
          {:else}
            <button class="btn send" on:click={sendMessage} disabled={!inputValue.trim() && attachedFiles.length === 0}>Send</button>
          {/if}
        </div>
      </div>
      <div class="footer-bar">
        <span class="cost-info">In: {fmt(cost.inputTokens)} · Out: {fmt(cost.outputTokens)} · {fmtCost(cost.cost)}</span>
        <span class="hint-txt">@file inline · Enter send · ⌘K palette</span>
      </div>
    </div>
  </div>

  {#if showInspector}
    <ContextInspector preview={contextPreview} loading={contextLoading}
      on:close={() => dispatch('closeInspector')}
      on:apply={() => dispatch('newChat')} />
  {/if}
</div>

{#if showMCPDashboard}
  <MCPDashboard sessionId={sessionId} on:close={() => showMCPDashboard = false} />
{/if}

<style>
  .chat-layout{flex:1;display:flex;min-height:0;overflow:hidden}
  .chat-col{flex:1;display:flex;flex-direction:column;min-width:0;min-height:0;overflow:hidden}
  .chat-toolbar{display:flex;align-items:center;gap:8px;padding:8px 14px;border-bottom:1px solid var(--border);flex-shrink:0;background:#080d18}
  .chat-title{font-size:13px;font-weight:600;color:var(--text);flex:1}
  .moe-pill{font-size:10px;font-weight:700;padding:2px 8px;border-radius:4px;background:#4c1d95;color:#c4b5fd}
  .mcp-pill{font-size:10px;font-weight:600;padding:2px 8px;border-radius:4px;background:#0c4a6e;color:#7dd3fc}
  .mcp-btn{border:none;cursor:pointer;font-family:inherit}
  .mcp-btn:hover{filter:brightness(1.15)}
  .persona-pill{font-size:10px;max-width:140px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .tb-btn{padding:5px 10px;border-radius:6px;border:1px solid var(--border);background:var(--bg-input);color:var(--muted);font-size:11px;font-weight:600;cursor:pointer}
  .tb-btn:hover{color:var(--text)}
  .tb-btn.on{border-color:var(--accent);color:#93c5fd}

  .messages{flex:1;overflow-y:auto;padding:20px;display:flex;flex-direction:column;gap:14px;min-height:0}
  .msg{padding:12px 16px;border-radius:8px;max-width:82%;word-break:break-word;flex-shrink:0}
  .msg.user{background:color-mix(in srgb,var(--user) 18%,transparent);align-self:flex-end;border:1px solid color-mix(in srgb,var(--user) 35%,transparent)}
  .msg.assistant{background:var(--bg-input);align-self:flex-start}
  .msg.tool{background:var(--bg-panel);align-self:flex-start;font-family:'SF Mono',Menlo,monospace;font-size:12px;color:var(--muted);max-width:100%}
  .msg.error{background:color-mix(in srgb,var(--err) 15%,transparent);border:1px solid color-mix(in srgb,var(--err) 30%,transparent);align-self:flex-start}
  .msg.system{background:transparent;align-self:center;color:var(--muted);font-size:12px;padding:4px 0}
  .lbl{font-size:11px;font-weight:600;color:var(--muted);margin-bottom:5px;text-transform:uppercase;letter-spacing:0.3px}
  .msg.user .lbl{color:var(--user)}
  .msg.error .lbl{color:var(--err)}
  .msg.tool .lbl{color:var(--tool)}
  .txt{font-size:14px;line-height:1.65;white-space:pre-wrap}
  .md-content{white-space:normal}
  .md-content :global(.md-h){font-weight:700;margin:8px 0 4px;color:var(--text)}
  .md-content :global(h1.md-h){font-size:20px}
  .md-content :global(h2.md-h){font-size:17px}
  .md-content :global(h3.md-h){font-size:15px}
  .md-content :global(.md-p){margin:4px 0;line-height:1.65}
  .md-content :global(.md-list){margin:4px 0 4px 18px;line-height:1.6}
  .md-content :global(.inline-code){background:#0f172a;border:1px solid var(--border);padding:1px 5px;border-radius:4px;font-family:'SF Mono',Menlo,monospace;font-size:12px}
  .md-content :global(.code-block){background:#0f172a;border:1px solid var(--border);border-radius:6px;padding:12px 14px;margin:8px 0;overflow-x:auto;font-family:'SF Mono',Menlo,monospace;font-size:12px;line-height:1.5;color:#e2e8f0}
  .cursor{animation:blink 1s infinite;color:var(--accent);font-weight:bold}
  @keyframes blink{0%,100%{opacity:1}50%{opacity:0}}
  .thinking{color:var(--muted);animation:pulse 1.5s infinite}
  @keyframes pulse{0%,100%{opacity:1}50%{opacity:0.4}}

  .approval{display:flex;align-items:center;justify-content:space-between;gap:12px;margin:0 20px 4px;padding:10px 14px;background:color-mix(in srgb,var(--tool) 14%,transparent);border:1px solid color-mix(in srgb,var(--tool) 40%,transparent);border-radius:8px;flex-shrink:0}
  .approval-info{display:flex;flex-direction:column;gap:4px;min-width:0;flex:1}
  .approval-tool{font-size:12px;font-weight:600;color:var(--tool);text-transform:uppercase}
  .approval-input{font-family:'SF Mono',Menlo,monospace;font-size:12px;color:var(--text);overflow:hidden;text-overflow:ellipsis;white-space:nowrap;background:var(--bg);padding:4px 8px;border-radius:4px}
  .approval.continue{background:color-mix(in srgb,var(--accent) 12%,transparent);border-color:color-mix(in srgb,var(--accent) 40%,transparent)}
  .approval.continue .approval-tool{color:var(--accent)}
  .continue-sub{font-size:13px;color:var(--text)}
  .approval-actions{display:flex;gap:8px;flex-shrink:0}
  .appr{padding:8px 18px;border:none;border-radius:6px;font-weight:600;font-size:13px;cursor:pointer}
  .appr.allow{background:var(--accent);color:#fff}
  .appr.deny{background:var(--bg-panel);color:var(--err);border:1px solid color-mix(in srgb,var(--err) 40%,transparent)}

  .attachments{display:flex;flex-wrap:wrap;gap:6px;padding:8px 20px 0;flex-shrink:0}
  .chip{display:flex;align-items:center;gap:5px;background:var(--bg-input);border:1px solid var(--border);border-radius:6px;padding:4px 10px;font-size:12px;color:var(--muted)}
  .chip span{max-width:140px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .chip button{background:none;border:none;color:var(--muted);cursor:pointer;font-size:11px}

  .input-area{padding:12px 20px 14px;border-top:1px solid var(--border);background:var(--bg);flex-shrink:0}
  .input-row{display:flex;gap:8px;align-items:flex-end}
  .input-wrap{flex:1;position:relative;min-width:0}
  .file-suggest{position:absolute;bottom:100%;left:0;right:0;margin-bottom:4px;background:#0c1322;border:1px solid var(--border);border-radius:8px;max-height:180px;overflow-y:auto;z-index:20;box-shadow:0 8px 24px rgba(0,0,0,0.4)}
  .file-opt{display:flex;flex-direction:column;align-items:flex-start;width:100%;padding:8px 12px;background:none;border:none;border-bottom:1px solid #1e293b;color:var(--text);cursor:pointer;text-align:left}
  .file-opt:last-child{border-bottom:none}
  .file-opt:hover,.file-opt.sel{background:#1e293b}
  .file-name{font-size:12px;font-family:'SF Mono',Menlo,monospace;color:#93c5fd}
  .file-dir{font-size:10px;color:var(--muted)}
  .attach-btn{background:var(--bg-input);border:1px solid var(--border);border-radius:8px;font-size:18px;padding:8px 10px;cursor:pointer;flex-shrink:0}
  textarea{flex:1;width:100%;padding:10px 14px;background:var(--bg-input);border:1px solid var(--border);border-radius:8px;color:var(--text);font-size:14px;font-family:inherit;resize:none;outline:none;line-height:1.5}
  textarea:focus{border-color:var(--accent)}
  .btn-col{display:flex;flex-direction:column;gap:4px;flex-shrink:0}
  .btn{padding:10px 22px;border:none;border-radius:8px;font-weight:600;font-size:13px;cursor:pointer}
  .btn.send{background:var(--accent);color:white}
  .btn.send:disabled{opacity:0.4;cursor:not-allowed}
  .btn.stop{background:var(--err);color:white}
  .btn.new-chat{background:var(--bg-panel);color:var(--muted);border:1px solid var(--border);padding:6px 12px;font-size:16px}
  .footer-bar{display:flex;justify-content:space-between;margin-top:8px;font-size:11px}
  .cost-info{color:var(--tool);font-family:'SF Mono',Menlo,monospace}
  .hint-txt{color:var(--muted)}
</style>
