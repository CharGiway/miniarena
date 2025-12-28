package server

import (
	"encoding/json"
	"math/rand"
	"time"
)

// Room 房间世界：权威状态维护在内存，单线程 Tick 推进
type Room struct {
	ID string

	Players   map[PlayerID]*Player
	inputChan chan Input
	leaveChan chan PlayerID

	// Phase 2：网络模拟与裁决
	simulateDelayMinMs int     // 输入延迟下限（毫秒）
	simulateDelayMaxMs int     // 输入延迟上限（毫秒）
	simulateDropProb   float64 // 随机丢弃比例（0~1）
	rng                *rand.Rand

	// 每 Tick 输入限流
	inputsAcceptedThisTick map[PlayerID]int
	maxInputsPerTick       int

	// 配置：世界边界与每 Tick 移动步长
	width  float64
	height float64
	step   float64

	tickerStarted bool

	// 阶段3：Tick 序号与输入确认序列
	tickSeq          int64
	lastSeqProcessed map[PlayerID]int64

	// 阶段4：玩家最近快照（断线重连恢复位置）
	lastKnown map[PlayerID]PlayerState

	// 阶段5：上一帧广播的权威状态，用于增量计算
	lastBroadcast map[PlayerID]PlayerState
}

// NewRoom 创建房间，初始化数据结构
func NewRoom(id string) *Room {
	return &Room{
		ID:        id,
		Players:   make(map[PlayerID]*Player),
		inputChan: make(chan Input, 256), // 足够缓冲，避免网络读阻塞影响 Tick
		leaveChan: make(chan PlayerID, 64),
		width:     100,
		height:    100,
		step:      1, // 每次输入仅移动 1 单位
		// Phase 2 默认参数
		simulateDelayMinMs:     150,
		simulateDelayMaxMs:     300,
		simulateDropProb:       0.10,
		rng:                    rand.New(rand.NewSource(time.Now().UnixNano())),
		inputsAcceptedThisTick: make(map[PlayerID]int),
		maxInputsPerTick:       1,
		// 阶段3：确认序列
		lastSeqProcessed: make(map[PlayerID]int64),
		// 阶段4：最近快照
		lastKnown:     make(map[PlayerID]PlayerState),
		lastBroadcast: make(map[PlayerID]PlayerState),
	}
}

// JoinPlayer 将玩家加入房间
func (r *Room) JoinPlayer(id PlayerID, conn *ClientConn) *Player {
	// 若存在历史快照，按最近位置恢复；否则默认居中
	initX, initY := 50.0, 50.0
	if st, ok := r.lastKnown[id]; ok {
		initX, initY = st.X, st.Y
	}
	p := &Player{ID: id, X: initX, Y: initY, Dir: DirNone, Conn: conn}
	r.Players[id] = p
	return p
}

// LeavePlayer 将玩家移出房间
func (r *Room) LeavePlayer(id PlayerID) {
	if p, ok := r.Players[id]; ok {
		if p.Conn != nil {
			p.Conn.Close()
		}
		// 记录最近位置快照，供断线重连恢复
		r.lastKnown[id] = PlayerState{ID: string(id), X: p.X, Y: p.Y}
		delete(r.Players, id)
	}
}

// OnInput 入站输入（不立即改变位置），仅记录意图，等下一次 Tick 处理
func (r *Room) OnInput(in Input) {
	// Phase 2：引入随机延迟与随机丢弃（延迟的是输入进入世界的时间）
	if r.simulateDropProb > 0 && r.rng.Float64() < r.simulateDropProb {
		// 丢弃该条输入，模拟丢包（调试）
		Log.Debugf("drop input: player=%s seq=%d", string(in.PlayerID), in.Seq)
		return
	}
	min, max := r.simulateDelayMinMs, r.simulateDelayMaxMs
	if max < min {
		max = min
	}
	delayMs := min
	if max > min {
		delayMs = min + r.rng.Intn(max-min+1)
	}
	time.AfterFunc(time.Duration(delayMs)*time.Millisecond, func() {
		// 不阻塞：入口通道满则丢弃，保证 Tick 准时
		select {
		case r.inputChan <- in:
		default:
			// 丢弃：避免背压影响世界推进
			Log.Warnf("discard due to chan full: player=%s seq=%d", string(in.PlayerID), in.Seq)
		}
	})
}

// ProcessInputs 处理当前帧的所有输入意图（非阻塞 drain）
func (r *Room) ProcessInputs() {
	for {
		select {
		case pid := <-r.leaveChan:
			r.LeavePlayer(pid)
		case in := <-r.inputChan:
			if p, ok := r.Players[in.PlayerID]; ok {
				// 阶段3：去重/乱序保护（按客户端序列号）
				if in.Seq > 0 {
					if last := r.lastSeqProcessed[in.PlayerID]; in.Seq <= last {
						Log.Debugf("ignore old seq: player=%s seq=%d last=%d", string(in.PlayerID), in.Seq, last)
						break
					}
				}
				// 每 Tick 限流：超额输入忽略（权威裁决）
				cnt := r.inputsAcceptedThisTick[in.PlayerID]
				if cnt >= r.maxInputsPerTick {
					// 超限输入丢弃
					Log.Warnf("rate limit: player=%s seq=%d cnt=%d", string(in.PlayerID), in.Seq, cnt)
				} else {
					r.applyMove(p, in.Command) // 每个输入仅移动一步
					r.inputsAcceptedThisTick[in.PlayerID] = cnt + 1
					if in.Seq > 0 {
						r.lastSeqProcessed[in.PlayerID] = in.Seq
						Log.Infof("accept input: player=%s seq=%d cnt=%d", string(in.PlayerID), in.Seq, cnt+1)
					}
				}
			}
		default:
			return
		}
	}
}

