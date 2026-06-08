package dashboard

const PageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Rideshare — Live Encrypted Auction</title>
<script src="https://unpkg.com/maplibre-gl@4/dist/maplibre-gl.js"></script>
<link href="https://unpkg.com/maplibre-gl@4/dist/maplibre-gl.css" rel="stylesheet">
<style>
:root {
  --yellow: #FEE900; --black: #000; --offwhite: #FAFAF7; --charcoal: #121212;
  --graphite: #232323; --border: #3A3A3A; --green: #22C55E; --amber: #FFB800;
  --red: #EF4444; --orange: #FF6B00; --blue: #3B82F6; --purple: #A855F7;
  --cyan: #06B6D4;
}
* { margin:0; padding:0; box-sizing:border-box; }
body { font-family: -apple-system, system-ui, sans-serif; background: var(--charcoal); color: var(--offwhite); height: 100vh; display: flex; flex-direction: column; }

header {
  background: var(--black); border-bottom: 3px solid var(--yellow);
  padding: 10px 20px; display: flex; justify-content: space-between; align-items: center;
  z-index: 10;
}
header h1 { font-size: 1.2em; color: var(--yellow); }
header .loop { font-size: 0.8em; color: #888; margin-left: 8px; }
.header-right { display: flex; gap: 10px; align-items: center; }

#trigger-btn {
  background: var(--yellow); color: #000; border: none; padding: 8px 18px;
  border-radius: 6px; font-weight: bold; font-size: 0.85em; cursor: pointer;
  transition: transform 0.1s, box-shadow 0.1s;
}
#trigger-btn:hover { transform: scale(1.03); box-shadow: 0 0 12px rgba(254,233,0,0.4); }
#trigger-btn:active { transform: scale(0.97); }
#trigger-btn:disabled { background: #555; color: #999; cursor: not-allowed; }
#phase-badge { font-size: 0.75em; padding: 4px 10px; border-radius: 4px; background: #2a2a2a; color: #999; min-width: 120px; text-align: center; font-weight: 600; }

#main { flex:1; display: flex; overflow: hidden; }
#left {
  width: 285px; background: var(--graphite); overflow-y: auto;
  padding: 10px; border-right: 1px solid var(--border); display: flex; flex-direction: column; gap: 8px;
}
#map { flex:1; position: relative; }

/* Legend */
#legend {
  position: absolute; bottom: 20px; right: 10px; z-index: 2;
  background: rgba(0,0,0,0.82); border-radius: 8px; padding: 10px 14px;
  font-size: 0.68em; border: 1px solid var(--border);
}
#legend .row { display: flex; align-items: center; gap: 8px; margin: 4px 0; }
#legend .swatch { width: 12px; height: 12px; border-radius: 50%; border: 2px solid #fff; }
#legend .swatch.rid { background: var(--blue); }
#legend .swatch.cab { background: var(--orange); }
#legend .swatch.won { background: var(--green); }
#legend .swatch.pkup { background: rgba(34,197,94,0.20); border: 1.5px solid rgba(34,197,94,0.55); border-radius: 2px; }
#legend .swatch.drop { background: rgba(168,85,247,0.20); border: 1.5px solid rgba(168,85,247,0.55); border-radius: 2px; }

/* Parties */
#parties { display: flex; flex-direction: column; gap: 4px; }
.party {
  padding: 7px 10px; border-radius: 6px; background: var(--charcoal);
  border: 1px solid var(--border); display: flex; justify-content: space-between; align-items: center;
}
.party .name { font-weight: 600; font-size: 0.82em; }
.party .phase-tag { font-size: 0.66em; padding: 2px 7px; border-radius: 4px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.3px; }

