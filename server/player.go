package server

// PlayerID 表示玩家唯一标识
type PlayerID string

// Direction 移动方向（服务端权威解释客户端“意图”）
type Direction int

const (
    DirNone Direction = iota
    DirUp
    DirDown
    DirLeft
    DirRight
)

// PlayerState 为广播给客户端的轻量状态
type PlayerState struct {
    ID string  `json:"id"`
    X  float64 `json:"x"`
    Y  float64 `json:"y"`
}

// Player 房间内的玩家实体（服务端权威状态）
type Player struct {
    ID  PlayerID
    X   float64
    Y   float64
    Dir Direction // 当前意图方向，在下一次 Tick 生效

    Conn *ClientConn // 网络连接的发送端（写协程）
}

