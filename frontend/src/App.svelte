<script>
  import { onMount } from 'svelte';
  import Chat from './components/Chat.svelte';
  import Sidebar from './components/Sidebar.svelte';
  import Settings from './components/Settings.svelte';
  import TopBar from './components/TopBar.svelte';
  
  let currentView = 'chat';
  let currentSession = null;
  let currentAgent = null;
  let agents = [];
  let sessions = [];
  let cost = { inputTokens: 0, outputTokens: 0, cost: 0, model: '' };
  
  onMount(async () => {
    try {
      const { ListAgents } = await import('./wailsjs/go/desktop/App');
      agents = await ListAgents();
    } catch (err) {
      console.error('Failed to load agents:', err);
    }
  });
  
  async function handleNewChat(agent) {
    try {
      const { CreateSession } = await import('./wailsjs/go/desktop/App');
      const session = await CreateSession(agent.name);
      currentSession = session.id;
      currentAgent = agent;
      currentView = 'chat';
      await refreshSessions();
    } catch (err) {
      console.error('Failed to create session:', err);
    }
  }
  
  async function handleSwitchAgent(agent) {
    if (!currentSession) {
      handleNewChat(agent);
      return;
    }
    try {
      const { SwitchAgent } = await import('./wailsjs/go/desktop/App');
      await SwitchAgent(currentSession, agent.name);
      currentAgent = agent;
    } catch (err) {
      console.error('Failed to switch agent:', err);
    }
  }
  
  async function handleResumeSession(session) {
    currentSession = session.id;
    currentAgent = agents.find(a => a.name === session.agent) || { name: session.agent, model: session.model };
    currentView = 'chat';
  }
  
  async function refreshSessions() {
    try {
      const { GetSessions } = await import('./wailsjs/go/desktop/App');
      sessions = await GetSessions();
    } catch (err) {}
  }
  
  function handleCostUpdate(e) {
    cost = e.detail;
  }
</script>

<main class="app">
  <TopBar 
    agent={currentAgent}
    {cost}
    on:switchAgent={e => handleSwitchAgent(e.detail)}
    on:settings={() => currentView = 'settings'}
  />
  
  <div class="body">
    <Sidebar 
      {agents}
      {sessions}
      on:newChat={e => handleNewChat(e.detail)}
      on:resumeSession={e => handleResumeSession(e.detail)}
      on:settings={() => currentView = 'settings'}
    />
    
    <div class="content">
      {#if currentView === 'chat'}
        <Chat 
          sessionId={currentSession} 
          on:costUpdate={handleCostUpdate}
        />
      {:else if currentView === 'settings'}
        <Settings on:close={() => currentView = 'chat'} />
      {/if}
    </div>
  </div>
</main>

<style>
  :global(body) {
    margin: 0;
    padding: 0;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    background-color: #0f172a;
    color: #e2e8f0;
  }
  
  .app {
    display: flex;
    flex-direction: column;
    height: 100vh;
    width: 100vw;
    overflow: hidden;
  }
  
  .body {
    display: flex;
    flex: 1;
    overflow: hidden;
  }
  
  .content {
    flex: 1;
    overflow: hidden;
  }
</style>
