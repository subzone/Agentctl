<script>
  import { onMount, onDestroy } from 'svelte';
  import { forceSimulation, forceManyBody, forceLink, forceCenter, forceCollide, forceX, forceY } from 'd3-force';
  import { select } from 'd3-selection';
  import { zoom as d3zoom, zoomIdentity } from 'd3-zoom';

  export let nodes = [];
  export let links = [];
  export let visibleTypes = null; // Set of type names, or null = all
  export let typeColors = {
    Repo: '#818cf8', Document: '#38bdf8', Person: '#fbbf24', Tech: '#34d399',
    Service: '#f472b6', Convention: '#c084fc', Chunk: '#64748b', default: '#94a3b8',
  };

  let canvas, wrap;
  let width = 800, height = 600;
  let sim;
  let tf = zoomIdentity;
  let hover = null;
  let simNodes = [], simLinks = [];
  let ro;

  function colorFor(t) { return typeColors[t] || typeColors.default; }
  function radiusFor(t) {
    if (t === 'Repo') return 9;
    if (t === 'Person' || t === 'Service') return 6;
    if (t === 'Chunk') return 2.5;
    return 4.5;
  }

  // Rebuild the simulation whenever the data or filter changes.
  $: rebuild(nodes, links, visibleTypes);

  function rebuild(ns, ls, vis) {
    if (!canvas) return;
    const allow = (t) => !vis || vis.has(t);
    const byId = new Map();
    simNodes = [];
    for (const n of ns) {
      if (!allow(n.type)) continue;
      const node = { id: n.id, type: n.type, label: n.label, category: n.category };
      byId.set(n.id, node);
      simNodes.push(node);
    }
    simLinks = [];
    for (const l of ls) {
      if (byId.has(l.source) && byId.has(l.target)) {
        simLinks.push({ source: l.source, target: l.target, type: l.type });
      }
    }
    if (sim) sim.stop();
    sim = forceSimulation(simNodes)
      .force('charge', forceManyBody().strength(-40))
      .force('link', forceLink(simLinks).id(d => d.id).distance(40).strength(0.4))
      .force('center', forceCenter(width / 2, height / 2))
      .force('x', forceX(width / 2).strength(0.03))
      .force('y', forceY(height / 2).strength(0.03))
      .force('collide', forceCollide().radius(d => radiusFor(d.type) + 2))
      .alpha(0.9).alphaDecay(0.025)
      .on('tick', draw);
    draw();
  }

  function draw() {
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    const dpr = window.devicePixelRatio || 1;
    ctx.save();
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    ctx.scale(dpr, dpr);
    ctx.translate(tf.x, tf.y);
    ctx.scale(tf.k, tf.k);

    // Links
    ctx.lineWidth = 0.6 / Math.max(tf.k, 0.4);
    ctx.strokeStyle = 'rgba(148,163,184,0.22)';
    ctx.beginPath();
    for (const l of simLinks) {
      if (!l.source.x || !l.target.x) continue;
      ctx.moveTo(l.source.x, l.source.y);
      ctx.lineTo(l.target.x, l.target.y);
    }
    ctx.stroke();

    // Nodes
    for (const n of simNodes) {
      const r = radiusFor(n.type);
      ctx.beginPath();
      ctx.fillStyle = colorFor(n.type);
      ctx.arc(n.x, n.y, r, 0, Math.PI * 2);
      ctx.fill();
      if (hover && hover.id === n.id) {
        ctx.lineWidth = 2 / tf.k;
        ctx.strokeStyle = '#fff';
        ctx.stroke();
      }
    }
    // Labels for larger nodes when zoomed in enough.
    if (tf.k > 0.7) {
      ctx.fillStyle = 'rgba(226,232,240,0.85)';
      ctx.font = `${11 / tf.k}px -apple-system, sans-serif`;
      for (const n of simNodes) {
        if (n.type === 'Chunk') continue;
        if (n.type !== 'Repo' && tf.k < 1.4) continue;
        if (n.label) ctx.fillText(n.label, n.x + radiusFor(n.type) + 2, n.y + 3);
      }
    }
    ctx.restore();
  }

  function screenToGraph(sx, sy) {
    return { x: (sx - tf.x) / tf.k, y: (sy - tf.y) / tf.k };
  }

  function onMove(e) {
    const rect = canvas.getBoundingClientRect();
    const p = screenToGraph(e.clientX - rect.left, e.clientY - rect.top);
    let best = null, bestD = 12 / tf.k;
    for (const n of simNodes) {
      const dx = n.x - p.x, dy = n.y - p.y;
      const d = Math.hypot(dx, dy);
      if (d < bestD) { bestD = d; best = n; }
    }
    hover = best;
    draw();
  }

  function resize() {
    if (!wrap || !canvas) return;
    width = wrap.clientWidth;
    height = wrap.clientHeight;
    const dpr = window.devicePixelRatio || 1;
    canvas.width = width * dpr;
    canvas.height = height * dpr;
    canvas.style.width = width + 'px';
    canvas.style.height = height + 'px';
    if (sim) {
      sim.force('center', forceCenter(width / 2, height / 2));
      sim.alpha(0.3).restart();
    }
    draw();
  }

  export function fit() {
    if (!simNodes.length) return;
    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    for (const n of simNodes) {
      minX = Math.min(minX, n.x); maxX = Math.max(maxX, n.x);
      minY = Math.min(minY, n.y); maxY = Math.max(maxY, n.y);
    }
    const gw = maxX - minX || 1, gh = maxY - minY || 1;
    const k = Math.min(width / (gw + 80), height / (gh + 80), 2);
    const tx = width / 2 - k * (minX + gw / 2);
    const ty = height / 2 - k * (minY + gh / 2);
    tf = zoomIdentity.translate(tx, ty).scale(k);
    select(canvas).call(zoomBehavior.transform, tf);
    draw();
  }

  let zoomBehavior;
  onMount(() => {
    resize();
    zoomBehavior = d3zoom().scaleExtent([0.1, 6]).on('zoom', (e) => { tf = e.transform; draw(); });
    select(canvas).call(zoomBehavior);
    ro = new ResizeObserver(resize);
    ro.observe(wrap);
    rebuild(nodes, links, visibleTypes);
    // Auto-fit once the layout settles a little.
    setTimeout(fit, 600);
  });

  onDestroy(() => {
    if (sim) sim.stop();
    if (ro) ro.disconnect();
  });
</script>

<div class="graph-wrap" bind:this={wrap}>
  <canvas bind:this={canvas} on:mousemove={onMove} on:mouseleave={() => { hover = null; draw(); }}></canvas>
  {#if hover}
    <div class="tip">
      <span class="dot" style="background:{colorFor(hover.type)}"></span>
      <span class="tip-type">{hover.type}</span>
      <span class="tip-label">{hover.label}</span>
    </div>
  {/if}
</div>

<style>
  .graph-wrap{position:relative;width:100%;height:100%;overflow:hidden;background:radial-gradient(circle at 50% 40%, #0f1a2e 0%, #0a0f1c 100%)}
  canvas{display:block;cursor:grab}
  canvas:active{cursor:grabbing}
  .tip{position:absolute;top:12px;left:12px;display:flex;align-items:center;gap:8px;background:rgba(15,23,42,0.92);border:1px solid #334155;border-radius:8px;padding:6px 12px;font-size:12px;color:#e2e8f0;pointer-events:none;max-width:80%}
  .dot{width:9px;height:9px;border-radius:50%;flex-shrink:0}
  .tip-type{color:#94a3b8;text-transform:uppercase;font-size:10px;letter-spacing:0.4px}
  .tip-label{font-weight:500;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
</style>
