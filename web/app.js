let ws = null;
const logEl = document.getElementById('log');
const statusEl = document.getElementById('status');
const cv = document.getElementById('cv');
const ctx = cv.getContext('2d');

// 客户端预测 + 回滚状态
let myId = null;
let nextSeq = 1;
let pendingInputs = []; // {seq, cmd}
let localPlayers = {};  // 客户端当前展示的世界（含预测）
let reconcileTarget = null; // 我的目标位置（服务器裁决+未确认重演）
let animating = false;
let lastAuthMy = null; // 最近一次服务器确认的我的权威位置

function log(msg) {
  const p = document.createElement('div');
  p.textContent = msg;
  logEl.appendChild(p);
  logEl.scrollTop = logEl.scrollHeight;
}

function resetUI() {
  statusEl.textContent = '未连接';
  localPlayers = {};
  drawPlayers();
}

function drawPlayers() {
  // 世界坐标 [0,100] 映射到 400 像素
  const scale = 4;
  ctx.clearRect(0,0,cv.width,cv.height);
  ctx.fillStyle = '#eef';
  ctx.fillRect(0,0,cv.width,cv.height);
  ctx.strokeStyle = '#555';
  ctx.strokeRect(0,0,cv.width,cv.height);
  // 渲染 localPlayers（包含客户端预测）
  const ids = Object.keys(localPlayers);
  for (const id of ids) {
    const p = localPlayers[id];
    const x = p.x * scale;
    const y = p.y * scale;
    ctx.fillStyle = '#1e88e5';
    ctx.beginPath();
    ctx.arc(x, y, 6, 0, Math.PI*2);
    ctx.fill();
    ctx.fillStyle = '#000';
    ctx.fillText(id, x+8, y-8);
  }
}

function startAnimation() {
  if (animating) return;
  animating = true;
  const step = () => {
    if (!reconcileTarget || !myId || !localPlayers[myId]) {
      animating = false;
      return;
    }
    const cur = localPlayers[myId];
    const tx = reconcileTarget.x;
    const ty = reconcileTarget.y;
    const dx = tx - cur.x;
    const dy = ty - cur.y;
    const dist = Math.hypot(dx, dy);
    if (dist < 0.01) {
      localPlayers[myId].x = tx;
      localPlayers[myId].y = ty;
      animating = false;
      drawPlayers();
      return;
    }
    // 线性插值，2~3帧内逐步靠近
    const alpha = 0.35;
    localPlayers[myId].x = cur.x + dx * alpha;
    localPlayers[myId].y = cur.y + dy * alpha;
    drawPlayers();
    requestAnimationFrame(step);
  };
  requestAnimationFrame(step);
}

