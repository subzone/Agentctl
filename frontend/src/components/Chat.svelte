<script>
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  
  export let sessionId;
  
  const dispatch = createEventDispatcher();
  
  let messages = [];
  let toolActivity = [];
  let inputValue = '';
  let streaming = false;
  let streamBuffer = '';
  let trustMode = false;
  let messagesEl;
  let unsubscribers = [];
  
  $: if (sessionId) {
    messages = [];
    toolActivity = [];
    streamBuffer = '';
  }
  
  onMount(() => {
    setupEventListeners();
  });
  
  onDestroy(() => {
    unsubscribers.forEach(fn => fn());
  });
  
  async function setupEventListeners() {
    try {
      const { EventsOn } = await import('../wailsjs/runtime/runtime');
      
      unsubscribers.push(EventsOn('stream', (data) => {
        if (data.sessionId !== sessionId) return;
        streamBuffer += data.text;
        scrollToBottom();
      }));
      
      unsubscribers.push(EventsOn('done', (data) => {
        if (data.sessionId !== sessionId) return;
        if (streamBuffer) {
          messages = [...messages, {
            id: 'msg_' + Date.now(),
            role: 'assistant',
            content: streamBuffer,
            timestamp: Math.floor(Date.now() / 1000)
          }];
          streamBuffer = '';
        }
        streaming = false;
        dispatch('costUpdate', {
          inputTokens: data.inputTokens,
          outputTokens: data.outputTokens,
          cost: 0, // calculated on frontend from pricing
          model: ''
        });
      }));
      
      unsubscribers.push(EventsOn('message', (data) => {
        if (data.sessionId !== sessionId) return;
        if (data.role === 'user') {
          messages = [...messages, {
            id: 'msg_' + Date.now(),
            role: 'user',
            content: data.content,
            timestamp: Math.floor(Date.now() / 1000)
          }];
        }
      }));
      
      unsubscribers.push(EventsOn('toolConfirm', (data) => {
        if (data.sessionId !== sessionId) return;
        toolActivity = [...toolActivity, {
          id: 'tool_' + Date.now(),
          name: data.tool,
          input: data.input,
          status: trustMode ? 'done' : 'pending',
          startTime: Date.now()
        }];
      }));
      
      unsubscribers.push(EventsOn('error', (data) => {
        if (data.sessionId !== sessionId) return;
        streaming = false;
        messages = [...messages, {
          id: 'msg_err_' + Date.now(),
          role: 'error',
          content: data.error,
          timestamp: Math.floor(Date.now() / 1000)
        }];
      }));
    } catch (err) {
      console.error('Failed to setup events:', err);
    }
  }
  
  async function sendMessage() {
    if (!inputValue.trim() || !sessionId || streaming) return;
    
    const message = inputValue.trim();
    inputValue = '';
    streaming = true;
    streamBuffer = '';
    
    try {
      const { SendMessage } = await import('../wailsjs/go/desktop/App');
      await SendMessage(sessionId, message);
    } catch (err) {
      streaming = false;
      console.error('Failed to send:', err);
    }
  }
  
  async function stopGeneration() {
    try {
      const { StopGeneration } = await import('../wailsjs/go/desktop/App');
      await StopGeneration(sessionId);
    } catch (err) {}
    streaming = false;
  }
  
  function handleKeyDown(e) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  }
  
  function handleFileDrop(e) {
    e.preventDefault();
    const files = e.dataTransfer?.files;
    if (!files || files.length === 0) return;
    for (const file of files) {
      inputValue += `@${file.name} `;
    }
  }
  
  function scrollToBottom() {
    if (messagesEl) {
      requestAnimationFrame(() => {
        messagesEl.scrollTop = messagesEl.scrollHeight;
      });
    }
  }
</script>

