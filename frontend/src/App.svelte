<script>
  import { onMount, tick } from 'svelte';
  import Sidebar from './components/Sidebar.svelte';
  import TopBar from './components/TopBar.svelte';
  import Settings from './components/Settings.svelte';

  let agents = [];
  let currentSession = null;
  let currentAgent = null;
  let messages = [];
  let inputValue = '';
  let streaming = false;
  let streamBuffer = '';
  let cost = { inputTokens: 0, outputTokens: 0, cost: 0 };
  let messagesEl;
  let errorBanner = '';
  let showSettings = false;
  let attachedFiles = [];
  let moeRoute = null;
  let contextUsage = 0;
  let pendingApproval = null;

  onMount(async () => {
    await waitForWails();
    try {
      const themes = await window.go.desktop.App.GetThemes();
      const saved = localStorage.getItem('theme') || 'default';
      const t = themes.find(x => x.name === saved) || themes[0];
      if (t) applyTheme(t);
      agents = await window.go.desktop.App.ListAgents();
    } catch (e) {
      errorBanner = 'Failed to load: ' + e;
    }
    registerEvents();
  });

  function waitForWails() {
    return new Promise(resolve => {
      const t = setInterval(() => {
        if (window.go?.desktop?.App?.ListAgents) { clearInterval(t); resolve(); }
      }, 50);
    });
  }

  function applyTheme(t) {
    const r = document.documentElement.style;
    r.setProperty('--bg', t.bg);
    r.setProperty('--bg-panel', t.bgPanel);
    r.setProperty('--bg-input', t.bgInput);
    r.setProperty('--border', t.border);
    r.setProperty('--user', t.user);
    r.setProperty('--tool', t.tool);
    r.setProperty('--err', t.error);
    r.setProperty('--accent', t.accent);
    r.setProperty('--text', t.text);
    r.setProperty('--muted', t.muted);
  }

  function registerEvents() {
    window.runtime.EventsOn('stream', (data) => {
      if (data.sessionId !== currentSession) return;
      streamBuffer = streamBuffer + data.text;
      scrollToBottom();
    });
    window.runtime.EventsOn('done', (data) => {
      if (data.sessionId !== currentSession) return;
      if (streamBuffer) {
        messages = [...messages, { role: 'assistant', content: streamBuffer }];
        streamBuffer = '';
      }
      streaming = false;
      cost = { inputTokens: data.inputTokens || 0, outputTokens: data.outputTokens || 0, cost: data.cost || 0 };
      if (data.contextUsage) contextUsage = data.contextUsage;
      scrollToBottom();
    });
    window.runtime.EventsOn('error', (data) => {
      if (data.sessionId !== currentSession) return;
      streaming = false; streamBuffer = '';
      messages = [...messages, { role: 'error', content: data.error }];
    });
    window.runtime.EventsOn('toolConfirm', (data) => {
      if (data.sessionId !== currentSession) return;
      pendingApproval = { requestId: data.requestId, tool: data.tool, input: data.input || '' };
      scrollToBottom();
    });
    window.runtime.EventsOn('route', (data) => {
      if (data.sessionId !== currentSession) return;
      moeRoute = { category: data.category, model: data.model };
      messages = [...messages, { role: 'system', content: '◆ routed → ' + data.category + ' (' + data.model + ')' }];
      scrollToBottom();
    });
  }

  function renderMarkdown(text) {
    if (!text) return '';
    let html = '';
    const lines = text.split('\n');
    let i = 0;
    while (i < lines.length) {
      const line = lines[i];
      // Fenced code block
      const fence = line.match(/^```(\w*)/);
      if (fence) {
        const lang = fence[1] || '';
        let code = '';
        i++;
        while (i < lines.length && !lines[i].startsWith('```')) {
          code += (code ? '\n' : '') + lines[i];
          i++;
        }
        i++; // skip closing ```
        const langClass = lang ? ' lang-' + lang : '';
        html += `<pre class="code-block${langClass}"><code>${escHtml(code)}</code></pre>`;
        continue;
      }
      // Header
      const hMatch = line.match(/^(#{1,3})\s+(.+)/);
      if (hMatch) {
        const level = hMatch[1].length;
        html += `<h${level} class="md-h">${inlineMd(hMatch[2])}</h${level}>`;
        i++; continue;
      }
      // Bullet list
      if (line.match(/^\s*[-*]\s+/)) {
        html += '<ul class="md-list">';
        while (i < lines.length && lines[i].match(/^\s*[-*]\s+/)) {
          html += `<li>${inlineMd(lines[i].replace(/^\s*[-*]\s+/, ''))}</li>`;
          i++;
        }
        html += '</ul>';
        continue;
      }
      // Empty line
      if (line.trim() === '') { html += '<br/>'; i++; continue; }
      // Paragraph
      html += `<p class="md-p">${inlineMd(line)}</p>`;
      i++;
    }
    return html;
  }

  function inlineMd(text) {
    let s = escHtml(text);
    s = s.replace(/`([^`]+)`/g, '<code class="inline-code">$1</code>');
    s = s.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
    s = s.replace(/\*([^*]+)\*/g, '<em>$1</em>');
    s = s.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" class="md-link">$1</a>');
    return s;
  }

  function escHtml(s) {
    return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
  }

  async function selectAgent(agent) {
    errorBanner = '';
    try {
      if (currentSession) {
        await window.go.desktop.App.SwitchAgent(currentSession, agent.name);
        currentAgent = agent;
        messages = [...messages, { role: 'system', content: '◆ Switched to ' + agent.name }];
      } else {
        const session = await window.go.desktop.App.CreateSession(agent.name);
        currentSession = session.id;
        currentAgent = agent;
        messages = []; streamBuffer = '';
        cost = { inputTokens: 0, outputTokens: 0, cost: 0 };
        moeRoute = null; contextUsage = 0; pendingApproval = null;
      }
    } catch (e) { errorBanner = String(e); }
  }

  async function newChat() {
    if (!currentAgent) return;
    try {
      const session = await window.go.desktop.App.CreateSession(currentAgent.name);
      currentSession = session.id;
      messages = []; streamBuffer = '';
      cost = { inputTokens: 0, outputTokens: 0, cost: 0 };
      moeRoute = null; contextUsage = 0; pendingApproval = null;
    } catch (e) { errorBanner = String(e); }
  }

  async function attachFile() {
    try {
      const file = await window.go.desktop.App.OpenFile();
      if (file && file.name && file.content !== undefined) {
        attachedFiles = [...attachedFiles, { name: file.name, content: file.content }];
      }
    } catch (e) { errorBanner = 'File error: ' + String(e); }
  }

  function removeFile(name) { attachedFiles = attachedFiles.filter(f => f.name !== name); }

  async function sendMessage() {
    const msg = inputValue.trim();
    if (!msg && attachedFiles.length === 0) return;
    if (!currentSession || streaming) return;
    let fullMsg = msg;
    for (const f of attachedFiles) {
      fullMsg += `\n\n[File: ${f.name}]\n\`\`\`\n${f.content}\n\`\`\``;
    }
    const displayMsg = msg + (attachedFiles.length > 0 ? ` [+${attachedFiles.length} file(s)]` : '');
    inputValue = ''; attachedFiles = [];
    messages = [...messages, { role: 'user', content: displayMsg }];
    streaming = true; streamBuffer = '';
    await tick(); scrollToBottom();
    try {
      await window.go.desktop.App.SendMessage(currentSession, fullMsg);
    } catch (e) {
      streaming = false;
      messages = [...messages, { role: 'error', content: String(e) }];
    }
  }

  async function stopGen() {
    try { await window.go.desktop.App.StopGeneration(currentSession); } catch (_) {}
    pendingApproval = null;
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
    try { await window.go.desktop.App.RespondToolApproval(requestId, approved); } catch (e) { errorBanner = String(e); }
    scrollToBottom();
  }

  function handleKey(e) {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); sendMessage(); }
  }

  function scrollToBottom() {
    tick().then(() => { if (messagesEl) messagesEl.scrollTop = messagesEl.scrollHeight; });
  }

  function fmt(n) { return (n||0) >= 1000 ? ((n||0)/1000).toFixed(1)+'k' : String(n||0); }
  function fmtCost(c) { if (!c) return '$0.00'; return c < 0.01 ? '$'+c.toFixed(4) : '$'+c.toFixed(2); }