.phase-tag.IDLE, .phase-tag.ACCEPTING_RIDES, .phase-tag.COMPLETE { background: #2a2a2a; color: #777; }
.phase-tag.KEYGEN { background: #7C3AED; color: #fff; }
.phase-tag.DISCOVERING { background: #2563EB; color: #fff; }
.phase-tag.OFFER_REVIEW { background: #0891B2; color: #fff; }
.phase-tag.DECIDING, .phase-tag.BIDDING { background: var(--amber); color: #000; }
.phase-tag.SCORING { background: #EA580C; color: #fff; }
.phase-tag.DECRYPT { background: #7C3AED; color: #fff; }
.phase-tag.WON, .phase-tag.MATCHED { background: var(--green); color: #000; }
.phase-tag.IN_RIDE { background: #0EA5E9; color: #fff; }

/* Wire */
#wire { display: flex; flex-direction: column; }
#wire h3 { font-size: 0.78em; color: var(--yellow); margin-bottom: 4px; }
#wire .entry { font-family: 'SF Mono', 'Fira Code', monospace; font-size: 0.62em; padding: 3px 5px; margin: 1px 0; background: #1a1a1a; border-radius: 3px; word-break: break-all; border-left: 2px solid var(--yellow); }
#wire .entry .label { color: var(--yellow); font-weight: 600; }

/* Log */
#log { display: flex; flex-direction: column; flex:1; overflow-y: auto; min-height: 40px; }
#log h3 { font-size: 0.78em; color: #888; margin-bottom: 4px; }
#log .line { font-size: 0.65em; color: #777; padding: 1px 0; border-bottom: 1px solid #1a1a1a; }
</style>
</head>
<body>
<header>
  <h1>&#x2605; Rideshare · Encrypted Auction</h1>
  <div class="header-right">
    <span id="phase-badge">—</span>
    <button id="trigger-btn" onclick="advancePhase()">&#x25B6; Next Phase</button>
    <span class="loop" id="loop-counter">—</span>
  </div>
</header>
<div id="main">
  <div id="left">
    <div id="parties"><div style="color:#666;font-size:0.78em;padding:20px;text-align:center">Waiting for demo...</div></div>
    <div id="wire"><h3>&#x1F4E6; Encrypted Wire</h3></div>
    <div id="log"><h3>Log</h3></div>
  </div>
  <div id="map">
    <div id="legend">
      <div class="row"><span class="swatch rid"></span> Rider</div>
      <div class="row"><span class="swatch cab"></span> Driver</div>
      <div class="row"><span class="swatch won"></span> Winner</div>
      <div class="row"><span class="swatch pkup"></span> Pickup zone</div>
      <div class="row"><span class="swatch drop"></span> Dropoff zone</div>
    </div>
  </div>
</div>
<script>
// ── State ──
var parties = {};
var driverStates = {};     // party -> {lat, lng, phase}
var wireEl = document.getElementById('wire');
var logEl = document.getElementById('log');
var partiesEl = document.getElementById('parties');
var loopEl = document.getElementById('loop-counter');
var phaseBadge = document.getElementById('phase-badge');
var triggerBtn = document.getElementById('trigger-btn');

var riderLat = 51.0493, riderLng = 13.7384;
var currentSessionPhase = '';
var hexReady = false;

// ── MapLibre ──
var map = new maplibregl.Map({
  container: 'map', style: {
    version: 8,
    sources: {
      'osm-tiles': { type: 'raster', tiles: ['https://tile.openstreetmap.org/{z}/{x}/{y}.png'], tileSize: 256, attribution: '&copy; OSM' }
    },
    layers: [{ id: 'osm', type: 'raster', source: 'osm-tiles' }]
  },
  center: [13.7384, 51.0493], zoom: 13
});
map.addControl(new maplibregl.NavigationControl());

map.on('load', function() {
  // Pickup hex fill
  map.addSource('hex-pickup', { type: 'geojson', data: { type: 'FeatureCollection', features: [] } });
  map.addLayer({ id: 'hex-pickup-fill', type: 'fill', source: 'hex-pickup', paint: {
    'fill-color': '#22C55E', 'fill-opacity': 0.20, 'fill-outline-color': '#22C55E'
  } });

  // Dropoff hex fill
  map.addSource('hex-dropoff', { type: 'geojson', data: { type: 'FeatureCollection', features: [] } });
  map.addLayer({ id: 'hex-dropoff-fill', type: 'fill', source: 'hex-dropoff', paint: {
    'fill-color': '#A855F7', 'fill-opacity': 0.20, 'fill-outline-color': '#A855F7'
  } });

  // Driver markers
  map.addSource('driver-points', { type: 'geojson', data: { type: 'FeatureCollection', features: [] } });
  map.addLayer({ id: 'driver-circles', type: 'circle', source: 'driver-points', paint: {
    'circle-radius': ['match', ['get', 'phase'], 'WON', 12, 7],
    'circle-color': ['match', ['get', 'phase'],
      'WON', '#22C55E', 'MATCHED', '#FFB800', 'BIDDING', '#FFB800',
      'DECIDING', '#EA580C', '#FF6B00'],
    'circle-stroke-width': 2, 'circle-stroke-color': '#fff', 'circle-opacity': 0.9
  } });

  // Rider marker
  map.addSource('rider-point', { type: 'geojson', data: { type: 'FeatureCollection', features: [] } });
  map.addLayer({ id: 'rider-circle', type: 'circle', source: 'rider-point', paint: {
    'circle-radius': 10,
    'circle-color': ['match', ['get', 'phase'],
      'WON', '#22C55E', 'IN_RIDE', '#0EA5E9', 'COMPLETE', '#666', 'IDLE', '#555',
      '#3B82F6'],
    'circle-stroke-width': 3, 'circle-stroke-color': '#fff', 'circle-opacity': 0.95
  } });

  // Dropoff pin
  map.addSource('dropoff-point', { type: 'geojson', data: { type: 'FeatureCollection', features: [] } });
  map.addLayer({ id: 'dropoff-circle', type: 'circle', source: 'dropoff-point', paint: {
    'circle-radius': 8, 'circle-color': '#A855F7', 'circle-stroke-width': 2,
    'circle-stroke-color': '#fff', 'circle-opacity': 0.85
  } });

  hexReady = true;
  // Replay deferred hex data
  replayHex();
});

// ── Hex rendering (geometry pre-computed server-side) ──
var deferredPickup = null, deferredDropoff = null;
var lastPickupPhase = 'IDLE', lastDropoffPhase = 'IDLE';

function replayHex() {
  if (deferredPickup) updateHexCell('pickup', deferredPickup, lastPickupPhase);
  if (deferredDropoff) updateHexCell('dropoff', deferredDropoff, lastDropoffPhase);
}

function updateHexCell(which, geometry, phase) {
  if (!hexReady) {
    if (which === 'pickup') deferredPickup = geometry;
    else deferredDropoff = geometry;
    return;
  }
  if (!geometry || !geometry.type) return;
  var color;
  switch (phase) {
    case 'IDLE': color = '#444'; break;
    case 'DISCOVERING': color = '#2563EB'; break;
    case 'OFFER_REVIEW': color = '#0891B2'; break;
    case 'SCORING': color = '#EA580C'; break;
    case 'DECRYPT': color = '#7C3AED'; break;
    case 'WON': color = '#22C55E'; break;
    case 'DROPOFF': color = '#A855F7'; break;
    case 'COMPLETE': color = '#333'; break;
    default: color = '#22C55E';
  }
  var srcId = which === 'pickup' ? 'hex-pickup' : 'hex-dropoff';
  var layId = which === 'pickup' ? 'hex-pickup-fill' : 'hex-dropoff-fill';
  var opacity = (phase === 'WON' || phase === 'DROPOFF') ? 0.28 : (phase === 'IDLE' || phase === 'COMPLETE') ? 0.08 : 0.18;
  map.setPaintProperty(layId, 'fill-color', color);
  map.setPaintProperty(layId, 'fill-outline-color', color);
  map.setPaintProperty(layId, 'fill-opacity', opacity);
  map.getSource(srcId).setData({
    type: 'FeatureCollection',
    features: [{ type: 'Feature', geometry: geometry }]
  });
}

// ── Marker updates ──
function updateDriverMarkers() {
  if (!hexReady) return;
  var feats = [];
  for (var name in driverStates) {
    var ds = driverStates[name];
    feats.push({ type: 'Feature', geometry: { type: 'Point', coordinates: [ds.lng, ds.lat] }, properties: { name: name, phase: ds.phase } });
  }
  map.getSource('driver-points').setData({ type: 'FeatureCollection', features: feats });
}

function updateRiderMarker(lat, lng, phase) {
  if (!hexReady) return;
  map.getSource('rider-point').setData({
    type: 'FeatureCollection',
    features: [{ type: 'Feature', geometry: { type: 'Point', coordinates: [lng, lat] }, properties: { phase: phase || 'IDLE' } }]
  });
}

function updateDropoffMarker(lat, lng) {
  if (!hexReady || !lat) return;
  map.getSource('dropoff-point').setData({
    type: 'FeatureCollection',
    features: [{ type: 'Feature', geometry: { type: 'Point', coordinates: [lng, lat] } }]
  });
}

// ── Sidebar ──
function renderParties() {
  var html = '';
  if (parties['rider']) html += partyHTML('\u{1F697}', 'rider', parties['rider'].phase);
  var drvNames = Object.keys(parties).filter(function(n) { return n !== 'rider' && n !== 'server'; }).sort();
  for (var i = 0; i < drvNames.length; i++) html += partyHTML('\u{1F695}', drvNames[i], parties[drvNames[i]].phase);
  if (parties['server']) html += partyHTML('\u{1F5A5}', 'server', parties['server'].phase);
  partiesEl.innerHTML = html || '<div style="color:#666;font-size:0.78em;padding:20px;text-align:center">No parties yet</div>';
}

function partyHTML(icon, name, phase) {
  return '<div class="party"><span class="name">' + icon + ' ' + esc(name) + '</span>' +
    '<span class="phase-tag ' + esc(phase) + '">' + esc(phase) + '</span></div>';
}

function esc(s) { return (s||'').replace(/</g,'&lt;').replace(/>/g,'&gt;'); }

// ── Wire + Log ──
function addWire(ev) {
  var div = document.createElement('div');
  div.className = 'entry';
  div.innerHTML = '<span class="label">' + esc(ev.party||'') + '</span> ' + esc(ev.payload || ev.detail || '');
  wireEl.appendChild(div);
  while (wireEl.children.length > 22 && wireEl.children[2]) wireEl.removeChild(wireEl.children[2]);
}

function addLog(msg) {
  var div = document.createElement('div');
  div.className = 'line'; div.textContent = msg;
  logEl.appendChild(div);
  while (logEl.children.length > 30 && logEl.children[1]) logEl.removeChild(logEl.children[1]);
}

// ── Trigger ──
function advancePhase() {
  triggerBtn.disabled = true;
  fetch('/dashboard/trigger', { method: 'POST' })
    .then(function(r) { return r.json(); })
    .then(function() { triggerBtn.disabled = false; })
    .catch(function() { triggerBtn.disabled = false; });
}

// ── Phase color for badge ──
function phaseBadgeColor(phase) {
  var m = {
    'IDLE': '#2a2a2a', 'DISCOVERING': '#2563EB', 'KEYGEN': '#7C3AED',
    'OFFER_REVIEW': '#0891B2', 'DECIDING': '#FFB800', 'BIDDING': '#FFB800',
    'SCORING': '#EA580C', 'DECRYPT': '#7C3AED', 'WON': '#22C55E',
    'IN_RIDE': '#0EA5E9', 'COMPLETE': '#2a2a2a'
  };
  return m[phase] || '#2a2a2a';
}

// ── SSE stream ──
var evtSource = new EventSource('/dashboard/events');
evtSource.onmessage = function(e) {
  var ev;
  try { ev = JSON.parse(e.data); } catch(err) { return; }
  if (ev.type === 'connected') return;

  // New session
  if (ev.type === 'loop') {
    loopEl.textContent = ev.detail;
    wireEl.innerHTML = '<h3>\u{1F4E6} Encrypted Wire</h3>';
    logEl.innerHTML = '<h3>Log</h3>';
    for (var k in parties) { if (k !== 'server') delete parties[k]; }
    for (var d in driverStates) { delete driverStates[d]; }
    deferredPickup = null; deferredDropoff = null;
    currentSessionPhase = 'IDLE';
    phaseBadge.textContent = 'IDLE';
    phaseBadge.style.background = phaseBadgeColor('IDLE');
    if (hexReady) {
      map.getSource('hex-pickup').setData({ type: 'FeatureCollection', features: [] });
      map.getSource('hex-dropoff').setData({ type: 'FeatureCollection', features: [] });
      map.getSource('driver-points').setData({ type: 'FeatureCollection', features: [] });
      map.getSource('rider-point').setData({ type: 'FeatureCollection', features: [] });
      map.getSource('dropoff-point').setData({ type: 'FeatureCollection', features: [] });
    }
    renderParties();
    return;
  }

  // Phase change — only update badge for rider phase
  if (ev.type === 'phase') {
    parties[ev.party] = { phase: ev.phase };

    // Track the *rider's* phase for the header badge
    if (ev.party === 'rider') {
      currentSessionPhase = ev.phase;
      phaseBadge.textContent = ev.phase;
      phaseBadge.style.background = phaseBadgeColor(ev.phase);
    }

    // Driver position + marker
    if (ev.party !== 'rider' && ev.party !== 'server' && ev.lat && ev.lng) {
      driverStates[ev.party] = { lat: ev.lat, lng: ev.lng, phase: ev.phase };
      updateDriverMarkers();
    }

    // Rider position tracking
    if (ev.party === 'rider') {
      if (ev.lat) { riderLat = ev.lat; riderLng = ev.lng; }
      updateRiderMarker(riderLat, riderLng, ev.phase);
    }

    if (ev.detail) addLog(ev.party + ': ' + ev.detail);
    renderParties();
  }

  // Hex cell event — uses server-computed geometry
  if (ev.type === 'hex') {
    var which = ev.party === 'dropoff' ? 'dropoff' : 'pickup';
    var geom = ev.geometry || null;
    if (which === 'pickup') {
      lastPickupPhase = ev.phase;
      if (ev.lat) { riderLat = ev.lat; riderLng = ev.lng; }
      updateRiderMarker(riderLat, riderLng, ev.phase);
    } else {
      lastDropoffPhase = ev.phase;
      if (ev.lat) updateDropoffMarker(ev.lat, ev.lng);
    }
    updateHexCell(which, geom, ev.phase);
    if (ev.detail) addLog(which + ' cell: ' + ev.detail);
  }

  // Wire event
  if (ev.type === 'wire') addWire(ev);

  // Marker event
  if (ev.type === 'marker') {
    if (ev.lat && ev.lng && ev.party) {
      driverStates[ev.party] = { lat: ev.lat, lng: ev.lng, phase: ev.phase };
      updateDriverMarkers();
    }
    if (ev.detail) addLog(ev.party + ': ' + ev.detail);
  }
};
</script>
</body>
</html>`
