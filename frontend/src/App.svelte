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

  onMount(async () => {
    await waitForWails();
    // Apply saved theme
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
      scrollToBottom();
    });
    window.runtime.EventsOn('error', (data) => {
      if (data.sessionId !== currentSession) return;
      streaming = false; streamBuffer = '';
      messages = [...messages, { role: 'error', content: data.error }];
    });
    window.runtime.EventsOn('toolConfirm', (data) => {
      if (data.sessionId !== currentSession) return;
      messages = [...messages, { role: 'tool', content: '→ ' + data.tool + ' ' + (data.input || '').substring(0, 80) }];
      scrollToBottom();
    });
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
      }
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
    streaming = false;
    if (streamBuffer) {
      messages = [...messages, { role: 'assistant', content: streamBuffer + '\n[stopped]' }];
      streamBuffer = '';
    }
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
    <TopBar agent={currentAgent} {cost} on:settings={() => showSettings = true} />

    <div class="body">
      <Sidebar {agents} on:select={e => selectAgent(e.detail)} on:settings={() => showSettings = true} />

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
                <div class="txt">{msg.content}</div>
              </div>
            {/each}
            {#if streaming}
              <div class="msg assistant">
                <div class="lbl">◆ {currentAgent?.name || 'Agent'}</div>
                <div class="txt">
                  {#if streamBuffer}{streamBuffer}<span class="cursor">▊</span>
                  {:else}<span class="thinking">Thinking...</span>{/if}
                </div>
              </div>
            {/if}
          </div>

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

  .lbl{font-size:11px;font-weight:600;color:var(--muted);margin-bottom:5px;text-transform:uppercase;letter-spacing:0.3px}
  .msg.user .lbl{color:var(--user)}
  .msg.error .lbl{color:var(--err)}
  .msg.tool .lbl{color:var(--tool)}
  .txt{font-size:14px;line-height:1.65;white-space:pre-wrap}

  .cursor{animation:blink 1s infinite;color:var(--accent);font-weight:bold}
  @keyframes blink{0%,100%{opacity:1}50%{opacity:0}}
  .thinking{color:var(--muted);animation:pulse 1.5s infinite}
  @keyframes pulse{0%,100%{opacity:1}50%{opacity:0.4}}

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

  .btn-col{display:flex;flex-direction:column;justify-content:flex-end;flex-shrink:0}
  .btn{padding:10px 22px;border:none;border-radius:8px;font-weight:600;font-size:13px;cursor:pointer;white-space:nowrap}
  .btn.send{background:var(--accent);color:white}
  .btn.send:hover:not(:disabled){filter:brightness(1.1)}
  .btn.send:disabled{opacity:0.4;cursor:not-allowed}
  .btn.stop{background:var(--err);color:white}
  .btn.stop:hover{filter:brightness(1.1)}

  .footer-bar{display:flex;justify-content:space-between;align-items:center;margin-top:8px;font-size:11px}
  .cost-info{color:var(--tool);font-family:'SF Mono',Menlo,monospace;font-weight:500}
  .hint-txt{color:var(--muted)}
</style>