// UpdateWorld 推进世界其他状态（本例中位置由输入驱动，世界无持续速度）
func (r *Room) UpdateWorld() {
	// 预留：例如子弹、碰撞、计时器等；当前无需位置变化
}

// Broadcast 将当前世界状态广播给所有玩家（文本 JSON）
func (r *Room) Broadcast() {
	snapshot := make([]PlayerState, 0, len(r.Players))
	for _, p := range r.Players {
		snapshot = append(snapshot, PlayerState{ID: string(p.ID), X: p.X, Y: p.Y})
	}
	acks := make(map[string]int64, len(r.lastSeqProcessed))
	for pid, seq := range r.lastSeqProcessed {
		acks[string(pid)] = seq
	}
	payload := struct {
		Type    string           `json:"type"`
		Tick    int64            `json:"tick"`
		Players []PlayerState    `json:"players"`
		Acks    map[string]int64 `json:"acks"`
	}{Type: "state", Tick: r.tickSeq, Players: snapshot, Acks: acks}

	b, _ := json.Marshal(payload)
	for _, p := range r.Players {
		if p.Conn != nil {
			p.Conn.Enqueue(b)
		}
	}
}

// BroadcastDelta 只广播变化的玩家，以及被移除的玩家列表
func (r *Room) BroadcastDelta() {
	// 计算变化与移除
	changed := make([]PlayerState, 0, len(r.Players))
	removed := make([]string, 0)

	// 被移除的玩家：存在于 lastBroadcast 但不在当前 Players
	for pid := range r.lastBroadcast {
		if _, ok := r.Players[pid]; !ok {
			removed = append(removed, string(pid))
		}
	}
	// 发生变化或新增的玩家
	for pid, p := range r.Players {
		prev, had := r.lastBroadcast[pid]
		if !had || prev.X != p.X || prev.Y != p.Y {
			changed = append(changed, PlayerState{ID: string(pid), X: p.X, Y: p.Y})
		}
	}

	// 如果变化覆盖率很高，降级为全量（state）
	if len(changed) >= len(r.Players) {
		r.Broadcast()
		// 同步 lastBroadcast 为当前全量
		r.lastBroadcast = make(map[PlayerID]PlayerState, len(r.Players))
		for pid, p := range r.Players {
			r.lastBroadcast[pid] = PlayerState{ID: string(pid), X: p.X, Y: p.Y}
		}
		return
	}

	acks := make(map[string]int64, len(r.lastSeqProcessed))
	for pid, seq := range r.lastSeqProcessed {
		acks[string(pid)] = seq
	}
	payload := struct {
		Type    string           `json:"type"`
		Tick    int64            `json:"tick"`
		Players []PlayerState    `json:"players"` // changed only
		Removed []string         `json:"removed"`
		Acks    map[string]int64 `json:"acks"`
	}{Type: "delta", Tick: r.tickSeq, Players: changed, Removed: removed, Acks: acks}

	b, _ := json.Marshal(payload)
	for _, p := range r.Players {
		if p.Conn != nil {
			p.Conn.Enqueue(b)
		}
	}

	// 更新 lastBroadcast：删除 removed，写入 changed
	for _, id := range removed {
		delete(r.lastBroadcast, PlayerID(id))
	}
	for _, st := range changed {
		r.lastBroadcast[PlayerID(st.ID)] = st
	}
}

// SendSnapshotTo 向指定玩家发送一次权威快照（初连/重连）
func (r *Room) SendSnapshotTo(id PlayerID) {
	p, ok := r.Players[id]
	if !ok || p.Conn == nil {
		return
	}
	world := make([]PlayerState, 0, len(r.Players))
	for _, pl := range r.Players {
		world = append(world, PlayerState{ID: string(pl.ID), X: pl.X, Y: pl.Y})
	}
	acks := make(map[string]int64, len(r.lastSeqProcessed))
	for pid, seq := range r.lastSeqProcessed {
		acks[string(pid)] = seq
	}
	payload := struct {
		Type    string           `json:"type"`
		Tick    int64            `json:"tick"`
		Players []PlayerState    `json:"players"`
		Acks    map[string]int64 `json:"acks"`
	}{Type: "snapshot", Tick: r.tickSeq, Players: world, Acks: acks}
	b, _ := json.Marshal(payload)
	// 打印快照（调试）
	Log.Debugf("snapshot: %s", string(b))
	p.Conn.Enqueue(b)
}

// applyMove 执行一次移动并进行越界裁剪
func (r *Room) applyMove(p *Player, dir Direction) {
	switch dir {
	case DirUp:
		p.Y -= r.step
	case DirDown:
		p.Y += r.step
	case DirLeft:
		p.X -= r.step
	case DirRight:
		p.X += r.step
	default:
		// no-op
	}
	if p.X < 0 {
		p.X = 0
	}
	if p.Y < 0 {
		p.Y = 0
	}
	if p.X > r.width {
		p.X = r.width
	}
	if p.Y > r.height {
		p.Y = r.height
	}
}

// RequestLeave 请求在 Tick 线程中移除玩家，避免并发改动房间状态
func (r *Room) RequestLeave(pid PlayerID) {
	// 为保证移除一定生效，这里采用阻塞式写入（通道有容量，避免死锁）
	r.leaveChan <- pid
}

// BeginTick 每帧开始时重置输入计数，保证同一时间线上的裁决一致
func (r *Room) BeginTick() {
	r.tickSeq++
	r.inputsAcceptedThisTick = make(map[PlayerID]int)
}
