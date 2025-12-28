let ws = null;
const logEl = document.getElementById('log');
const statusEl = document.getElementById('status');
const cv = document.getElementById('cv');
const ctx = cv.getContext('2d');

function log(msg) {
  const p = document.createElement('div');
  p.textContent = msg;
  logEl.appendChild(p);
  logEl.scrollTop = logEl.scrollHeight;
}

function resetUI() {
  statusEl.textContent = '未连接';
  drawPlayers([]);
}

function drawPlayers(players) {
  // 世界坐标 [0,100] 映射到 400 像素
  const scale = 4;
  ctx.clearRect(0,0,cv.width,cv.height);
  ctx.fillStyle = '#eef';
  ctx.fillRect(0,0,cv.width,cv.height);
  ctx.strokeStyle = '#555';
  ctx.strokeRect(0,0,cv.width,cv.height);
  for (const p of players) {
    const x = p.x * scale;
    const y = p.y * scale;
    ctx.fillStyle = '#1e88e5';
    ctx.beginPath();
    ctx.arc(x, y, 6, 0, Math.PI*2);
    ctx.fill();
    ctx.fillStyle = '#000';
    ctx.fillText(p.id, x+8, y-8);
  }
}

function connect() {
  if (ws) { try { ws.close(); } catch(e){} ws = null; }
  const player = document.getElementById('player').value || 'alice';
  const url = 'ws://' + location.host + '/ws?room=room-1&player=' + encodeURIComponent(player);
  log('connecting ' + url);
  ws = new WebSocket(url);
  ws.onopen = () => { statusEl.textContent = '已连接'; log('connected'); };
  ws.onclose = () => { log('disconnected'); resetUI(); setTimeout(() => location.reload(), 50); };
  ws.onerror = (e) => { log('error: ' + e); };
  ws.onmessage = (ev) => {
    try {
      const msg = JSON.parse(ev.data);
      if (msg.type === 'state') {
        drawPlayers(msg.players || []);
      }
    } catch (e) {}
  };
}

document.getElementById('btnConnect').onclick = connect;
document.getElementById('btnDisconnect').onclick = () => { if (ws) { ws.close(); ws = null; }};

window.addEventListener('keydown', (e) => {
  if (!ws || ws.readyState !== WebSocket.OPEN) return;
  let cmd = null;
  if (e.key === 'ArrowUp') cmd = 'up';
  else if (e.key === 'ArrowDown') cmd = 'down';
  else if (e.key === 'ArrowLeft') cmd = 'left';
  else if (e.key === 'ArrowRight') cmd = 'right';
  if (cmd) {
    e.preventDefault();
    ws.send(JSON.stringify({type:'move', command:cmd}));
  }
});
