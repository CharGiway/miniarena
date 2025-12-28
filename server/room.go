package server

import (
	"encoding/json"
)

// Room 房间世界：权威状态维护在内存，单线程 Tick 推进
type Room struct {
	ID string

	Players   map[PlayerID]*Player
	inputChan chan Input
	leaveChan chan PlayerID

	// 配置：世界边界与每 Tick 移动步长
	width  float64
	height float64
	step   float64

	tickerStarted bool
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
		step:      1, // 每个 Tick 移动 1 单位
	}
}

// JoinPlayer 将玩家加入房间
func (r *Room) JoinPlayer(id PlayerID, conn *ClientConn) *Player {
	p := &Player{ID: id, X: 50, Y: 50, Dir: DirNone, Conn: conn}
	r.Players[id] = p
	return p
}

// LeavePlayer 将玩家移出房间
func (r *Room) LeavePlayer(id PlayerID) {
	if p, ok := r.Players[id]; ok {
		if p.Conn != nil {
			p.Conn.Close()
		}
		delete(r.Players, id)
	}
}

// OnInput 入站输入（不立即改变位置），仅记录意图，等下一次 Tick 处理
func (r *Room) OnInput(in Input) {
	// 不阻塞：输入拥塞时丢弃最旧（由通道容量控制），保证 Tick 准时
	select {
	case r.inputChan <- in:
	default:
		// 丢弃：为了实时性，避免背压影响世界推进
	}
}

// ProcessInputs 处理当前帧的所有输入意图（非阻塞 drain）
func (r *Room) ProcessInputs() {
	for {
		select {
		case pid := <-r.leaveChan:
			r.LeavePlayer(pid)
		case in := <-r.inputChan:
			if p, ok := r.Players[in.PlayerID]; ok {
				r.applyMove(p, in.Command) // 每个输入仅移动一步
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
	payload := struct {
		Type    string        `json:"type"`
		Players []PlayerState `json:"players"`
	}{Type: "state", Players: snapshot}

	b, _ := json.Marshal(payload)
	for _, p := range r.Players {
		if p.Conn != nil {
			p.Conn.Enqueue(b)
		}
	}
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
