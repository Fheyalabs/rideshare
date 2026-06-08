package dashboard

const PageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Rideshare — Live Auction</title>
<script src="https://unpkg.com/maplibre-gl@4/dist/maplibre-gl.js"></script>
<link href="https://unpkg.com/maplibre-gl@4/dist/maplibre-gl.css" rel="stylesheet">
<style>
:root {
  --yellow: #FEE900; --black: #000; --offwhite: #FAFAF7; --charcoal: #121212;
  --graphite: #232323; --border: #D9D9D9; --green: #22C55E; --amber: #FFB800; --red: #EF4444;
}
* { margin:0; padding:0; box-sizing:border-box; }
body { font-family: -apple-system, system-ui, sans-serif; background: var(--charcoal); color: var(--offwhite); height: 100vh; display: flex; flex-direction: column; }
header { background: var(--black); border-bottom: 3px solid var(--yellow); padding: 12px 20px; display: flex; justify-content: space-between; align-items: center; }
header h1 { font-size: 1.2em; color: var(--yellow); }
header .loop { font-size: 0.8em; color: #888; }
#main { flex:1; display: flex; overflow: hidden; }
#left { width: 320px; background: var(--graphite); overflow-y: auto; padding: 12px; border-right: 1px solid var(--border); }
#map { flex:1; }
.party { margin-bottom: 12px; padding: 8px; border-radius: 6px; background: var(--charcoal); border: 1px solid var(--border); }
.party .name { font-weight: bold; font-size: 0.9em; }
.party .phase { font-size: 0.75em; margin-top: 4px; padding: 2px 6px; border-radius: 4px; display: inline-block; }
.phase.IDLE, .phase.COMPLETE { background: #444; color: #999; }
.phase.KEYGEN, .phase.DISCOVERING, .phase.DECIDING { background: #FE0; color: #000; }
.phase.AUCTIONING, .phase.BIDDING { background: #FFB800; color: #000; }
.phase.REVIEW, .phase.AWAITING { background: #FFB800; color: #000; }
.phase.ACCEPTED, .phase.WON { background: var(--green); color: #000; }
.phase.IN_RIDE { background: #3B82F6; color: #fff; }
#wire { margin-top: 16px; }
#wire h3 { font-size: 0.85em; color: var(--yellow); margin-bottom: 8px; }
#wire .entry { font-family: monospace; font-size: 0.7em; padding: 4px; margin: 2px 0; background: #1a1a1a; border-radius: 3px; word-break: break-all; }
#wire .entry .label { color: var(--yellow); }
#log { margin-top: 12px; }
#log h3 { font-size: 0.85em; color: #888; margin-bottom: 4px; }
#log .line { font-size: 0.7em; color: #666; }
.marker { width: 12px; height: 12px; border-radius: 50%; border: 2px solid #fff; }
.marker.rider { background: var(--yellow); }
.marker.driver { background: #3B82F6; }
.marker.winner { background: var(--green); width: 16px; height: 16px; }
</style>
</head>
<body>
<header>
  <h1>&#x2605; Rideshare · Live Auction</h1>
  <span class="loop" id="loop-counter">Loop —</span>
</header>
<div id="main">
  <div id="left">
    <div id="parties"></div>
    <div id="wire"><h3>&#x1F4E6; Wire (ciphertext only)</h3></div>
    <div id="log"><h3>Log</h3></div>
  </div>
  <div id="map"></div>
</div>
<script>
const parties = {};
const wireEl = document.getElementById('wire');
const logEl = document.getElementById('log');
const partiesEl = document.getElementById('parties');
const loopEl = document.getElementById('loop-counter');

// MapLibre (OSM tiles)
const map = new maplibregl.Map({
  container: 'map', style: {
    version: 8,
    sources: { 'osm-tiles': { type: 'raster', tiles: ['https://tile.openstreetmap.org/{z}/{x}/{y}.png'], tileSize: 256, attribution: '&copy; OSM' } },
    layers: [{ id: 'osm', type: 'raster', source: 'osm-tiles' }]
  },
  center: [13.7384, 51.0493], zoom: 13
});
map.addControl(new maplibregl.NavigationControl());

const evtSource = new EventSource('/dashboard/events');
evtSource.onmessage = function(e) {
  const ev = JSON.parse(e.data);
  if (ev.type === 'connected') return;

  if (ev.type === 'loop') {
    loopEl.textContent = ev.detail;
    wireEl.innerHTML = '<h3>&#x1F4E6; Wire (ciphertext only)</h3>';
    logEl.innerHTML = '<h3>Log</h3>';
    // Keep existing party phase state but reset markers
  }

  if (ev.type === 'phase') {
    if (!parties[ev.party]) { parties[ev.party] = { phase: ev.phase }; }
    else { parties[ev.party].phase = ev.phase; }
    if (ev.detail) {
      const line = document.createElement('div');
      line.className = 'line'; line.textContent = ev.detail;
      logEl.appendChild(line);
      if (logEl.children.length > 20) logEl.removeChild(logEl.firstChild);
    }
    renderParties();
  }

  if (ev.type === 'wire') {
    const entry = document.createElement('div');
    entry.className = 'entry';
    entry.innerHTML = '<span class="label">' + ev.party + '</span> ' + (ev.payload || ev.detail || '');
    wireEl.appendChild(entry);
    if (wireEl.children.length > 15 && wireEl.children[2]) wireEl.removeChild(wireEl.children[2]);
  }

  if (ev.type === 'marker') {
    if (ev.party === 'rider') {
      new maplibregl.Marker({ color: '#FEE900' }).setLngLat([ev.lng||13.7384, ev.lat||51.0493]).addTo(map);
    } else {
      new maplibregl.Marker({ color: ev.phase==='WON'?'#22C55E':'#3B82F6' }).setLngLat([ev.lng||13.7384, ev.lat||51.0493]).addTo(map);
    }
  }
};

function renderParties() {
  const order = ['rider', 'drv-1','drv-2','drv-3','drv-4','drv-5'];
  let html = '';
  for (const name of order) {
    const p = parties[name];
    if (!p) continue;
    html += '<div class="party"><span class="name">' + name + '</span> ' +
      '<span class="phase ' + p.phase + '">' + p.phase + '</span></div>';
  }
  for (const name in parties) {
    if (order.includes(name)) continue;
    const p = parties[name];
    html += '<div class="party"><span class="name">' + name + '</span> ' +
      '<span class="phase ' + p.phase + '">' + p.phase + '</span></div>';
  }
  partiesEl.innerHTML = html;
}
</script>
</body>
</html>`
