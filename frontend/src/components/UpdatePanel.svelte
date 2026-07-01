<script>
  import { createEventDispatcher, onMount, onDestroy } from 'svelte';
  const dispatch = createEventDispatcher();

  export let updateInfo = null;
  export let compact = false;

  let downloading = false;
  let progress = 0;
  let result = null;
  let error = '';

  function onProgress(data) {
    if (data?.phase === 'downloading') {
      downloading = true;
      progress = data.percent || 0;
    } else if (data?.phase === 'done') {
      downloading = false;
      progress = 100;
      result = { path: data.path, filename: data.filename };
    }
  }

  onMount(() => {
    window.runtime?.EventsOn('updateProgress', onProgress);
  });

  onDestroy(() => {
    window.runtime?.EventsOff('updateProgress');
  });

  async function download() {
    error = '';
    result = null;
    downloading = true;
    progress = 0;
    try {
      const res = await window.go.desktop.App.DownloadUpdate();
      result = res;
      downloading = false;
      progress = 100;
    } catch (e) {
      error = String(e);
      downloading = false;
    }
  }

  async function reveal() {
    const path = result?.path;
    if (!path) return;
    try {
      await window.go.desktop.App.OpenPath(path);
    } catch (e) {
      error = String(e);
    }
  }
</script>

{#if updateInfo?.updateAvailable}
  <div class="panel" class:compact>
    <div class="head">
      <span class="title">↑ Update available</span>
      <span class="ver">v{updateInfo.current} → v{updateInfo.latest}</span>
    </div>
    {#if !compact && updateInfo.downloadNotes}
      <p class="notes">{updateInfo.downloadNotes}</p>
    {/if}
    {#if downloading}
      <div class="prog"><div class="fill" style="width:{progress}%"></div></div>
      <div class="prog-meta">Downloading… {Math.round(progress)}%</div>
    {/if}
    {#if error}<div class="err">{error}</div>{/if}
    {#if result}
      <div class="ok">Downloaded {result.filename}</div>
      {#if result.notes}<p class="notes">{result.notes}</p>{/if}
    {/if}
    <div class="actions">
      <button class="btn primary" disabled={downloading} on:click={download}>
        {downloading ? 'Downloading…' : 'Download update'}
      </button>
      {#if result}
        <button class="btn" on:click={reveal}>Show in Finder</button>
      {/if}
      <a class="btn link" href={updateInfo.releaseUrl} target="_blank" rel="noreferrer">Release notes</a>
    </div>
  </div>
{/if}

<style>
  .panel{background:#1a2332;border:1px solid #334155;border-radius:10px;padding:14px 16px;margin-bottom:16px}
  .panel.compact{padding:10px 12px;margin-bottom:0}
  .head{display:flex;align-items:center;justify-content:space-between;gap:8px;margin-bottom:8px}
  .title{font-size:13px;font-weight:700;color:#fcd34d}
  .ver{font-size:11px;color:#94a3b8;font-family:'SF Mono',Menlo,monospace}
  .notes{font-size:12px;color:#94a3b8;line-height:1.5;margin-bottom:10px}
  .prog{height:6px;background:#0f172a;border-radius:3px;overflow:hidden;margin-bottom:4px}
  .fill{height:100%;background:#6366f1;transition:width 0.2s}
  .prog-meta{font-size:11px;color:#64748b;margin-bottom:8px}
  .err{font-size:12px;color:#fca5a5;margin-bottom:8px}
  .ok{font-size:12px;color:#86efac;margin-bottom:8px}
  .actions{display:flex;flex-wrap:wrap;gap:8px}
  .btn{padding:7px 14px;border-radius:6px;border:1px solid #334155;background:#0f172a;color:#e2e8f0;font-size:12px;font-weight:600;cursor:pointer;text-decoration:none}
  .btn.primary{background:#6366f1;border-color:#6366f1;color:#fff}
  .btn:disabled{opacity:0.5;cursor:not-allowed}
  .btn.link{display:inline-flex;align-items:center}
</style>