<div class="chat">
  {#if !sessionId}
    <div class="welcome">
      <div class="welcome-icon">◆</div>
      <h2>AgentCTL</h2>
      <p>Select an agent from the sidebar to start</p>
      <div class="shortcuts">
        <kbd>Enter</kbd> send &nbsp; <kbd>Shift+Enter</kbd> new line &nbsp; <kbd>@file</kbd> attach
      </div>
    </div>
  {:else}
    <div class="chat-body">
      <div class="messages" bind:this={messagesEl}>
        {#each messages as msg}
          <div class="message {msg.role}">
            <div class="msg-header">
              <span class="msg-role">{msg.role === 'user' ? '» You' : msg.role === 'error' ? '⚠ Error' : '◆ Agent'}</span>
              <span class="msg-time">{new Date(msg.timestamp * 1000).toLocaleTimeString()}</span>
            </div>
            <div class="msg-content">{msg.content}</div>
          </div>
        {/each}
        
        {#if streaming && streamBuffer}
          <div class="message assistant streaming">
            <div class="msg-header">
              <span class="msg-role">◆ Agent</span>
              <span class="streaming-indicator">● streaming</span>
            </div>
            <div class="msg-content">{streamBuffer}</div>
          </div>
        {:else if streaming}
          <div class="message assistant thinking">
            <div class="msg-header">
              <span class="msg-role">◆ Agent</span>
            </div>
            <div class="msg-content thinking-dots">Thinking<span>...</span></div>
          </div>
        {/if}
      </div>
      
      {#if toolActivity.length > 0}
        <div class="tool-panel">
          <div class="tool-panel-header">Tool Activity</div>
          {#each toolActivity.slice(-5) as tool}
            <div class="tool-item {tool.status}">
              <span class="tool-status">
                {#if tool.status === 'running'}⏳
                {:else if tool.status === 'done'}✓
                {:else if tool.status === 'error'}✗
                {:else}?{/if}
              </span>
              <span class="tool-name">{tool.name}</span>
              <span class="tool-input">{tool.input?.substring(0, 40)}</span>
            </div>
          {/each}
        </div>
      {/if}
    </div>
    
    <div class="input-area" on:drop={handleFileDrop} on:dragover|preventDefault>
      <div class="input-row">
        <textarea
          bind:value={inputValue}
          on:keydown={handleKeyDown}
          placeholder="Type a message... (Shift+Enter for new line, @file to attach)"
          rows="2"
          disabled={streaming}
        />
        <div class="input-actions">
          {#if streaming}
            <button class="stop-btn" on:click={stopGeneration}>■ Stop</button>
          {:else}
            <button class="send-btn" on:click={sendMessage} disabled={!inputValue.trim()}>
              Send
            </button>
          {/if}
        </div>
      </div>
      <div class="input-footer">
        <label class="trust-toggle">
          <input type="checkbox" bind:checked={trustMode} />
          <span>⚡ Trust</span>
        </label>
        <span class="hint">Drop files to attach • @path to reference</span>
      </div>
    </div>
  {/if}
</div>

<style>
  .chat {
    display: flex;
    flex-direction: column;
    height: 100%;
    background-color: #0f172a;
  }
  
  .welcome {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    color: #64748b;
  }
  
  .welcome-icon { font-size: 48px; color: #3b82f6; }
  .welcome h2 { color: #e2e8f0; margin: 0; }
  .welcome p { margin: 0; }
  .shortcuts { font-size: 12px; margin-top: 16px; }
  .shortcuts kbd {
    background: #1e293b;
    padding: 2px 6px;
    border-radius: 3px;
    border: 1px solid #334155;
    font-size: 11px;
  }
  
  .chat-body {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  
  .messages {
    flex: 1;
    overflow-y: auto;
    padding: 16px 20px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  
  .message {
    padding: 12px 16px;
    border-radius: 8px;
    max-width: 85%;
  }
  
  .message.user {
    background-color: #1e3a5f;
    align-self: flex-end;
    border: 1px solid #2563eb33;
  }
  
  .message.assistant, .message.streaming, .message.thinking {
    background-color: #1e293b;
    align-self: flex-start;
  }
  
  .message.error {
    background-color: #3b1111;
    border: 1px solid #ef444433;
    align-self: flex-start;
  }
  
  .msg-header {
    display: flex;
    justify-content: space-between;
    margin-bottom: 6px;
    font-size: 12px;
  }
  
  .msg-role { font-weight: 600; color: #94a3b8; }
  .msg-time { color: #475569; }
  .streaming-indicator { color: #4ade80; animation: pulse 1.5s infinite; }
  
  .msg-content {
    font-size: 14px;
    line-height: 1.6;
    white-space: pre-wrap;
    word-break: break-word;
  }
  
  .thinking-dots span { animation: dots 1.5s infinite; }
  
  @keyframes pulse { 0%,100% { opacity: 1; } 50% { opacity: 0.4; } }
  @keyframes dots { 0% { content: '.'; } 33% { content: '..'; } 66% { content: '...'; } }
  
  .tool-panel {
    border-top: 1px solid #1e293b;
    padding: 8px 16px;
    max-height: 120px;
    overflow-y: auto;
    background-color: #0c1322;
  }
  
  .tool-panel-header {
    font-size: 11px;
    text-transform: uppercase;
    color: #475569;
    margin-bottom: 6px;
    font-weight: 600;
  }
  
  .tool-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 3px 0;
    font-size: 12px;
    color: #94a3b8;
  }
  
  .tool-item.running { color: #fbbf24; }
  .tool-item.done { color: #4ade80; }
  .tool-item.error { color: #ef4444; }
  
  .tool-name { font-weight: 500; }
  .tool-input { color: #475569; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  
  .input-area {
    padding: 12px 16px;
    border-top: 1px solid #1e293b;
    background-color: #0f172a;
  }
  
  .input-row {
    display: flex;
    gap: 8px;
  }
  
  textarea {
    flex: 1;
    padding: 10px 14px;
    background-color: #1e293b;
    border: 1px solid #334155;
    border-radius: 8px;
    color: #e2e8f0;
    font-family: inherit;
    font-size: 14px;
    resize: none;
    outline: none;
    transition: border-color 0.2s;
  }
  
  textarea:focus { border-color: #3b82f6; }
  textarea:disabled { opacity: 0.5; }
  
  .input-actions {
    display: flex;
    flex-direction: column;
    justify-content: flex-end;
  }
  
  .send-btn, .stop-btn {
    padding: 8px 20px;
    border: none;
    border-radius: 6px;
    font-weight: 600;
    font-size: 13px;
    cursor: pointer;
    transition: all 0.2s;
  }
  
  .send-btn {
    background-color: #3b82f6;
    color: white;
  }
  .send-btn:hover:not(:disabled) { background-color: #2563eb; }
  .send-btn:disabled { opacity: 0.4; cursor: not-allowed; }
  
  .stop-btn {
    background-color: #dc2626;
    color: white;
  }
  .stop-btn:hover { background-color: #b91c1c; }
  
  .input-footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-top: 6px;
    font-size: 11px;
    color: #475569;
  }
  
  .trust-toggle {
    display: flex;
    align-items: center;
    gap: 4px;
    cursor: pointer;
    color: #64748b;
  }
  
  .trust-toggle input { width: 14px; height: 14px; }
  .trust-toggle input:checked + span { color: #fbbf24; }
</style>