function connect() {
  if (ws) { try { ws.close(); } catch(e){} ws = null; }
  const player = (document.getElementById('player').value || 'alice').trim();
  myId = player;
  // 断线重连时重置本地序列与未确认输入，避免不一致
  nextSeq = 1;
  pendingInputs = [];
  localPlayers = {};
  const url = 'ws://' + location.host + '/ws?room=room-1&player=' + encodeURIComponent(player);
  log('connecting ' + url);
  ws = new WebSocket(url);
  ws.onopen = () => { statusEl.textContent = '已连接'; log('connected'); };
  ws.onclose = () => { log('disconnected'); resetUI(); setTimeout(() => location.reload(), 50); };
  ws.onerror = (e) => { log('error: ' + e); };
  ws.onmessage = (ev) => {
    try {
      const msg = JSON.parse(ev.data);
      if (msg.type === 'state' || msg.type === 'snapshot') {
        // 权威状态（服务器裁决）
        const auth = {};
        for (const p of (msg.players || [])) auth[p.id] = {x:p.x, y:p.y};
        log(`recv ${msg.type} tick=${msg.tick} myId=${myId} ack=${msg.acks?msg.acks[myId]:0} players=[${Object.keys(auth).join(',')}]`);
        // 初始化 localPlayers 中其他人的位置为权威值
        for (const id of Object.keys(auth)) {
          if (id !== myId) localPlayers[id] = {x:auth[id].x, y:auth[id].y};
        }
        // 处理我的未确认输入的确认序列
        let ack = 0;
        if (msg.acks && msg.acks[myId] != null) ack = msg.acks[myId];
        pendingInputs = pendingInputs.filter(it => it.seq > ack);
        // 关键修正：根据服务器确认序列推进 nextSeq，避免重连后继续从 1 发送被判旧包
        if (ack + 1 > nextSeq) {
          nextSeq = ack + 1;
          log(`advance nextSeq to ${nextSeq} by ack=${ack}`);
        }
        log(`pending after ack=${ack}: ${pendingInputs.map(it=>it.seq).join(',')}`);
        // 刷新我的权威位置
        if (auth[myId]) lastAuthMy = {x:auth[myId].x, y:auth[myId].y};
        // 以服务器最近位置为历史起点，重演未确认输入
        let desired = lastAuthMy ? {x:lastAuthMy.x, y:lastAuthMy.y} : localPlayers[myId] || {x:50, y:50};
        if (!auth[myId]) log('auth missing for myId, fallback to lastAuth/local');
        for (const it of pendingInputs) {
          if (it.cmd === 'up') desired.y -= 1;
          else if (it.cmd === 'down') desired.y += 1;
          else if (it.cmd === 'left') desired.x -= 1;
          else if (it.cmd === 'right') desired.x += 1;
        }
        log(`desired target: (${desired.x},${desired.y})`);
        // 目标位置（平滑拉回）
        reconcileTarget = desired;
        // 如果本地没有我的位置，则初始化为服务器位置，避免瞬移过大
        if (!localPlayers[myId] && auth[myId]) localPlayers[myId] = {x:auth[myId].x, y:auth[myId].y};
        startAnimation();
      }
      else if (msg.type === 'delta') {
        const auth = {};
        for (const p of (msg.players || [])) auth[p.id] = {x:p.x, y:p.y};
        log(`recv delta tick=${msg.tick} myId=${myId} ack=${msg.acks?msg.acks[myId]:0} changed=[${Object.keys(auth).join(',')}] removed=[${(msg.removed||[]).join(',')}]`);
        // 应用 removed
        for (const id of (msg.removed || [])) {
          delete localPlayers[id];
          if (id === myId) lastAuthMy = null;
        }
        // 应用 changed 到其他人
        for (const id of Object.keys(auth)) {
          if (id !== myId) localPlayers[id] = {x:auth[id].x, y:auth[id].y};
        }
        // ack 推进与未确认过滤
        let ack = 0;
        if (msg.acks && msg.acks[myId] != null) ack = msg.acks[myId];
        pendingInputs = pendingInputs.filter(it => it.seq > ack);
        if (ack + 1 > nextSeq) {
          nextSeq = ack + 1;
          log(`advance nextSeq to ${nextSeq} by ack=${ack}`);
        }
        // 权威起点：如 delta 包含我的变化，则刷新 lastAuthMy
        if (auth[myId]) lastAuthMy = {x:auth[myId].x, y:auth[myId].y};
        let desired = lastAuthMy ? {x:lastAuthMy.x, y:lastAuthMy.y} : (localPlayers[myId] || {x:50, y:50});
        for (const it of pendingInputs) {
          if (it.cmd === 'up') desired.y -= 1;
          else if (it.cmd === 'down') desired.y += 1;
          else if (it.cmd === 'left') desired.x -= 1;
          else if (it.cmd === 'right') desired.x += 1;
        }
        reconcileTarget = desired;
        if (!localPlayers[myId]) localPlayers[myId] = desired;
        startAnimation();
      }
    } catch (e) {}
  };
}

document.getElementById('btnConnect').onclick = connect;
document.getElementById('btnDisconnect').onclick = () => { if (ws) { ws.close(); ws = null; }};

window.addEventListener('keydown', (e) => {
  if (!ws || ws.readyState !== WebSocket.OPEN) return;
  if (e.repeat) return; // 避免系统长按重复
  let cmd = null;
  if (e.key === 'ArrowUp') cmd = 'up';
  else if (e.key === 'ArrowDown') cmd = 'down';
  else if (e.key === 'ArrowLeft') cmd = 'left';
  else if (e.key === 'ArrowRight') cmd = 'right';
  if (cmd) {
    e.preventDefault();
    // 客户端先动（预测）
    if (!localPlayers[myId]) localPlayers[myId] = lastAuthMy ? {x:lastAuthMy.x, y:lastAuthMy.y} : {x:50, y:50};
    if (cmd === 'up') localPlayers[myId].y -= 1;
    else if (cmd === 'down') localPlayers[myId].y += 1;
    else if (cmd === 'left') localPlayers[myId].x -= 1;
    else if (cmd === 'right') localPlayers[myId].x += 1;
    drawPlayers();
    // 记录未确认输入
    const seq = nextSeq++;
    pendingInputs.push({seq, cmd});
    log(`send input: id=${myId} cmd=${cmd} seq=${seq}`);
    // 发送意图及序列号
    ws.send(JSON.stringify({type:'move', command:cmd, seq}));
  }
});
