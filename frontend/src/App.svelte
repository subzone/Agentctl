<script>
  import { onMount, tick } from 'svelte';
  import Sidebar from './components/Sidebar.svelte';
  import KnowledgePanel from './components/KnowledgePanel.svelte';
  import KnowledgeGraph from './components/KnowledgeGraph.svelte';
  import ToolsSidebar from './components/ToolsSidebar.svelte';
  import SkillsSidebar from './components/SkillsSidebar.svelte';
  import MCPSidebar from './components/MCPSidebar.svelte';
  import PersonaPanel from './components/PersonaPanel.svelte';
  import CommandCenter from './components/CommandCenter.svelte';
  import ContextInspector from './components/ContextInspector.svelte';
  import ActivityTimeline from './components/ActivityTimeline.svelte';
  import CommandPalette from './components/CommandPalette.svelte';
  import ToastStack from './components/ToastStack.svelte';
  import ExtensionsNav from './components/ExtensionsNav.svelte';
  import ToolFormEditor from './components/ToolFormEditor.svelte';
  import SkillFormEditor from './components/SkillFormEditor.svelte';
  import MCPFormEditor from './components/MCPFormEditor.svelte';
  import TestBench from './components/TestBench.svelte';
  import Onboarding from './components/Onboarding.svelte';
  import TopBar from './components/TopBar.svelte';
  import Settings from './components/Settings.svelte';

  const KM_COLORS = {
    Repo: '#818cf8', Document: '#38bdf8', Person: '#fbbf24', Tech: '#34d399',
    Service: '#f472b6', Convention: '#c084fc', Chunk: '#64748b', default: '#94a3b8',
  };

  // Left column: 'agents' (chat) or 'knowledge' (graph).
  let leftTab = 'agents';
  let kmHealth = { available: false, error: '' };
  let kmStats = null;
  let kmNodes = [];
  let kmLinks = [];
  let kmNodeCounts = {};
  let kmVisibleTypes = new Set();
  let kmIndexing = null;
  let kmLoaded = false;
  let kmSelectedId = '';
  let graphRef;

  // Custom tools (Tools tab).
  let toolsList = [];
  let toolsLoaded = false;
  let activeTool = null; // { name, isNew }
  let toolContent = '';
  let toolError = '';
  let toolSaving = false;
  let toolSaved = false;
  let toolAdvanced = false;

  // Skills (Skills tab). Authored skill bodies are composed into every new
  // chat's system prompt, so they take effect on the next New Chat.
  let skillsList = [];
  let skillsLoaded = false;
  let activeSkill = null; // { name, isNew }
  let skillContent = '';
  let skillError = '';
  let skillSaving = false;
  let skillSaved = false;
  let skillAdvanced = false;

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

  // MCP servers (MCP tab).
  let mcpList = [];
  let mcpLoaded = false;
  let activeMCP = null; // { name, isNew }
  let mcpContent = '';
  let mcpError = '';
  let mcpSaving = false;
  let mcpSaved = false;
  let mcpAdvanced = false;

  // Per-agent personality overlay (Personality tab).
  let persona = { instructions: '', tone: '', verbosity: '', temperature: null };
  let personaSaved = false;
  let personaKey = 0; // bumped after persona loads to remount the panel
  let attachedFiles = [];
  let moeRoute = null;
  let contextUsage = 0;
  let pendingApproval = null;
  let pendingContinue = null;

  // Command Center + observability
  let healthReport = { ok: true, checks: [] };
  let homeStats = { tools: 0, skills: 0, mcp: 0, km: false };
  let homeSessions = [];
  let contextPreview = null;
  let contextLoading = false;
  let showInspector = true;
  let activityEvents = [];
  let activitySeq = 0;
  let toasts = [];
  let paletteOpen = false;
  let extensionsSubTab = 'tools';
  let showOnboarding = false;

  onMount(async () => {
    await waitForWails();
    try {
      const themes = await window.go.desktop.App.GetThemes();
      const saved = localStorage.getItem('theme') || 'default';
      const t = themes.find(x => x.name === saved) || themes[0];
      if (t) applyTheme(t);
      agents = await window.go.desktop.App.ListAgents();
      await refreshHomeData();
      // Auto-activate the default MoE agent so the app is usable immediately.
      const def = agents.find(a => a.category === 'default') || agents.find(a => a.name === 'm');
      if (def) await selectAgent(def);
      leftTab = 'home';
      if (!localStorage.getItem('agentctl_onboarded_v1')) showOnboarding = true;
    } catch (e) {
      errorBanner = 'Failed to load: ' + e;
    }
    registerEvents();
    window.addEventListener('keydown', onGlobalKey);
    return () => window.removeEventListener('keydown', onGlobalKey);
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
      pushActivity('assistant', 'response complete', { tokens: (data.outputTokens || 0) });
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
      pushActivity('approval', data.tool, { input: (data.input || '').substring(0, 120) });
      scrollToBottom();
    });
    window.runtime.EventsOn('route', (data) => {
      if (data.sessionId !== currentSession) return;
      moeRoute = { category: data.category, model: data.model };
    });

    window.runtime.EventsOn('activity', (data) => {
      if (data.sessionId !== currentSession) return;
      pushActivity(data.kind, data.label, data.detail, data.ts);
    });

    window.runtime.EventsOn('continueConfirm', (data) => {
      if (data.sessionId !== currentSession) return;
      pendingContinue = { requestId: data.requestId, turns: data.turns };
      pushActivity('continue', 'tool-step limit', { turns: data.turns });
      scrollToBottom();
    });

    window.runtime.EventsOn('kmIndexProgress', (d) => {
      kmIndexing = { active: true, current: d.current || 0, total: d.total || 0, file: d.file || '' };
    });
    window.runtime.EventsOn('kmIndexDone', (d) => {
      kmIndexing = { active: false, message: d.message || 'Indexing complete' };
      refreshKnowledge();
    });
    window.runtime.EventsOn('kmIndexError', (d) => {
      kmIndexing = { active: false, error: d.error || 'Indexing failed' };
    });
  }

  async function ensureKnowledge() {
    if (kmLoaded) return;
    kmLoaded = true;
    await refreshKnowledge();
  }

  async function refreshKnowledge() {
    try {
      kmHealth = await window.go.desktop.App.KMHealth();
    } catch (e) {
      kmHealth = { available: false, error: String(e) };
    }
    if (!kmHealth.available) { kmStats = null; kmNodes = []; kmLinks = []; return; }
    try {
      kmStats = await window.go.desktop.App.KMStatus();
      const g = await window.go.desktop.App.KMGraph();
      kmNodes = g.nodes || [];
      kmLinks = g.links || [];
      const counts = {};
      for (const n of kmNodes) counts[n.type] = (counts[n.type] || 0) + 1;
      kmNodeCounts = counts;
      // Default view shows everything except low-signal Chunk nodes.
      kmVisibleTypes = new Set(Object.keys(counts).filter(t => t !== 'Chunk'));
    } catch (e) {
      errorBanner = 'Knowledge graph: ' + String(e);
    }
  }

  function toggleKMType(t) {
    const next = new Set(kmVisibleTypes);
    if (next.has(t)) next.delete(t); else next.add(t);
    kmVisibleTypes = next;
  }

  async function kmIndexFolder() {
    try {
      const path = await window.go.desktop.App.KMPickDirectory();
      if (!path) return;
      kmIndexing = { active: true, current: 0, total: 0, file: '' };
      await window.go.desktop.App.KMIndex(path, 'auto');
    } catch (e) {
      kmIndexing = { active: false, error: String(e) };
    }
  }

  function openKnowledge() {
    leftTab = 'knowledge';
    ensureKnowledge();
  }

  function openExtensions(sub = 'tools') {
    leftTab = 'extensions';
    extensionsSubTab = sub;
    if (sub === 'tools' && !toolsLoaded) { toolsLoaded = true; loadTools(); }
    if (sub === 'skills' && !skillsLoaded) { skillsLoaded = true; loadSkills(); }
    if (sub === 'mcp' && !mcpLoaded) { mcpLoaded = true; loadMCP(); }
  }

  async function loadTools() {
    try { toolsList = await window.go.desktop.App.ListTools(); }
    catch (e) { errorBanner = 'Tools: ' + String(e); }
  }

  function selectTool(td) {
    activeTool = { name: td.name, isNew: false };
    toolContent = td.content;
    toolError = td.error || '';
    toolSaved = false;
  }

  async function newTool() {
    try {
      const tpl = await window.go.desktop.App.NewToolTemplate();
      // Give each new tool a unique default name so saving several in a row
      // doesn't silently overwrite the first (all templates start "my_tool").
      const existing = new Set(toolsList.map(t => t.name));
      let name = 'my_tool';
      for (let i = 2; existing.has(name); i++) name = 'my_tool_' + i;
      toolContent = tpl.replace(/^name:.*$/m, 'name: ' + name);
      activeTool = { name: '', isNew: true };
      toolError = ''; toolSaved = false;
    } catch (e) { errorBanner = String(e); }
  }

  function toolNameFromContent(c) {
    const m = c.match(/^name:\s*(.+)$/m);
    return m ? m[1].trim() : '';
  }

  // Two-way bridge between the dedicated Name field and the frontmatter
  // `name:` line, so renaming is obvious without editing raw YAML. Function
  // replacement avoids `$` in the value being treated as a backreference.
  function setToolName(n) {
    if (/^name:.*$/m.test(toolContent)) {
      toolContent = toolContent.replace(/^name:.*$/m, () => 'name: ' + n);
    } else {
      toolContent = 'name: ' + n + '\n' + toolContent;
    }
  }

  async function saveTool() {
    if (!activeTool) return;
    toolSaving = true; toolError = ''; toolSaved = false;
    try {
      await window.go.desktop.App.SaveTool(activeTool.isNew ? '' : activeTool.name, toolContent);
      activeTool = { name: toolNameFromContent(toolContent) || activeTool.name, isNew: false };
      toolSaved = true;
      toast('Tool saved — New Chat to apply', 'info');
      await loadTools();
      setTimeout(() => toolSaved = false, 2000);
    } catch (e) { toolError = String(e); }
    finally { toolSaving = false; }
  }

  async function deleteTool() {
    if (!activeTool) return;
    if (activeTool.isNew) { activeTool = null; toolContent = ''; return; }
    try {
      await window.go.desktop.App.DeleteTool(activeTool.name);
      activeTool = null; toolContent = ''; toolError = '';
      await loadTools();
    } catch (e) { toolError = String(e); }
  }

  async function loadSkills() {
    try { skillsList = await window.go.desktop.App.ListSkills(); }
    catch (e) { errorBanner = 'Skills: ' + String(e); }
  }

  function selectSkill(sd) {
    activeSkill = { name: sd.name, isNew: false };
    skillContent = sd.content;
    skillError = sd.error || '';
    skillSaved = false;
  }

  async function newSkill() {
    try {
      const tpl = await window.go.desktop.App.NewSkillTemplate();
      const existing = new Set(skillsList.map(s => s.name));
      let name = 'my_skill';
      for (let i = 2; existing.has(name); i++) name = 'my_skill_' + i;
      skillContent = tpl.replace(/^name:.*$/m, 'name: ' + name);
      activeSkill = { name: '', isNew: true };
      skillError = ''; skillSaved = false;
    } catch (e) { errorBanner = String(e); }
  }

  function skillNameFromContent(c) {
    const m = c.match(/^name:\s*(.+)$/m);
    return m ? m[1].trim() : '';
  }

  function setSkillName(n) {
    if (/^name:.*$/m.test(skillContent)) {
      skillContent = skillContent.replace(/^name:.*$/m, () => 'name: ' + n);
    } else {
      skillContent = 'name: ' + n + '\n' + skillContent;
    }
  }

  async function saveSkill() {
    if (!activeSkill) return;
    skillSaving = true; skillError = ''; skillSaved = false;
    try {
      await window.go.desktop.App.SaveSkill(activeSkill.isNew ? '' : activeSkill.name, skillContent);
      activeSkill = { name: skillNameFromContent(skillContent) || activeSkill.name, isNew: false };
      skillSaved = true;
      toast('Skill saved — New Chat to apply', 'info');
      await loadSkills();
      setTimeout(() => skillSaved = false, 2000);
    } catch (e) { skillError = String(e); }
    finally { skillSaving = false; }
  }

  async function deleteSkill() {
    if (!activeSkill) return;
    if (activeSkill.isNew) { activeSkill = null; skillContent = ''; return; }
    try {
      await window.go.desktop.App.DeleteSkill(activeSkill.name);
      activeSkill = null; skillContent = ''; skillError = '';
      await loadSkills();
    } catch (e) { skillError = String(e); }
  }

  async function loadMCP() {
    try { mcpList = await window.go.desktop.App.ListMCP(); }
    catch (e) { errorBanner = 'MCP: ' + String(e); }
  }

  function selectMCP(sd) {
    activeMCP = { name: sd.name, isNew: false };
    mcpContent = sd.content;
    mcpError = sd.error || '';
    mcpSaved = false;
  }

  async function newMCP() {
    try {
      const tpl = await window.go.desktop.App.NewMCPTemplate();
      const existing = new Set(mcpList.map(s => s.name));
      let name = 'my_server';
      for (let i = 2; existing.has(name); i++) name = 'my_server_' + i;
      mcpContent = tpl.replace(/^name:.*$/m, 'name: ' + name);
      activeMCP = { name: '', isNew: true };
      mcpError = ''; mcpSaved = false;
    } catch (e) { errorBanner = String(e); }
  }

  function mcpNameFromContent(c) {
    const m = c.match(/^name:\s*(.+)$/m);
    return m ? m[1].trim() : '';
  }
  function setMCPName(n) {
    if (/^name:.*$/m.test(mcpContent)) {
      mcpContent = mcpContent.replace(/^name:.*$/m, () => 'name: ' + n);
    } else {
      mcpContent = 'name: ' + n + '\n' + mcpContent;
    }
  }

  async function saveMCP() {
    if (!activeMCP) return;
    mcpSaving = true; mcpError = ''; mcpSaved = false;
    try {
      await window.go.desktop.App.SaveMCP(activeMCP.isNew ? '' : activeMCP.name, mcpContent);
      activeMCP = { name: mcpNameFromContent(mcpContent) || activeMCP.name, isNew: false };
      mcpSaved = true;
      toast('MCP server saved — New Chat to apply', 'info');
      await loadMCP();
      setTimeout(() => mcpSaved = false, 2000);
    } catch (e) { mcpError = String(e); }
    finally { mcpSaving = false; }
  }

  async function deleteMCP() {
    if (!activeMCP) return;
    if (activeMCP.isNew) { activeMCP = null; mcpContent = ''; return; }
    try {
      await window.go.desktop.App.DeleteMCP(activeMCP.name);
      activeMCP = null; mcpContent = ''; mcpError = '';
      await loadMCP();
    } catch (e) { mcpError = String(e); }
  }

  async function openPersonality() {
    leftTab = 'personality';
    await loadPersona();
  }

  async function loadPersona() {
    personaSaved = false;
    if (!currentAgent) { persona = { instructions: '', tone: '', verbosity: '', temperature: null }; personaKey++; return; }
    try {
      const p = await window.go.desktop.App.GetPersona(currentAgent.name);
      persona = {
        instructions: p.instructions || '',
        tone: p.tone || '',
        verbosity: p.verbosity || '',
        temperature: (p.temperature === undefined ? null : p.temperature),
      };
    } catch (e) {
      persona = { instructions: '', tone: '', verbosity: '', temperature: null };
    }
    personaKey++;
  }

  async function savePersona(e) {
    if (!currentAgent) { errorBanner = 'Select an agent first.'; return; }
    try {
      await window.go.desktop.App.SavePersona(currentAgent.name, e.detail);
      persona = e.detail;
      personaSaved = true;
      toast('Personality saved — New Chat to apply', 'info');
      await loadContextPreview();
      setTimeout(() => personaSaved = false, 2000);
    } catch (err) { errorBanner = 'Personality: ' + String(err); }
  }

  function toast(message, type = 'info') {
    const id = Date.now() + Math.random();
    toasts = [...toasts, { id, message, type }];
    setTimeout(() => { toasts = toasts.filter(t => t.id !== id); }, 4000);
  }

  function pushActivity(kind, label, detail = null, ts = null) {
    activitySeq++;
    activityEvents = [...activityEvents, {
      id: activitySeq,
      kind, label, detail,
      ts: ts || Date.now(),
    }].slice(-80);
  }

  async function refreshHomeData() {
    try {
      healthReport = await window.go.desktop.App.GetHealth();
    } catch (e) {
      healthReport = { ok: false, checks: [{ id: 'health', label: 'Health', status: 'error', detail: String(e) }] };
    }
    if (!toolsLoaded) { toolsLoaded = true; await loadTools(); }
    if (!skillsLoaded) { skillsLoaded = true; await loadSkills(); }
    if (!mcpLoaded) { mcpLoaded = true; await loadMCP(); }
    try {
      const km = await window.go.desktop.App.KMHealth();
      homeStats = {
        tools: toolsList.filter(t => !t.error).length,
        skills: skillsList.filter(s => !s.error).length,
        mcp: mcpList.filter(m => !m.error).length,
        km: !!km.available,
      };
    } catch (_) {
      homeStats = { tools: toolsList.length, skills: skillsList.length, mcp: mcpList.length, km: false };
    }
    try {
      homeSessions = await window.go.desktop.App.GetSessions() || [];
    } catch (_) {
      homeSessions = [];
    }
  }

  function personaLabel() {
    const parts = [];
    if (persona.tone) parts.push(persona.tone);
    if (persona.verbosity) parts.push(persona.verbosity);
    if (persona.instructions?.trim()) parts.push('custom');
    return parts.length ? parts.join(' · ') : 'default';
  }

  function openHome() {
    leftTab = 'home';
    refreshHomeData();
  }

  function navTab(tab) {
    if (tab === 'ext-tools') { openExtensions('tools'); return; }
    if (tab === 'ext-skills') { openExtensions('skills'); return; }
    if (tab === 'ext-mcp') { openExtensions('mcp'); return; }
    leftTab = tab;
    if (tab === 'knowledge') ensureKnowledge();
    if (tab === 'personality') loadPersona();
    if (tab === 'agents' && currentAgent) loadContextPreview();
  }

  async function loadContextPreview() {
    if (!currentAgent) { contextPreview = null; return; }
    contextLoading = true;
    try {
      contextPreview = await window.go.desktop.App.PreviewContext(currentAgent.name);
    } catch (e) {
      contextPreview = null;
    } finally {
      contextLoading = false;
    }
  }

  $: paletteItems = [
    { label: 'Home — Command Center', icon: '⌂', action: 'home', hint: '' },
    { label: 'New chat', icon: '＋', action: 'newChat', hint: currentAgent?.name || '' },
    { label: 'Open chat', icon: '⬡', action: 'agents', hint: '' },
    { label: 'Knowledge graph', icon: '◆', action: 'knowledge', hint: '' },
    { label: 'Extensions — tools', icon: '🛠', action: 'ext-tools', hint: '' },
    { label: 'Extensions — skills', icon: '✦', action: 'ext-skills', hint: '' },
    { label: 'Extensions — MCP', icon: '🔌', action: 'ext-mcp', hint: '' },
    { label: 'Personality', icon: '🎭', action: 'personality', hint: currentAgent?.name || '' },
    { label: 'Settings', icon: '⚙', action: 'settings', hint: '' },
    { label: 'Refresh health', icon: '↻', action: 'refreshHealth', hint: '' },
    { label: 'Toggle context inspector', icon: '◧', action: 'toggleInspector', hint: '' },
  ];

  function onPaletteSelect(action) {
    if (action === 'home') openHome();
    else if (action === 'newChat') newChat();
    else if (action === 'settings') showSettings = true;
    else if (action === 'refreshHealth') refreshHomeData();
    else if (action === 'toggleInspector') { showInspector = !showInspector; if (showInspector) loadContextPreview(); }
    else navTab(action);
  }

  function onGlobalKey(e) {
    if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
      e.preventDefault();
      paletteOpen = true;
    }
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
        moeRoute = null; contextUsage = 0; pendingApproval = null; pendingContinue = null;
      }
      await loadPersona();
      await loadContextPreview();
    } catch (e) { errorBanner = String(e); }
  }

  async function newChat() {
    if (!currentAgent) return;
    try {
      if (currentSession) { try { await window.go.desktop.App.CloseSession(currentSession); } catch (_) {} }
      const session = await window.go.desktop.App.CreateSession(currentAgent.name);
      currentSession = session.id;
      messages = []; streamBuffer = '';
      activityEvents = [];
      cost = { inputTokens: 0, outputTokens: 0, cost: 0 };
      moeRoute = null; contextUsage = 0; pendingApproval = null; pendingContinue = null;
      await loadContextPreview();
      toast('New chat started with latest config', 'ok');
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
    pushActivity('user', displayMsg.substring(0, 80));
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
    try { await window.go.desktop.App.RespondToolApproval(requestId, approved); } catch (e) { errorBanner = String(e); }
    pushActivity('tool', tool, { ok: approved, approval: true });
    scrollToBottom();
  }

  async function respondContinue(keepGoing) {
    if (!pendingContinue) return;
    const { requestId } = pendingContinue;
    pendingContinue = null;
    if (!keepGoing) {
      messages = [...messages, { role: 'system', content: '■ stopped after tool-step limit' }];
    }
    try { await window.go.desktop.App.RespondToolApproval(requestId, keepGoing); } catch (e) { errorBanner = String(e); }
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
    <TopBar agent={currentAgent} {cost} {moeRoute} {contextUsage}
      on:settings={() => showSettings = true} on:persona={openPersonality} />

    <div class="body">
      <nav class="rail">
        <button class="rail-btn" class:active={leftTab === 'home'} on:click={openHome} title="Command Center">
          <span class="rail-ico">⌂</span><span class="rail-lbl">Home</span>
        </button>
        <button class="rail-btn" class:active={leftTab === 'agents'} on:click={() => { leftTab = 'agents'; loadContextPreview(); }} title="Chat">
          <span class="rail-ico">⬡</span><span class="rail-lbl">Chat</span>
        </button>
        <button class="rail-btn" class:active={leftTab === 'knowledge'} on:click={openKnowledge} title="Knowledge graph">
          <span class="rail-ico">◆</span><span class="rail-lbl">Knowledge</span>
        </button>
        <button class="rail-btn" class:active={leftTab === 'extensions'} on:click={() => openExtensions('tools')} title="Extensions">
          <span class="rail-ico">🧩</span><span class="rail-lbl">Extensions</span>
        </button>
        <button class="rail-btn" class:active={leftTab === 'personality'} on:click={openPersonality} title="Personality">
          <span class="rail-ico">🎭</span><span class="rail-lbl">Persona</span>
        </button>
        <div class="rail-spacer"></div>
        <button class="rail-btn" on:click={() => showSettings = true} title="Settings">
          <span class="rail-ico">⚙</span><span class="rail-lbl">Settings</span>
        </button>
      </nav>

      {#if leftTab !== 'home' && leftTab !== 'knowledge'}
      <div class="sidecol">
        {#if leftTab === 'agents'}
          <Sidebar {agents} active={currentAgent} on:select={e => selectAgent(e.detail)} on:settings={() => showSettings = true} />
        {:else if leftTab === 'extensions'}
          <ExtensionsNav active={extensionsSubTab} on:select={e => openExtensions(e.detail)} />
          {#if extensionsSubTab === 'tools'}
            <ToolsSidebar tools={toolsList} activeName={activeTool?.name}
              on:select={e => selectTool(e.detail)} on:new={newTool} on:settings={() => showSettings = true} />
          {:else if extensionsSubTab === 'skills'}
            <SkillsSidebar skills={skillsList} activeName={activeSkill?.name}
              on:select={e => selectSkill(e.detail)} on:new={newSkill} on:settings={() => showSettings = true} />
          {:else if extensionsSubTab === 'mcp'}
            <MCPSidebar servers={mcpList} activeName={activeMCP?.name}
              on:select={e => selectMCP(e.detail)} on:new={newMCP} on:settings={() => showSettings = true} />
          {/if}
        {:else if leftTab === 'personality'}
          <Sidebar {agents} active={currentAgent} on:select={e => selectAgent(e.detail)} on:settings={() => showSettings = true} />
        {/if}
      </div>
      {/if}

      <div class="main">
        {#if errorBanner}
          <div class="err-banner">⚠ {errorBanner} <button on:click={() => errorBanner = ''}>✕</button></div>
        {/if}

        {#if leftTab === 'home'}
          <CommandCenter agent={currentAgent} health={healthReport} stats={homeStats} {moeRoute} sessions={homeSessions}
            on:nav={e => navTab(e.detail)}
            on:newChat={newChat}
            on:refresh={refreshHomeData} />
        {:else if leftTab === 'extensions' && extensionsSubTab === 'tools'}
          <div class="tools-main">
            {#if activeTool}
              {#if !activeTool.isNew}
                <div class="tool-bar slim">
                  <button class="tbtn delete" on:click={deleteTool}>Delete tool</button>
                </div>
              {/if}
              <ToolFormEditor bind:content={toolContent} bind:advanced={toolAdvanced}
                saving={toolSaving} saved={toolSaved} error={toolError}
                on:change={e => { toolContent = e.detail; toolSaved = false; }}
                on:save={saveTool} />
              <TestBench mode="tool" name={toolNameFromContent(toolContent)} disabled={!!toolError || activeTool.isNew} />
            {:else}
              <div class="tools-empty">
                <div class="tools-empty-card">
                  <h2>Custom tools</h2>
                  <p>Define a tool in an MD file — a script, CLI, or HTTP shim the agent can call.
                  Pick one on the left, or create a new tool.</p>
                  <button on:click={newTool}>＋ New tool</button>
                </div>
              </div>
            {/if}
          </div>
        {:else if leftTab === 'extensions' && extensionsSubTab === 'skills'}
          <div class="tools-main">
            {#if activeSkill}
              {#if !activeSkill.isNew}
                <div class="tool-bar slim">
                  <button class="tbtn delete" on:click={deleteSkill}>Delete skill</button>
                </div>
              {/if}
              <SkillFormEditor bind:content={skillContent} bind:advanced={skillAdvanced}
                saving={skillSaving} saved={skillSaved} error={skillError}
                on:change={e => { skillContent = e.detail; skillSaved = false; }}
                on:save={saveSkill} />
            {:else}
              <div class="tools-empty">
                <div class="tools-empty-card">
                  <h2>Skills</h2>
                  <p>Capture reusable instructions, conventions, or domain knowledge in an MD file.
                  Every skill's body is added to the agent's system prompt, so it follows them on every task.
                  Pick one on the left, or create a new skill.</p>
                  <button on:click={newSkill}>＋ New skill</button>
                </div>
              </div>
            {/if}
          </div>
        {:else if leftTab === 'extensions' && extensionsSubTab === 'mcp'}
          <div class="tools-main">
            {#if activeMCP}
              {#if !activeMCP.isNew}
                <div class="tool-bar slim">
                  <button class="tbtn delete" on:click={deleteMCP}>Delete server</button>
                </div>
              {/if}
              <MCPFormEditor bind:content={mcpContent} bind:advanced={mcpAdvanced}
                saving={mcpSaving} saved={mcpSaved} error={mcpError}
                on:change={e => { mcpContent = e.detail; mcpSaved = false; }}
                on:save={saveMCP} />
              <TestBench mode="mcp" name={mcpNameFromContent(mcpContent)} disabled={!!mcpError || activeMCP.isNew} />
            {:else}
              <div class="tools-empty">
                <div class="tools-empty-card">
                  <h2>MCP servers</h2>
                  <p>Connect external tool servers via the Model Context Protocol. Define one in an MD file —
                  a local stdio command or a remote http/sse endpoint — and its tools become available to the agent.
                  Pick one on the left, or add a new server.</p>
                  <button on:click={newMCP}>＋ New MCP server</button>
                </div>
              </div>
            {/if}
          </div>
        {:else if leftTab === 'personality'}
          {#if currentAgent}
            {#key personaKey}
              <PersonaPanel agentName={currentAgent.name} {persona} saved={personaSaved}
                on:save={savePersona} />
            {/key}
          {:else}
            <div class="tools-main">
              <div class="tools-empty">
                <div class="tools-empty-card">
                  <h2>Personality</h2>
                  <p>Select an agent on the left to customize its tone, verbosity, creativity, and custom instructions.</p>
                </div>
              </div>
            </div>
          {/if}
        {:else if leftTab === 'knowledge'}
          <div class="km-split">
            {#if kmHealth.available}
              <div class="km-graph-pane">
                <div class="km-graphbar">
                  <span class="km-title">◆ Knowledge graph</span>
                  <span class="km-sub">{kmNodes.length} nodes · {kmLinks.length} links</span>
                  <button class="km-fit" on:click={() => graphRef?.fit()}>⊡ Fit</button>
                </div>
                <div class="km-canvas">
                  <KnowledgeGraph bind:this={graphRef} nodes={kmNodes} links={kmLinks}
                    visibleTypes={kmVisibleTypes} typeColors={KM_COLORS} />
                </div>
              </div>
              <KnowledgePanel health={kmHealth} stats={kmStats} indexing={kmIndexing}
                visibleTypes={kmVisibleTypes} nodeCounts={kmNodeCounts} typeColors={KM_COLORS}
                nodes={kmNodes} selectedId={kmSelectedId}
                on:index={kmIndexFolder} on:refresh={refreshKnowledge}
                on:toggle={e => toggleKMType(e.detail)}
                on:select={e => kmSelectedId = e.detail} />
            {:else}
              <div class="km-empty">
                <div class="km-empty-card">
                  <h2>Knowledge graph unavailable</h2>
                  <p>knowledge-master isn't reachable at <code>127.0.0.1:9999</code>. Start the local backend:</p>
                  <pre>km start
km serve</pre>
                  {#if kmHealth.error}<div class="km-err">{kmHealth.error}</div>{/if}
                  <button on:click={refreshKnowledge}>↻ Retry</button>
                </div>
              </div>
            {/if}
          </div>
        {:else if !currentSession}
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
          <div class="chat-layout">
            <div class="chat-col">
              <div class="chat-toolbar">
                <span class="chat-title">◆ {currentAgent?.name}</span>
                {#if moeRoute}<span class="moe-pill">{moeRoute.category}</span>{/if}
                <button class="tb-btn persona-pill" on:click={openPersonality} title="Edit personality">
                  🎭 {personaLabel()}
                </button>
                <button class="tb-btn" class:on={showInspector} on:click={() => { showInspector = !showInspector; if (showInspector) loadContextPreview(); }} title="Context inspector">◧ Context</button>
                <button class="tb-btn" on:click={() => paletteOpen = true} title="Command palette (⌘K)">⌘K</button>
              </div>
              <ActivityTimeline events={activityEvents} />

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
              <span class="hint-txt">Enter to send · ⌘K palette · ◧ context</span>
            </div>
          </div>
            </div>
            {#if showInspector}
              <ContextInspector preview={contextPreview} loading={contextLoading}
                on:close={() => showInspector = false}
                on:apply={newChat} />
            {/if}
          </div>
        {/if}
      </div>
    </div>

    <CommandPalette bind:open={paletteOpen} items={paletteItems} on:select={e => onPaletteSelect(e.detail)} />
    <ToastStack {toasts} />
    {#if showOnboarding}
      <Onboarding on:done={() => showOnboarding = false} />
    {/if}

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

  .chat-layout{flex:1;display:flex;min-height:0;overflow:hidden}
  .chat-col{flex:1;display:flex;flex-direction:column;min-width:0;min-height:0;overflow:hidden}
  .chat-toolbar{display:flex;align-items:center;gap:8px;padding:8px 14px;border-bottom:1px solid var(--border);flex-shrink:0;background:#080d18}
  .chat-title{font-size:13px;font-weight:600;color:var(--text);flex:1}
  .moe-pill{font-size:10px;font-weight:700;padding:2px 8px;border-radius:4px;background:#4c1d95;color:#c4b5fd}
  .persona-pill{font-size:10px;max-width:140px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .tb-btn{padding:5px 10px;border-radius:6px;border:1px solid var(--border);background:var(--bg-input);color:var(--muted);font-size:11px;font-weight:600;cursor:pointer}
  .tb-btn:hover{color:var(--text)}
  .tb-btn.on{border-color:var(--accent);color:#93c5fd}

  .rail{width:62px;flex-shrink:0;display:flex;flex-direction:column;align-items:stretch;gap:2px;padding:8px 6px;background:#080d18;border-right:1px solid #1e293b;overflow:hidden}
  .rail-spacer{flex:1}
  .rail-btn{display:flex;flex-direction:column;align-items:center;gap:3px;padding:8px 2px;background:none;border:none;border-radius:8px;color:#64748b;cursor:pointer;position:relative}
  .rail-btn:hover{background:#111c30;color:#cbd5e1}
  .rail-btn.active{background:#1e293b;color:#e2e8f0}
  .rail-btn.active::before{content:'';position:absolute;left:-6px;top:8px;bottom:8px;width:3px;border-radius:0 3px 3px 0;background:#6366f1}
  .rail-ico{font-size:17px;line-height:1}
  .rail-lbl{font-size:9px;font-weight:600;letter-spacing:-0.1px}

  .sidecol{width:240px;flex-shrink:0;display:flex;flex-direction:column;background:#0c1322;border-right:1px solid #1e293b;overflow:hidden}

  .km-main,.km-split{flex:1;display:flex;min-height:0;overflow:hidden}
  .km-split{flex-direction:row}
  .km-graph-pane{flex:1;display:flex;flex-direction:column;min-width:0;min-height:0}
  .km-graphbar{display:flex;align-items:center;gap:12px;padding:8px 16px;border-bottom:1px solid var(--border);flex-shrink:0}
  .km-title{font-size:13px;font-weight:600;color:var(--text)}
  .km-sub{font-size:12px;color:var(--muted);flex:1}
  .km-fit{background:var(--bg-input);border:1px solid var(--border);border-radius:6px;color:var(--text);font-size:12px;padding:5px 12px;cursor:pointer}
  .km-fit:hover{background:var(--border)}
  .km-canvas{flex:1;min-height:0;position:relative}

  .tools-main{flex:1;display:flex;flex-direction:column;min-height:0;overflow:hidden}
  .tool-bar{display:flex;align-items:center;justify-content:space-between;padding:10px 16px;border-bottom:1px solid var(--border);flex-shrink:0}
  .tool-bar.slim{justify-content:flex-end;padding:8px 16px;background:#080d18}
  .tool-title{font-size:14px;font-weight:600;color:var(--text);font-family:'SF Mono',Menlo,monospace}
  .tool-name-field{display:flex;align-items:center;gap:8px;min-width:0;flex:1}
  .tool-name-lbl{font-size:11px;font-weight:700;color:var(--muted);text-transform:uppercase;letter-spacing:0.4px;flex-shrink:0}
  .tool-name-input{flex:1;min-width:0;max-width:280px;padding:6px 10px;background:#0c1322;border:1px solid var(--border);border-radius:6px;color:#e2e8f0;font-family:'SF Mono',Menlo,monospace;font-size:13px;outline:none}
  .tool-name-input:focus{border-color:var(--accent)}
  .tool-actions{display:flex;gap:8px;flex-shrink:0}
  .tbtn{padding:7px 16px;border-radius:7px;font-size:13px;font-weight:600;cursor:pointer;border:1px solid var(--border)}
  .tbtn.save{background:var(--accent);color:#fff;border-color:var(--accent)}
  .tbtn.save:hover:not(:disabled){filter:brightness(1.1)}
  .tbtn.save:disabled{opacity:0.6;cursor:not-allowed}
  .tbtn.delete{background:var(--bg-panel);color:var(--err);border-color:color-mix(in srgb,var(--err) 40%,transparent)}
  .tbtn.delete:hover{background:color-mix(in srgb,var(--err) 15%,transparent)}
  .tool-err{margin:10px 16px 0;padding:8px 12px;background:color-mix(in srgb,var(--err) 14%,transparent);border:1px solid color-mix(in srgb,var(--err) 35%,transparent);border-radius:7px;color:#fca5a5;font-size:12px;word-break:break-word}
  .tool-edit{flex:1;min-height:0;margin:12px 16px;padding:14px;background:#0c1322;border:1px solid var(--border);border-radius:8px;color:#e2e8f0;font-family:'SF Mono',Menlo,monospace;font-size:13px;line-height:1.55;resize:none;outline:none;tab-size:2}
  .tool-edit:focus{border-color:var(--accent)}
  .tool-hint{padding:0 16px 14px;font-size:12px;color:var(--muted);line-height:1.5}
  .tool-hint code{background:var(--bg-panel);padding:1px 5px;border-radius:4px;font-size:11px}
  .tools-empty{flex:1;display:flex;align-items:center;justify-content:center;padding:24px}
  .tools-empty-card{max-width:420px;text-align:center;background:var(--bg-panel);border:1px solid var(--border);border-radius:12px;padding:28px}
  .tools-empty-card h2{font-size:18px;color:var(--text);margin-bottom:8px}
  .tools-empty-card p{font-size:13px;color:var(--muted);line-height:1.6;margin-bottom:16px}
  .tools-empty-card button{background:var(--accent);color:#fff;border:none;border-radius:8px;padding:9px 22px;font-weight:600;font-size:13px;cursor:pointer}
  .tools-empty-card button:hover{filter:brightness(1.1)}

  .km-empty{flex:1;display:flex;align-items:center;justify-content:center;padding:24px}
  .km-empty-card{max-width:420px;text-align:center;background:var(--bg-panel);border:1px solid var(--border);border-radius:12px;padding:28px}
  .km-empty-card h2{font-size:18px;color:var(--text);margin-bottom:8px}
  .km-empty-card p{font-size:13px;color:var(--muted);line-height:1.6}
  .km-empty-card code{background:var(--bg);padding:1px 6px;border-radius:4px;font-size:12px}
  .km-empty-card pre{text-align:left;background:var(--bg);border:1px solid var(--border);border-radius:8px;padding:12px 14px;margin:14px 0;font-size:13px;color:#cbd5e1;font-family:'SF Mono',Menlo,monospace}
  .km-empty-card .km-err{font-size:12px;color:#fca5a5;margin-bottom:12px;word-break:break-word}
  .km-empty-card button{background:var(--accent);color:#fff;border:none;border-radius:8px;padding:9px 22px;font-weight:600;font-size:13px;cursor:pointer}
  .km-empty-card button:hover{filter:brightness(1.1)}

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
  .approval.continue{background:color-mix(in srgb,var(--accent) 12%,transparent);border-color:color-mix(in srgb,var(--accent) 40%,transparent)}
  .approval.continue .approval-tool{color:var(--accent)}
  .continue-sub{font-size:13px;color:var(--text)}
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