</script>

{#if showSettings}
  <Settings
    on:close={() => showSettings = false}
    on:theme={e => applyTheme(e.detail)}
  />
{:else}
  <div class="app">
    <TopBar agent={currentAgent} {cost} {moeRoute} {contextUsage} on:settings={() => showSettings = true} />

    <div class="body">
      <Sidebar {agents} active={currentAgent} on:select={e => selectAgent(e.detail)} on:settings={() => showSettings = true} />

      <div class="main">
        {#if errorBanner}
          <div class="err-banner">⚠ {errorBanner} <button on:click={() => errorBanner = ''}>✕</button></div>
        {/if}

        {#if !currentSession}
          <div class="welcome">
            <svg viewBox="0 0 32 32" width="80" height="80">
              <rect width="32" height="32" rx="6" fill="#4f46e5"/>
              <rect x="4" y="4" width="6" height="24" rx="1" fill="#fff"/>
              <rect x="22" y="4" width="6" height="24" rx="1" fill="#fff"/>
              <rect x="10" y="4" width="6" height="6" rx="1" fill="#fff"/>
              <rect x="12" y="10" width="4" height="4" rx="1" fill="#fff"/>
              <rect x="16" y="4" width="6" height="6" rx="1" fill="#fff"/>
              <rect x="16" y="10" width="4" height="4" rx="1" fill="#fff"/>
              <rect x="14" y="12" width="4" height="4" rx="1" fill="#fff"/>
            </svg>
            <h1>AgentCTL</h1>
            <p>Select an agent from the sidebar to start</p>
          </div>
        {:else}
          <div class="messages" bind:this={messagesEl}>
            {#each messages as msg (msg)}
              <div class="msg {msg.role}">
                <div class="lbl">
                  {#if msg.role === 'user'}» You
                  {:else if msg.role === 'assistant'}◆ {currentAgent?.name || 'Agent'}
                  {:else if msg.role === 'tool'}⚙ Tool
                  {:else if msg.role === 'error'}⚠ Error
                  {:else}ℹ{/if}
                </div>
                {#if msg.role === 'assistant'}
                  <div class="txt md-content">{@html renderMarkdown(msg.content)}</div>
                {:else}
                  <div class="txt">{msg.content}</div>
                {/if}
              </div>
            {/each}
            {#if streaming}
              <div class="msg assistant">
                <div class="lbl">◆ {currentAgent?.name || 'Agent'}</div>
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

          {#if attachedFiles.length > 0}
            <div class="attachments">
              {#each attachedFiles as f}
                <div class="chip">📄 <span>{f.name}</span> <button on:click={() => removeFile(f.name)}>✕</button></div>
              {/each}
            </div>
          {/if}

          <div class="input-area">
            <div class="input-row">
              <button class="attach-btn" on:click={attachFile} disabled={streaming} title="Attach file">📎</button>
              <textarea bind:value={inputValue} on:keydown={handleKey}
                placeholder="Type a message... (Enter to send, Shift+Enter for new line)"
                disabled={streaming} rows="3"></textarea>
              <div class="btn-col">
                <button class="btn new-chat" on:click={newChat} title="New Chat">＋</button>
                {#if streaming}
                  <button class="btn stop" on:click={stopGen}>■ Stop</button>
                {:else}
                  <button class="btn send" on:click={sendMessage} disabled={!inputValue.trim() && attachedFiles.length === 0}>Send</button>
                {/if}
              </div>
            </div>
            <div class="footer-bar">
              <span class="cost-info">In: {fmt(cost.inputTokens)} · Out: {fmt(cost.outputTokens)} · {fmtCost(cost.cost)}</span>
              <span class="hint-txt">Enter to send · Shift+Enter new line · 📎 attach file</span>
            </div>
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  :global(*){box-sizing:border-box;margin:0;padding:0}
  :global(:root){
    --bg:#0f172a;--bg-panel:#1e293b;--bg-input:#1e293b;--border:#334155;
    --user:#5f87ff;--tool:#d7af00;--err:#ff5f5f;--accent:#3b82f6;
    --text:#e2e8f0;--muted:#64748b;
  }
  :global(body){font-family:-apple-system,BlinkMacSystemFont,'SF Pro Text',sans-serif;background:var(--bg);color:var(--text);overflow:hidden;height:100vh}

  .app{display:flex;flex-direction:column;height:100vh;overflow:hidden}
  .body{display:flex;flex:1;overflow:hidden;min-height:0}
  .main{flex:1;display:flex;flex-direction:column;overflow:hidden;min-width:0;min-height:0}

  .err-banner{background:#7f1d1d;color:#fca5a5;padding:8px 16px;font-size:13px;display:flex;justify-content:space-between;align-items:center;flex-shrink:0}
  .err-banner button{background:none;border:none;color:#fca5a5;cursor:pointer;font-size:16px;padding:0 4px}

  .welcome{flex:1;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:12px;color:var(--muted)}
  .welcome h1{font-size:28px;color:var(--text);font-weight:700}
  .welcome p{font-size:15px}

  .messages{flex:1;overflow-y:auto;padding:20px;display:flex;flex-direction:column;gap:14px;min-height:0}

  .msg{padding:12px 16px;border-radius:8px;max-width:82%;word-break:break-word;flex-shrink:0}
  .msg.user{background:color-mix(in srgb,var(--user) 18%,transparent);align-self:flex-end;border:1px solid color-mix(in srgb,var(--user) 35%,transparent)}
  .msg.assistant{background:var(--bg-input);align-self:flex-start}
  .msg.tool{background:var(--bg-panel);align-self:flex-start;font-family:'SF Mono',Menlo,monospace;font-size:12px;color:var(--muted);max-width:100%}
  .msg.error{background:color-mix(in srgb,var(--err) 15%,transparent);border:1px solid color-mix(in srgb,var(--err) 30%,transparent);align-self:flex-start}
  .msg.system{background:transparent;align-self:center;color:var(--muted);font-size:12px;padding:4px 0}

  .lbl{font-size:11px;font-weight:600;color:var(--muted);margin-bottom:5px;text-transform:uppercase;letter-spacing:0.3px;display:flex;align-items:center;gap:6px}
  .msg.user .lbl{color:var(--user)}
  .msg.error .lbl{color:var(--err)}
  .msg.tool .lbl{color:var(--tool)}
  .txt{font-size:14px;line-height:1.65;white-space:pre-wrap}

  .moe-badge{font-size:10px;background:#6366f1;color:#fff;padding:1px 6px;border-radius:4px;text-transform:none;letter-spacing:0;font-weight:500}

  /* Markdown rendered content */
  .md-content{white-space:normal}
  .md-content :global(.md-h){font-weight:700;margin:8px 0 4px;color:var(--text)}
  .md-content :global(h1.md-h){font-size:20px}
  .md-content :global(h2.md-h){font-size:17px}
  .md-content :global(h3.md-h){font-size:15px}
  .md-content :global(.md-p){margin:4px 0;line-height:1.65}
  .md-content :global(.md-list){margin:4px 0 4px 18px;line-height:1.6}
  .md-content :global(.md-list li){margin:2px 0}
  .md-content :global(.md-link){color:var(--accent);text-decoration:underline}
  .md-content :global(.inline-code){background:#0f172a;border:1px solid var(--border);padding:1px 5px;border-radius:4px;font-family:'SF Mono',Menlo,monospace;font-size:12px}
  .md-content :global(strong){font-weight:700;color:#f1f5f9}
  .md-content :global(em){font-style:italic;color:#cbd5e1}

  /* Code blocks */
  .md-content :global(.code-block){background:#0f172a;border:1px solid var(--border);border-radius:6px;padding:12px 14px;margin:8px 0;overflow-x:auto;font-family:'SF Mono',Menlo,monospace;font-size:12px;line-height:1.5;color:#e2e8f0}
  .md-content :global(.code-block code){font-family:inherit;font-size:inherit;color:inherit}
  .md-content :global(.code-block.lang-go){border-left:3px solid #00add8}
  .md-content :global(.code-block.lang-python){border-left:3px solid #3776ab}
  .md-content :global(.code-block.lang-javascript),
  .md-content :global(.code-block.lang-js){border-left:3px solid #f7df1e}
  .md-content :global(.code-block.lang-typescript),
  .md-content :global(.code-block.lang-ts){border-left:3px solid #3178c6}
  .md-content :global(.code-block.lang-rust){border-left:3px solid #dea584}
  .md-content :global(.code-block.lang-bash),
  .md-content :global(.code-block.lang-sh),
  .md-content :global(.code-block.lang-shell){border-left:3px solid #4ade80}
  .md-content :global(.code-block.lang-yaml),
  .md-content :global(.code-block.lang-yml){border-left:3px solid #cb171e}
  .md-content :global(.code-block.lang-json){border-left:3px solid #a3a3a3}
  .md-content :global(.code-block.lang-sql){border-left:3px solid #e38c00}
  .md-content :global(.code-block.lang-css){border-left:3px solid #264de4}
  .md-content :global(.code-block.lang-html){border-left:3px solid #e34c26}

  .cursor{animation:blink 1s infinite;color:var(--accent);font-weight:bold}
  @keyframes blink{0%,100%{opacity:1}50%{opacity:0}}
  .thinking{color:var(--muted);animation:pulse 1.5s infinite}
  @keyframes pulse{0%,100%{opacity:1}50%{opacity:0.4}}

  .approval{display:flex;align-items:center;justify-content:space-between;gap:12px;margin:0 20px 4px;padding:10px 14px;background:color-mix(in srgb,var(--tool) 14%,transparent);border:1px solid color-mix(in srgb,var(--tool) 40%,transparent);border-radius:8px;flex-shrink:0}
  .approval-info{display:flex;flex-direction:column;gap:4px;min-width:0;flex:1}
  .approval-tool{font-size:12px;font-weight:600;color:var(--tool);text-transform:uppercase;letter-spacing:0.3px}
  .approval-input{font-family:'SF Mono',Menlo,monospace;font-size:12px;color:var(--text);overflow:hidden;text-overflow:ellipsis;white-space:nowrap;background:var(--bg);padding:4px 8px;border-radius:4px}
  .approval-actions{display:flex;gap:8px;flex-shrink:0}
  .appr{padding:8px 18px;border:none;border-radius:6px;font-weight:600;font-size:13px;cursor:pointer}
  .appr.allow{background:var(--accent);color:#fff}
  .appr.allow:hover{filter:brightness(1.1)}
  .appr.deny{background:var(--bg-panel);color:var(--err);border:1px solid color-mix(in srgb,var(--err) 40%,transparent)}
  .appr.deny:hover{background:color-mix(in srgb,var(--err) 15%,transparent)}

  .attachments{display:flex;flex-wrap:wrap;gap:6px;padding:8px 20px 0;flex-shrink:0}
  .chip{display:flex;align-items:center;gap:5px;background:var(--bg-input);border:1px solid var(--border);border-radius:6px;padding:4px 10px;font-size:12px;color:var(--muted)}
  .chip span{max-width:140px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .chip button{background:none;border:none;color:var(--muted);cursor:pointer;font-size:11px;padding:0 2px}
  .chip button:hover{color:var(--err)}

  .input-area{padding:12px 20px 14px;border-top:1px solid var(--border);background:var(--bg);flex-shrink:0}
  .input-row{display:flex;gap:8px;align-items:flex-end}

  .attach-btn{background:var(--bg-input);border:1px solid var(--border);border-radius:8px;font-size:18px;padding:8px 10px;cursor:pointer;flex-shrink:0;align-self:flex-end;transition:background 0.15s}
  .attach-btn:hover:not(:disabled){background:var(--border)}
  .attach-btn:disabled{opacity:0.4;cursor:not-allowed}

  textarea{flex:1;padding:10px 14px;background:var(--bg-input);border:1px solid var(--border);border-radius:8px;color:var(--text);font-size:14px;font-family:inherit;resize:none;outline:none;line-height:1.5}
  textarea:focus{border-color:var(--accent)}
  textarea:disabled{opacity:0.5;cursor:not-allowed}

  .btn-col{display:flex;flex-direction:column;justify-content:flex-end;flex-shrink:0;gap:4px}
  .btn{padding:10px 22px;border:none;border-radius:8px;font-weight:600;font-size:13px;cursor:pointer;white-space:nowrap}
  .btn.send{background:var(--accent);color:white}
  .btn.send:hover:not(:disabled){filter:brightness(1.1)}
  .btn.send:disabled{opacity:0.4;cursor:not-allowed}
  .btn.stop{background:var(--err);color:white}
  .btn.stop:hover{filter:brightness(1.1)}
  .btn.new-chat{background:var(--bg-panel);color:var(--muted);border:1px solid var(--border);padding:6px 12px;font-size:16px}
  .btn.new-chat:hover{background:var(--border);color:var(--text)}

  .footer-bar{display:flex;justify-content:space-between;align-items:center;margin-top:8px;font-size:11px}
  .cost-info{color:var(--tool);font-family:'SF Mono',Menlo,monospace;font-weight:500}
  .hint-txt{color:var(--muted)}
</style>
